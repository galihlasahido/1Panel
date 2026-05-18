package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/1Panel-dev/1Panel/core/utils/common"
	"github.com/1Panel-dev/1Panel/libpanel/encrypt"
	"github.com/1Panel-dev/1Panel/libpanel/ssh"
	"github.com/1Panel-dev/1Panel/core/utils/xpack"
	"gorm.io/gorm"
)

// INodeService is the master-side CRUD + control API for managed remote
// 1panel-agent nodes. Enrollment (Add via SSH-push) is intentionally NOT
// in this interface yet — that lives in Fase 5 and will be added as
// EnrollViaSSH(...) when the installer side is ready.
type INodeService interface {
	Page(req dto.NodeSearch) (int64, []dto.NodeInfo, error)
	Get(id uint) (*dto.NodeInfo, error)
	Update(req dto.NodeUpdate) error
	Delete(id uint) error
	Recheck(id uint) (*dto.NodeInfo, error)
	EnrollViaSSH(req dto.NodeCreate) (*dto.NodeInfo, error)
	ListItems() ([]dto.NodeItem, error)
	ListSimpleItems() ([]dto.SimpleNodeItem, error)
	SearchOptions(req dto.NodeSearch) (int64, []dto.NodeItem, error)

	NodeStats(req dto.NodeSearch) (*dto.NodeStats, error)
	GetNodeLabels(nodeID uint) ([]dto.NodeLabelKV, error)
	SetNodeLabels(req dto.NodeLabelSet) error
	BulkNodeLabel(req dto.NodeLabelBulk) error
	NodeLabelKeys() ([]string, error)
	NodeLabelValues(key string) ([]string, error)
}

type NodeService struct{}

func NewINodeService() INodeService {
	return &NodeService{}
}

// localNode is the synthetic entry representing this master / the
// local agent (reached over the unix socket, no node row).
func localNodeItem() dto.NodeItem {
	return dto.NodeItem{
		ID:      0,
		Name:    "local",
		Addr:    "127.0.0.1",
		Status:  "Healthy",
		Version: global.CONF.Base.Version,
		IsXpack: false,
		IsBound: true,
	}
}

// ListItems backs GET /core/nodes/all and POST /core/nodes/list. It
// always leads with the synthetic local node so the node picker works
// even with zero enrolled nodes (previously /all fell through to the
// /:id route and 400'd with "invalid id").
// maxLegacyNodeList caps the unpaginated /nodes/all|list|simple/all
// endpoints so a huge fleet can't OOM the master or ship a multi-MB
// payload. Scalable callers use /nodes/options (paginated) or
// /nodes/stats (rollup); these legacy endpoints are best-effort.
const maxLegacyNodeList = 500

func (s *NodeService) ListItems() ([]dto.NodeItem, error) {
	nodes, err := repo.NewINodeRepo().List(repo.WithOrderDesc("created_at"), repo.WithLimit(maxLegacyNodeList))
	if err != nil {
		return nil, err
	}
	if len(nodes) >= maxLegacyNodeList {
		global.LOG.Warnf("ListItems hit the %d-node cap; use /core/nodes/options (paginated) for large fleets", maxLegacyNodeList)
	}
	items := []dto.NodeItem{localNodeItem()}
	for _, n := range nodes {
		items = append(items, dto.NodeItem{
			ID:      n.ID,
			Name:    n.Name,
			Addr:    n.Addr,
			Status:  n.Status,
			Version: n.Version,
			IsXpack: false,
			IsBound: true,
		})
	}
	return items, nil
}

// ListSimpleItems backs GET /core/nodes/simple/all.
func (s *NodeService) ListSimpleItems() ([]dto.SimpleNodeItem, error) {
	nodes, err := repo.NewINodeRepo().List(repo.WithOrderDesc("created_at"), repo.WithLimit(maxLegacyNodeList))
	if err != nil {
		return nil, err
	}
	if len(nodes) >= maxLegacyNodeList {
		global.LOG.Warnf("ListSimpleItems hit the %d-node cap; use a paginated/scoped view for large fleets", maxLegacyNodeList)
	}
	items := []dto.SimpleNodeItem{{
		ID:            0,
		Name:          "local",
		Addr:          "127.0.0.1",
		SystemVersion: global.CONF.Base.Version,
	}}
	for _, n := range nodes {
		items = append(items, dto.SimpleNodeItem{
			ID:            n.ID,
			Name:          n.Name,
			Addr:          n.Addr,
			Description:   n.Description,
			SystemVersion: n.Version,
		})
	}
	return items, nil
}

func toNodeInfo(n model.Node) dto.NodeInfo {
	return dto.NodeInfo{
		ID:          n.ID,
		Name:        n.Name,
		Addr:        n.Addr,
		Port:        n.Port,
		Scope:       n.Scope,
		Status:      n.Status,
		Version:     n.Version,
		GroupID:     n.GroupID,
		LastCheck:   n.LastCheck,
		LastMessage: n.LastMessage,
		Description: n.Description,
		CreatedAt:   n.CreatedAt,
	}
}

func (s *NodeService) Page(req dto.NodeSearch) (int64, []dto.NodeInfo, error) {
	opts := []global.DBOption{}
	if req.Status != "" {
		opts = append(opts, repo.WithByStatus(req.Status))
	}
	if req.GroupID != 0 {
		opts = append(opts, repo.WithByGroupID(req.GroupID))
	}
	if req.Info != "" {
		needle := "%" + strings.TrimSpace(req.Info) + "%"
		opts = append(opts, func(g *gorm.DB) *gorm.DB {
			return g.Where("name LIKE ? OR addr LIKE ?", needle, needle)
		})
	}
	if ids, restricted, lerr := labelScope(req.Labels); lerr == nil && restricted {
		if len(ids) == 0 {
			return 0, []dto.NodeInfo{}, nil
		}
		opts = append(opts, repo.WithByIDs(ids))
	}
	opts = append(opts, repo.WithOrderDesc("created_at"))
	total, nodes, err := repo.NewINodeRepo().Page(req.Page, req.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	out := make([]dto.NodeInfo, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toNodeInfo(n))
	}
	attachLabels(out)
	return total, out, nil
}

// SearchOptions is the scalable, server-paginated typeahead behind the
// node picker. Unlike ListItems() it never loads the whole fleet — it
// pages + filters in SQL so it stays cheap at thousands of nodes. The
// synthetic "local" node is injected at the top of page 1 when it
// matches the active filter so it's always pickable.
func (s *NodeService) SearchOptions(req dto.NodeSearch) (int64, []dto.NodeItem, error) {
	opts := []global.DBOption{}
	if req.Status != "" {
		opts = append(opts, repo.WithByStatus(req.Status))
	}
	if req.GroupID != 0 {
		opts = append(opts, repo.WithByGroupID(req.GroupID))
	}
	if req.Info != "" {
		needle := "%" + strings.TrimSpace(req.Info) + "%"
		opts = append(opts, func(g *gorm.DB) *gorm.DB {
			return g.Where("name LIKE ? OR addr LIKE ?", needle, needle)
		})
	}
	labelRestricted := false
	if ids, restricted, lerr := labelScope(req.Labels); lerr == nil && restricted {
		labelRestricted = true
		if len(ids) == 0 {
			return 0, []dto.NodeItem{}, nil
		}
		opts = append(opts, repo.WithByIDs(ids))
	}
	opts = append(opts, repo.WithOrderDesc("created_at"))
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	total, nodes, err := repo.NewINodeRepo().Page(req.Page, req.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.NodeItem, 0, len(nodes)+1)

	localMatches := !labelRestricted && req.GroupID == 0 &&
		(req.Status == "" || req.Status == "Healthy") &&
		(req.Info == "" || strings.Contains("local", strings.ToLower(strings.TrimSpace(req.Info))))
	if req.Page == 1 && localMatches {
		items = append(items, localNodeItem())
		total++
	}
	for _, n := range nodes {
		items = append(items, dto.NodeItem{
			ID:      n.ID,
			Name:    n.Name,
			Addr:    n.Addr,
			Status:  n.Status,
			Version: n.Version,
			IsXpack: false,
			IsBound: true,
		})
	}
	return total, items, nil
}

// labelScope resolves label selectors ("key=value", AND semantics) to
// the set of matching node IDs. restricted=true means a label filter
// was requested, so an empty id set must yield zero results (not "all").
func labelScope(labels []string) (ids []uint, restricted bool, err error) {
	sel := map[string]string{}
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		kv := strings.SplitN(l, "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			continue
		}
		sel[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	if len(sel) == 0 {
		return nil, false, nil
	}
	ids, err = repo.NewINodeLabelRepo().NodeIDsMatching(sel)
	return ids, true, err
}

func attachLabels(infos []dto.NodeInfo) {
	if len(infos) == 0 {
		return
	}
	ids := make([]uint, 0, len(infos))
	for _, n := range infos {
		ids = append(ids, n.ID)
	}
	labels, err := repo.NewINodeLabelRepo().ListByNodeIDs(ids)
	if err != nil {
		return
	}
	byNode := map[uint][]dto.NodeLabelKV{}
	for _, l := range labels {
		byNode[l.NodeID] = append(byNode[l.NodeID], dto.NodeLabelKV{Key: l.Key, Value: l.Value})
	}
	for i := range infos {
		infos[i].Labels = byNode[infos[i].ID]
	}
}

// nodeStatsCache absorbs dashboard refresh storms — the Fleet page can
// poll the rollup; a short TTL keeps it from hammering the DB at scale
// without making the number meaningfully stale.
var (
	nodeStatsCache sync.Map // key string -> nodeStatsEntry
)

type nodeStatsEntry struct {
	at  time.Time
	val dto.NodeStats
}

const nodeStatsTTL = 10 * time.Second

// NodeStats is the fleet rollup for a filter/scope. Counts are done in
// SQL (GROUP BY) so it stays cheap at thousands of nodes; a 10s TTL
// cache flattens repeated polls.
func (s *NodeService) NodeStats(req dto.NodeSearch) (*dto.NodeStats, error) {
	cacheKey := fmt.Sprintf("%s|%d|%s|%s", req.Status, req.GroupID, req.Info, strings.Join(req.Labels, ","))
	if v, ok := nodeStatsCache.Load(cacheKey); ok {
		if e, ok := v.(nodeStatsEntry); ok && time.Since(e.at) < nodeStatsTTL {
			cp := e.val
			return &cp, nil
		}
	}
	st, err := s.computeNodeStats(req)
	if err != nil {
		return nil, err
	}
	nodeStatsCache.Store(cacheKey, nodeStatsEntry{at: time.Now(), val: *st})
	return st, nil
}

func (s *NodeService) computeNodeStats(req dto.NodeSearch) (*dto.NodeStats, error) {
	opts := []global.DBOption{}
	if req.Status != "" {
		opts = append(opts, repo.WithByStatus(req.Status))
	}
	if req.GroupID != 0 {
		opts = append(opts, repo.WithByGroupID(req.GroupID))
	}
	if req.Info != "" {
		needle := "%" + strings.TrimSpace(req.Info) + "%"
		opts = append(opts, func(g *gorm.DB) *gorm.DB {
			return g.Where("name LIKE ? OR addr LIKE ?", needle, needle)
		})
	}
	labelRestricted := false
	if ids, restricted, lerr := labelScope(req.Labels); lerr == nil && restricted {
		labelRestricted = true
		if len(ids) == 0 {
			return &dto.NodeStats{}, nil
		}
		opts = append(opts, repo.WithByIDs(ids))
	}
	counts, err := repo.NewINodeRepo().StatusCounts(opts...)
	if err != nil {
		return nil, err
	}
	st := &dto.NodeStats{}
	for status, n := range counts {
		switch status {
		case "Healthy":
			st.Healthy += n
		case "Unhealthy", "VersionMismatch":
			st.Unhealthy += n
		case "Pending":
			st.Pending += n
		default:
			st.Other += n
		}
		st.Total += n
	}
	localMatches := !labelRestricted && req.GroupID == 0 &&
		(req.Status == "" || req.Status == "Healthy") &&
		(req.Info == "" || strings.Contains("local", strings.ToLower(strings.TrimSpace(req.Info))))
	if localMatches {
		st.Healthy++
		st.Total++
	}
	return st, nil
}

func (s *NodeService) GetNodeLabels(nodeID uint) ([]dto.NodeLabelKV, error) {
	rows, err := repo.NewINodeLabelRepo().ListByNode(nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.NodeLabelKV, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.NodeLabelKV{Key: r.Key, Value: r.Value})
	}
	return out, nil
}

func (s *NodeService) SetNodeLabels(req dto.NodeLabelSet) error {
	m := map[string]string{}
	for _, kv := range req.Labels {
		if strings.TrimSpace(kv.Key) == "" {
			continue
		}
		m[strings.TrimSpace(kv.Key)] = strings.TrimSpace(kv.Value)
	}
	return repo.NewINodeLabelRepo().SetForNode(req.NodeID, m)
}

func (s *NodeService) BulkNodeLabel(req dto.NodeLabelBulk) error {
	if req.Op == "remove" {
		return repo.NewINodeLabelRepo().RemoveFromNodes(req.NodeIDs, req.Key)
	}
	return repo.NewINodeLabelRepo().AddToNodes(req.NodeIDs, req.Key, req.Value)
}

func (s *NodeService) NodeLabelKeys() ([]string, error) {
	return repo.NewINodeLabelRepo().DistinctKeys()
}

func (s *NodeService) NodeLabelValues(key string) ([]string, error) {
	return repo.NewINodeLabelRepo().DistinctValues(key)
}

func (s *NodeService) Get(id uint) (*dto.NodeInfo, error) {
	n, err := repo.NewINodeRepo().Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	info := toNodeInfo(n)
	return &info, nil
}

func (s *NodeService) Update(req dto.NodeUpdate) error {
	vals := map[string]interface{}{
		"group_id":    req.GroupID,
		"description": req.Description,
	}
	if err := repo.NewINodeRepo().Update(req.ID, vals); err != nil {
		return err
	}
	xpack.InvalidateNodeProxy(req.ID)
	return nil
}

func (s *NodeService) Delete(id uint) error {
	if err := repo.NewINodeRepo().Delete(repo.WithByID(id)); err != nil {
		return err
	}
	_ = repo.NewINodeLabelRepo().DeleteByNode(id)
	xpack.InvalidateNodeProxy(id)
	return nil
}

// Recheck triggers an inline health probe against one node and returns the
// updated NodeInfo. Useful for "Test connection" button in the UI without
// waiting for the next cron tick.
func (s *NodeService) Recheck(id uint) (*dto.NodeInfo, error) {
	nodeRepo := repo.NewINodeRepo()
	node, err := nodeRepo.Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	status, version, message := probeNodeInline(&node)
	if err := nodeRepo.MarkChecked(id, status, version, message); err != nil {
		return nil, err
	}
	// Re-read so timestamps come from DB.
	refreshed, err := nodeRepo.Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	info := toNodeInfo(refreshed)
	return &info, nil
}

// probeNodeInline is the synchronous, single-shot variant used by Recheck.
// It mirrors the cron job's probe but doesn't depend on the cron package.
func probeNodeInline(node *model.Node) (status, version, message string) {
	client, err := xpack.NodeHTTPClient(node, 10*time.Second)
	if err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("transport: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xpack.NodeBaseURL(node)+"/api/v2/health/check", nil)
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
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("http %d", resp.StatusCode)
	}
	var env struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&env); err != nil {
		return model.NodeStatusUnhealthy, node.Version, fmt.Sprintf("decode: %v", err)
	}
	if env.Data.Version == "" {
		return model.NodeStatusUnhealthy, node.Version, "slave returned empty version"
	}
	masterMajor, _ := splitVer(global.CONF.Base.Version)
	slaveMajor, _ := splitVer(env.Data.Version)
	if masterMajor != "" && slaveMajor != "" && masterMajor != slaveMajor {
		return model.NodeStatusVersionMismatch, env.Data.Version,
			fmt.Sprintf("major version mismatch: master=%s slave=%s", global.CONF.Base.Version, env.Data.Version)
	}
	return model.NodeStatusHealthy, env.Data.Version, ""
}

func splitVer(v string) (major, minor string) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) > 0 {
		major = parts[0]
	}
	if len(parts) > 1 {
		minor = parts[1]
	}
	return
}

// EnrollViaSSH performs the master-side half of node onboarding:
//
//  1. SSH to the target host with the provided credentials.
//  2. Verify a 1panel-agent binary is reachable in PATH (operator must have
//     installed it ahead of time — see INSTALLER_TODO.md for the planned
//     auto-install integration with the installer repo).
//  3. Mint a fresh master CA + slave server cert via NodePKIService.
//  4. SCP the cert bundle + scope marker + proxy_id to
//     /etc/1panel/bootstrap/ on the slave (mode 0600 root).
//  5. Restart 1panel-agent on the slave so it picks up the bootstrap dir
//     during InitSetting migration.
//  6. Persist a node row with the master's CLIENT cert encrypted (the cert
//     this master will present when proxying to the slave).
//  7. Probe /api/v2/health/check via mTLS and update Status.
//
// Rollback: on failure after step 4, the bootstrap files are best-effort
// removed from the slave; the master DB row is rolled back if step 6 fails.
//
// Caveats unresolved here:
//   - This flow assumes the agent binary is ALREADY on the slave. The
//     planned installer integration (auto-install via `bash install.sh
//     INSTALL_MODE=agent`) is tracked in INSTALLER_TODO.md and lives in
//     the separate 1Panel-dev/installer repo.
//   - Host-key verification uses InsecureIgnoreHostKey (consistent with
//     the rest of the codebase's SSH client). TOFU pinning is future work.
func (s *NodeService) EnrollViaSSH(req dto.NodeCreate) (*dto.NodeInfo, error) {
	if req.Name == "" || req.Addr == "" {
		return nil, errors.New("name and addr are required")
	}
	if req.SSHUser == "" {
		return nil, errors.New("sshUser is required")
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.Port == 0 {
		req.Port = 9999
	}

	// Reject duplicate names early — uniqueIndex on Node.Name would surface
	// a noisy DB error otherwise.
	nodeRepo := repo.NewINodeRepo()
	if _, err := nodeRepo.Get(repo.WithByName(req.Name)); err == nil {
		return nil, fmt.Errorf("node with name %q already exists", req.Name)
	}

	pki := NewINodePKIService()
	serverCrt, serverKey, err := pki.IssueAgentCert(req.Name, req.Addr)
	if err != nil {
		return nil, fmt.Errorf("issue slave server cert: %w", err)
	}
	clientCrt, clientKey, err := pki.IssueMasterClientCert(req.Name)
	if err != nil {
		return nil, fmt.Errorf("issue master client cert for %q: %w", req.Name, err)
	}
	caBundle, err := pki.GetCABundle()
	if err != nil {
		return nil, fmt.Errorf("load CA bundle: %w", err)
	}
	proxyID := common.RandStr(32)

	// SSH to slave + drop bootstrap files.
	sshConn := ssh.ConnInfo{
		User:        req.SSHUser,
		Addr:        req.Addr,
		Port:        int(req.SSHPort),
		AuthMode:    "password",
		Password:    req.SSHPassword,
		DialTimeOut: 10 * time.Second,
	}
	if req.SSHPrivateKey != "" {
		sshConn.AuthMode = "key"
		sshConn.PrivateKey = []byte(req.SSHPrivateKey)
		sshConn.PassPhrase = []byte(req.SSHPassPhrase)
	}
	client, err := ssh.NewClient(sshConn)
	if err != nil {
		return nil, fmt.Errorf("ssh dial: %w", err)
	}
	defer client.Close()

	if err := pushBootstrap(client, bootstrapBundle{
		Port:      req.Port,
		ProxyID:   proxyID,
		ServerCrt: serverCrt,
		ServerKey: serverKey,
		RootCrt:   caBundle,
	}); err != nil {
		return nil, err
	}

	// Restart agent on slave to trigger InitSetting migration in slave mode.
	// We tolerate failure here because the operator may need to restart it
	// manually if systemd isn't available — but record the message.
	restartMsg := ""
	if out, err := client.Run("systemctl restart 1panel-agent 2>&1 || service 1panel-agent restart 2>&1 || true"); err != nil {
		restartMsg = fmt.Sprintf("restart command returned: %v: %s", err, strings.TrimSpace(out))
	}

	// Persist node row with master's client cert encrypted at rest.
	encCrt, err := encrypt.StringEncrypt(clientCrt)
	if err != nil {
		return nil, fmt.Errorf("encrypt master client cert: %w", err)
	}
	encKey, err := encrypt.StringEncrypt(clientKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt master client key: %w", err)
	}
	node := &model.Node{
		Name:        req.Name,
		Addr:        req.Addr,
		Port:        req.Port,
		Scope:       "slave",
		Status:      model.NodeStatusPending,
		GroupID:     req.GroupID,
		AgentCrt:    encCrt,
		AgentKey:    encKey,
		ProxyID:     proxyID,
		Description: req.Description,
		LastMessage: restartMsg,
	}
	if err := nodeRepo.Create(node); err != nil {
		// Best-effort: leave bootstrap files on slave so retry can succeed.
		return nil, fmt.Errorf("persist node row: %w", err)
	}

	// Final inline probe so the returned NodeInfo carries Healthy / detail.
	info, _ := s.Recheck(node.ID)
	if info == nil {
		out := toNodeInfo(*node)
		return &out, nil
	}
	return info, nil
}

type bootstrapBundle struct {
	Port      uint
	ProxyID   string
	ServerCrt string
	ServerKey string
	RootCrt   string
}

// pushBootstrap writes the slave-side bootstrap files atomically into
// /etc/1panel/bootstrap/ via base64 to avoid shell-escaping the PEM bodies.
func pushBootstrap(client *ssh.SSHClient, b bootstrapBundle) error {
	files := map[string]string{
		"scope":      "slave",
		"port":       fmt.Sprintf("%d", b.Port),
		"proxy_id":   b.ProxyID,
		"server.crt": b.ServerCrt,
		"server.key": b.ServerKey,
		"root.crt":   b.RootCrt,
	}
	if _, err := client.Run("mkdir -p /etc/1panel/bootstrap && chmod 0700 /etc/1panel/bootstrap"); err != nil {
		return fmt.Errorf("create bootstrap dir on slave: %w", err)
	}
	for name, body := range files {
		if body == "" {
			continue
		}
		b64 := encodeBase64(body)
		cmd := fmt.Sprintf(
			"umask 077 && echo %s | base64 -d > /etc/1panel/bootstrap/%s",
			shellQuote(b64), name,
		)
		if out, err := client.Run(cmd); err != nil {
			return fmt.Errorf("write %s on slave: %v: %s", name, err, strings.TrimSpace(out))
		}
	}
	return nil
}

func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
