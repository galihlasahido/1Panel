package service

import (
	"encoding/json"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/utils/xpack"
)

type ISecurityService interface {
	Overview() (*dto.SecurityOverview, error)
}

type SecurityService struct{}

func NewISecurityService() ISecurityService {
	return &SecurityService{}
}

// agentEnvelope is the standard 1Panel response wrapper.
type agentEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func unwrap(body []byte) (json.RawMessage, bool) {
	var e agentEnvelope
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, false
	}
	return e.Data, e.Code == 200
}

// resultsByName indexes a fan-out so each metric call lines up per node.
func collect(method, path string, body []byte) map[string]xpack.CollectResult {
	out := map[string]xpack.CollectResult{}
	for _, r := range xpack.CollectFromNodes(method, path, body, 6*time.Second) {
		out[r.NodeName] = r
	}
	return out
}

func (s *SecurityService) Overview() (*dto.SecurityOverview, error) {
	f2bBase := collect("GET", "/api/v2/toolbox/fail2ban/base", nil)
	f2bBanned := collect("POST", "/api/v2/toolbox/fail2ban/search", []byte(`{"status":"banned"}`))
	fwBase := collect("POST", "/api/v2/hosts/firewall/base", []byte(`{}`))
	listening := collect("POST", "/api/v2/process/listening", []byte(`{}`))
	sshFailed := collect("POST", "/api/v2/hosts/ssh/log",
		[]byte(`{"page":1,"pageSize":50,"Status":"Failed","info":""}`))

	// The local node anchors which nodes exist; every metric fan-out
	// visits the same node set, so iterate one of them.
	ov := &dto.SecurityOverview{GeneratedAt: time.Now(), Nodes: []dto.SecurityNodeSummary{}}
	ov.RecentFailedSSH = []dto.SecuritySSHEntry{}

	for name, base := range f2bBase {
		ns := dto.SecurityNodeSummary{NodeName: name, BannedIPs: []string{}}
		ns.Reachable = base.Err == "" && base.Status == 200
		if base.Err != "" {
			ns.Error = base.Err
		}

		if data, ok := unwrap(base.Body); ok {
			var fb struct {
				Enable bool `json:"enable"`
				Active bool `json:"active"`
			}
			_ = json.Unmarshal(data, &fb)
			ns.Fail2BanActive = fb.Enable || fb.Active
		}

		if r, has := f2bBanned[name]; has {
			if data, ok := unwrap(r.Body); ok {
				ns.BannedIPs = parseStringList(data)
			}
		}

		if r, has := fwBase[name]; has {
			if data, ok := unwrap(r.Body); ok {
				var fw struct {
					Status string `json:"status"`
				}
				_ = json.Unmarshal(data, &fw)
				ns.FirewallStatus = fw.Status
			}
		}

		if r, has := listening[name]; has {
			if data, ok := unwrap(r.Body); ok {
				ns.ListeningPorts = countList(data)
			}
		}

		if r, has := sshFailed[name]; has {
			if data, ok := unwrap(r.Body); ok {
				entries := parseSSHHistory(data)
				ns.FailedSSHCount = len(entries)
				for _, e := range entries {
					if len(ov.RecentFailedSSH) >= 50 {
						break
					}
					ov.RecentFailedSSH = append(ov.RecentFailedSSH, dto.SecuritySSHEntry{
						Node:     name,
						DateStr:  e.DateStr,
						User:     e.User,
						Address:  e.Address,
						AuthMode: e.AuthMode,
						Message:  e.Message,
					})
				}
			}
		}

		ov.NodesTotal++
		if ns.Reachable {
			ov.NodesReachable++
		}
		ov.TotalBannedIPs += len(ns.BannedIPs)
		ov.TotalFailedSSH += ns.FailedSSHCount
		ov.Nodes = append(ov.Nodes, ns)
	}
	return ov, nil
}

// parseStringList tolerates either a bare ["ip", ...] or {"items":[...]}.
func parseStringList(data json.RawMessage) []string {
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var wrapped struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		return wrapped.Items
	}
	return []string{}
}

func countList(data json.RawMessage) int {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		return len(arr)
	}
	var wrapped struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		return len(wrapped.Items)
	}
	return 0
}

type sshHist struct {
	DateStr  string `json:"dateStr"`
	User     string `json:"user"`
	Address  string `json:"address"`
	AuthMode string `json:"authMode"`
	Message  string `json:"message"`
}

func parseSSHHistory(data json.RawMessage) []sshHist {
	var arr []sshHist
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var wrapped struct {
		Items []sshHist `json:"items"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil {
		return wrapped.Items
	}
	return nil
}
