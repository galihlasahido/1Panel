//go:build !xpack && !xpackee

package xpack

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
)

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

// CollectFromNodes fans out an agent call to the local node (via the
// agent unix socket) plus every registered remote node (via mTLS),
// returning each response body. A single node failing never aborts the
// rest — the caller decides how to render partial data.
func CollectFromNodes(method, path string, body []byte, timeout time.Duration) []CollectResult {
	out := []CollectResult{}

	// Local node — always present, reached over the agent unix socket.
	local := CollectResult{NodeID: 0, NodeName: "local"}
	if st, b, err := doCollect(localSocketClient(timeout), "http://unix", method, path, body); err != nil {
		local.Err = err.Error()
	} else {
		local.Status = st
		local.Body = b
		if st >= 400 {
			local.Err = http.StatusText(st)
		}
	}
	out = append(out, local)

	nodes, err := repo.NewINodeRepo().List(repo.WithByStatus(model.NodeStatusHealthy))
	if err != nil {
		return out
	}
	for i := range nodes {
		n := &nodes[i]
		r := CollectResult{NodeID: n.ID, NodeName: n.Name}
		client, cerr := NodeHTTPClient(n, timeout)
		if cerr != nil {
			r.Err = cerr.Error()
			out = append(out, r)
			continue
		}
		st, b, derr := doCollect(client, NodeBaseURL(n), method, path, body)
		if derr != nil {
			r.Err = derr.Error()
		} else {
			r.Status = st
			r.Body = b
			if st >= 400 {
				r.Err = http.StatusText(st)
			}
		}
		out = append(out, r)
	}
	return out
}
