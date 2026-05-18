package job

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/utils/xpack"
)

// NodeHealth probes every registered node's /api/v2/health/check endpoint
// via mTLS, then writes Status/Version/LastMessage back into the nodes
// table. Runs every 30 seconds from cron.Init().
type NodeHealth struct{}

func NewNodeHealthJob() *NodeHealth {
	return &NodeHealth{}
}

// healthPayload mirrors agent/app/api/v2.HealthResponse — duplicated here
// because core does not import agent. Keep in sync.
type healthPayload struct {
	Version    string `json:"version"`
	Scope      string `json:"scope"`
	UptimeSecs int64  `json:"uptimeSecs"`
}

type healthEnvelope struct {
	Code int           `json:"code"`
	Data healthPayload `json:"data"`
}

const (
	healthRequestTimeout = 10 * time.Second
	healthPath           = "/api/v2/health/check"
)

func (j *NodeHealth) Run() {
	nodeRepo := repo.NewINodeRepo()
	nodes, err := nodeRepo.List()
	if err != nil {
		global.LOG.Errorf("node health: list nodes failed: %v", err)
		return
	}
	if len(nodes) == 0 {
		return
	}
	// Probe concurrently (bounded) so a 1000-node fleet isn't an
	// O(n × timeout) serial wall. Persist sequentially afterwards —
	// SQLite is a single writer, so concurrent MarkChecked would just
	// contend on the write lock.
	type result struct {
		id                       uint
		status, version, message string
	}
	const probeConcurrency = 24
	sem := make(chan struct{}, probeConcurrency)
	results := make([]result, len(nodes))
	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s, v, m := probeNode(&nodes[idx])
			results[idx] = result{id: nodes[idx].ID, status: s, version: v, message: m}
		}(i)
	}
	wg.Wait()
	for _, r := range results {
		if err := nodeRepo.MarkChecked(r.id, r.status, r.version, r.message); err != nil {
			global.LOG.Errorf("node health: persist result for node %d failed: %v", r.id, err)
		}
	}
}

func probeNode(node *model.Node) (status, version, message string) {
	client, err := xpack.NodeHTTPClient(node, healthRequestTimeout)
	if err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("transport: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xpack.NodeBaseURL(node)+healthPath, nil)
	if err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("request: %v", err)
	}
	if node.ProxyID != "" {
		req.Header.Set("Proxy-Id", node.ProxyID)
	}
	resp, err := client.Do(req)
	if err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("dial: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return model.NodeStatusUnhealthy, node.Version,
			fmt.Sprintf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var env healthEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&env); err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("decode: %v", err)
	}
	slaveVersion := env.Data.Version
	masterVersion := global.CONF.Base.Version
	switch compareMajor(masterVersion, slaveVersion) {
	case versionMatch:
		return model.NodeStatusHealthy, slaveVersion, ""
	case versionMinorDiff:
		return model.NodeStatusHealthy, slaveVersion,
			fmt.Sprintf("minor version drift: master=%s slave=%s", masterVersion, slaveVersion)
	default:
		return model.NodeStatusVersionMismatch, slaveVersion,
			fmt.Sprintf("major version mismatch: master=%s slave=%s", masterVersion, slaveVersion)
	}
}

const (
	versionMatch = iota
	versionMinorDiff
	versionMajorDiff
)

// compareMajor parses two version strings like "v2.1.12" and returns whether
// they share a major component (refuse proxy on mismatch), minor differ
// (warn), or are identical at major.minor.
func compareMajor(a, b string) int {
	if a == "" || b == "" {
		return versionMinorDiff // can't decide; treat as warning, not block
	}
	majA, minA := splitMajorMinor(a)
	majB, minB := splitMajorMinor(b)
	if majA != majB {
		return versionMajorDiff
	}
	if minA != minB {
		return versionMinorDiff
	}
	return versionMatch
}

func splitMajorMinor(v string) (int, int) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	maj := atoiSafe(parts[0])
	min := 0
	if len(parts) > 1 {
		min = atoiSafe(parts[1])
	}
	return maj, min
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
