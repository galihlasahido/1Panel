//go:build !xpack && !xpackee

package xpack

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
)

// fanoutConcurrency bounds simultaneous per-node calls so a fan-out
// across thousands of nodes can't exhaust sockets/goroutines.
const fanoutConcurrency = 16

// localSockPath is the agent's unix socket on the master host. Kept as a
// literal (not imported from core/init/proxy) to avoid an import cycle.
const localSockPath = "/etc/1panel/agent.sock"

// CollectResult is one node's response during a body-returning fan-out.
// Unlike BroadcastResult it keeps the payload so the master can
// aggregate (security dashboard, inventory, ...).
type CollectResult struct {
	NodeID   uint   `json:"nodeID"`
	NodeName string `json:"nodeName"`
	Status   int    `json:"status"`
	Body     []byte `json:"-"`
	Err      string `json:"err,omitempty"`
}

func localSocketClient(timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: 5 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return d.DialContext(ctx, "unix", localSockPath)
			},
			IdleConnTimeout: 30 * time.Second,
		},
	}
}

func doCollect(client *http.Client, base, method, path string, body []byte) (int, []byte, error) {
	var req *http.Request
	var err error
	if len(body) > 0 {
		req, err = http.NewRequest(method, base+path, bytesReader(body))
	} else {
		req, err = http.NewRequest(method, base+path, http.NoBody)
	}
	if err != nil {
		return 0, nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	return resp.StatusCode, b, nil
}

func collectOneRemote(n *model.Node, method, path string, body []byte, timeout time.Duration) CollectResult {
	r := CollectResult{NodeID: n.ID, NodeName: n.Name}
	client, cerr := NodeHTTPClient(n, timeout)
	if cerr != nil {
		r.Err = cerr.Error()
		return r
	}
	st, b, derr := doCollect(client, NodeBaseURL(n), method, path, body)
	if derr != nil {
		r.Err = derr.Error()
		return r
	}
	r.Status = st
	r.Body = b
	if st >= 400 {
		r.Err = http.StatusText(st)
	}
	return r
}

// collectScoped fans an agent call out concurrently (bounded) to the
// selected nodes, returning each response body. includeLocal adds the
// master's local agent (unix socket). nodeIDs nil = every Healthy node;
// non-nil = exactly those node IDs (the resolved scope). A single node
// failing never aborts the rest.
func collectScoped(method, path string, body []byte, timeout time.Duration, includeLocal bool, nodeIDs []uint) []CollectResult {
	var (
		mu  sync.Mutex
		out []CollectResult
		wg  sync.WaitGroup
	)
	sem := make(chan struct{}, fanoutConcurrency)
	add := func(r CollectResult) {
		mu.Lock()
		out = append(out, r)
		mu.Unlock()
	}

	if includeLocal {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r := CollectResult{NodeID: 0, NodeName: "local"}
			if st, b, err := doCollect(localSocketClient(timeout), "http://unix", method, path, body); err != nil {
				r.Err = err.Error()
			} else {
				r.Status = st
				r.Body = b
				if st >= 400 {
					r.Err = http.StatusText(st)
				}
			}
			add(r)
		}()
	}

	opts := []global.DBOption{repo.WithByStatus(model.NodeStatusHealthy)}
	if nodeIDs != nil {
		if len(nodeIDs) == 0 {
			wg.Wait()
			return out
		}
		opts = append(opts, repo.WithByIDs(nodeIDs))
	}
	nodes, err := repo.NewINodeRepo().List(opts...)
	if err != nil {
		wg.Wait()
		return out
	}
	for i := range nodes {
		wg.Add(1)
		go func(n *model.Node) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			add(collectOneRemote(n, method, path, body, timeout))
		}(&nodes[i])
	}
	wg.Wait()
	return out
}

// CollectFromNodes fans an agent call out to the local node plus every
// Healthy remote node (concurrent, bounded). Back-compat entry point.
func CollectFromNodes(method, path string, body []byte, timeout time.Duration) []CollectResult {
	return collectScoped(method, path, body, timeout, true, nil)
}

// CollectFromScope restricts the fan-out to a resolved node-ID set
// (a label/saved scope). Local is excluded — label scopes don't cover
// the master's synthetic local node.
func CollectFromScope(method, path string, body []byte, timeout time.Duration, nodeIDs []uint) []CollectResult {
	return collectScoped(method, path, body, timeout, false, nodeIDs)
}
