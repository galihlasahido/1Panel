//go:build !xpack && !xpackee

package xpack

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/gin-gonic/gin"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

// nodeProxy holds the per-node reverse proxy + the ProxyID secret the master
// must mirror back to the slave on every request.
type nodeProxy struct {
	proxy   *httputil.ReverseProxy
	proxyID string
}

// proxyCache maps Node.ID (uint) to *nodeProxy. Entries are evicted by
// InvalidateNodeProxy when a node is updated or deleted, so stale TLS
// material is never reused after a re-enrollment.
var proxyCache sync.Map

// InvalidateNodeProxy drops the cached reverse proxy + TLS material for one
// node. Callers (node update/delete service) MUST invoke this so the next
// request rebuilds the transport from the persisted, decrypted cert.
func InvalidateNodeProxy(nodeID uint) {
	proxyCache.Delete(nodeID)
}

// Proxy fan-outs an /api/v2/* request from the master's HTTP handler to the
// remote 1panel-agent identified by `currentNode`. It is wired in from
// core/init/router/proxy.go for every request whose CurrentNode header is
// not "local" / empty.
//
// Failure modes mapped to HTTP status:
//   - unknown node      → 502 (NodeUnBind)
//   - cert decrypt err  → 502 (NodeUnBind)
//   - version mismatch  → 502 (refused, see Fase 4)
//   - upstream unreach  → 502 from the ErrorHandler below
func Proxy(c *gin.Context, currentNode string) {
	if currentNode == "" || currentNode == "local" {
		// Defensive: the proxy middleware should have routed local
		// traffic to the unix socket before calling us.
		c.AbortWithStatus(http.StatusBadGateway)
		return
	}

	nodeRepo := repo.NewINodeRepo()
	node, err := nodeRepo.Get(repo.WithByName(currentNode))
	if err != nil {
		helper.ErrorWithDetail(c, http.StatusBadGateway, "ErrNodeUnBind", err)
		return
	}
	if node.Status == model.NodeStatusVersionMismatch {
		helper.ErrorWithDetail(c, http.StatusBadGateway, "ErrNodeVersionMismatch",
			fmt.Errorf("node %q version %q is incompatible with this master", node.Name, node.Version))
		return
	}

	np, err := getOrBuildNodeProxy(&node)
	if err != nil {
		helper.ErrorWithDetail(c, http.StatusBadGateway, "ErrNodeUnBind", err)
		return
	}
	// Proxy-Id is set inside the Director (built per-node, closes over
	// the secret), not here — so clients cannot inject the header.
	np.proxy.ServeHTTP(c.Writer, c.Request)
}

func getOrBuildNodeProxy(node *model.Node) (*nodeProxy, error) {
	if v, ok := proxyCache.Load(node.ID); ok {
		return v.(*nodeProxy), nil
	}
	np, err := buildNodeProxy(node)
	if err != nil {
		return nil, err
	}
	// LoadOrStore so concurrent first-builds dedupe to one entry.
	actual, _ := proxyCache.LoadOrStore(node.ID, np)
	return actual.(*nodeProxy), nil
}

func buildNodeProxy(node *model.Node) (*nodeProxy, error) {
	if node.AgentCrt == "" || node.AgentKey == "" {
		return nil, errors.New("node has no enrolled client cert; re-enroll required")
	}
	crtPEM, err := encrypt.StringDecrypt(node.AgentCrt)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent cert: %w", err)
	}
	keyPEM, err := encrypt.StringDecrypt(node.AgentKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent key: %w", err)
	}
	clientCert, err := tls.X509KeyPair([]byte(crtPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("load client X509 keypair: %w", err)
	}

	caBundle, err := loadMasterCABundle()
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caBundle)) {
		return nil, errors.New("invalid master CA bundle in setting table")
	}

	port := node.Port
	if port == 0 {
		port = 9999
	}
	target := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", node.Addr, port),
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      pool,
			// ServerName must match a SAN on the slave's server cert;
			// the slave cert is issued with commonName == node.Name.
			ServerName: node.Name,
		},
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	origDirector := rp.Director
	proxyIDSecret := node.ProxyID
	rp.Director = func(req *http.Request) {
		origDirector(req)
		if req.Header.Get("X-Forwarded-Proto") == "" {
			if req.TLS != nil {
				req.Header.Set("X-Forwarded-Proto", "https")
			} else {
				req.Header.Set("X-Forwarded-Proto", "http")
			}
		}
		// Always overwrite Proxy-Id with the master-issued secret so a
		// caller cannot inject their own value to bypass the slave's
		// constant-time check against /etc/1panel/.nodeProxyID.
		if proxyIDSecret != "" {
			req.Header.Set("Proxy-Id", proxyIDSecret)
		} else {
			req.Header.Del("Proxy-Id")
		}
	}

	rp.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		global.LOG.Errorf("proxy to node %q (%s:%d) failed: %v", node.Name, node.Addr, port, err)
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = rw.Write([]byte("node unreachable: " + err.Error()))
	}

	return &nodeProxy{proxy: rp, proxyID: node.ProxyID}, nil
}

// NodeHTTPClient returns an *http.Client preconfigured with the same mTLS
// material that the reverse proxy uses for `node`. It is intended for
// internal callers like the health-check cron job that need to talk to a
// slave outside of an HTTP request context.
//
// The returned client uses a fresh transport (not the proxy cache) so the
// proxy connection pool is not contaminated by short-lived control calls.
func NodeHTTPClient(node *model.Node, timeout time.Duration) (*http.Client, error) {
	if node.AgentCrt == "" || node.AgentKey == "" {
		return nil, errors.New("node has no enrolled client cert")
	}
	crtPEM, err := encrypt.StringDecrypt(node.AgentCrt)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent cert: %w", err)
	}
	keyPEM, err := encrypt.StringDecrypt(node.AgentKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt agent key: %w", err)
	}
	clientCert, err := tls.X509KeyPair([]byte(crtPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("load client X509 keypair: %w", err)
	}
	caBundle, err := loadMasterCABundle()
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caBundle)) {
		return nil, errors.New("invalid master CA bundle")
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      pool,
				ServerName:   node.Name,
			},
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     30 * time.Second,
		},
	}, nil
}

// BroadcastResult captures the outcome of one node's call during a fan-out.
type BroadcastResult struct {
	NodeID   uint
	NodeName string
	Status   int
	Err      error
}

// BroadcastToHealthyNodes fans out a control-plane HTTP call to every node
// in `Healthy` state via mTLS. Used by Sync() and (future) SSL push. Failures
// for a single node are returned in the result slice but never abort the
// fan-out — callers decide whether partial failure is fatal.
func BroadcastToHealthyNodes(method, path string, body []byte, timeout time.Duration) []BroadcastResult {
	nodes, err := repo.NewINodeRepo().List(repo.WithByStatus(model.NodeStatusHealthy))
	if err != nil {
		global.LOG.Errorf("broadcast: list nodes failed: %v", err)
		return nil
	}
	results := make([]BroadcastResult, 0, len(nodes))
	for i := range nodes {
		results = append(results, callOne(&nodes[i], method, path, body, timeout))
	}
	return results
}

func callOne(node *model.Node, method, path string, body []byte, timeout time.Duration) BroadcastResult {
	r := BroadcastResult{NodeID: node.ID, NodeName: node.Name}
	client, err := NodeHTTPClient(node, timeout)
	if err != nil {
		r.Err = err
		return r
	}
	req, err := http.NewRequest(method, NodeBaseURL(node)+path, http.NoBody)
	if err != nil {
		r.Err = err
		return r
	}
	if len(body) > 0 {
		req, err = http.NewRequest(method, NodeBaseURL(node)+path, bytesReader(body))
		if err != nil {
			r.Err = err
			return r
		}
		req.Header.Set("Content-Type", "application/json")
	}
	if node.ProxyID != "" {
		req.Header.Set("Proxy-Id", node.ProxyID)
	}
	resp, err := client.Do(req)
	if err != nil {
		r.Err = err
		return r
	}
	defer resp.Body.Close()
	r.Status = resp.StatusCode
	if resp.StatusCode >= 400 {
		r.Err = fmt.Errorf("node %q returned http %d", node.Name, resp.StatusCode)
	}
	return r
}

// NodeBaseURL returns the canonical mTLS base URL for a node.
func NodeBaseURL(node *model.Node) string {
	port := node.Port
	if port == 0 {
		port = 9999
	}
	return fmt.Sprintf("https://%s:%d", node.Addr, port)
}

func loadMasterCABundle() (string, error) {
	settingRepo := repo.NewISettingRepo()
	enc, err := settingRepo.GetValueByKey("MasterCACrt")
	if err != nil {
		return "", err
	}
	if enc == "" {
		return "", errors.New("master CA cert missing; migrations may not have run")
	}
	return encrypt.StringDecrypt(enc)
}
