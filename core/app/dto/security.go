package dto

import "time"

// SecuritySSHEntry is one failed SSH login as reported by an agent.
type SecuritySSHEntry struct {
	Node     string `json:"node"`
	DateStr  string `json:"dateStr"`
	User     string `json:"user"`
	Address  string `json:"address"`
	AuthMode string `json:"authMode"`
	Message  string `json:"message"`
}

// SecurityNodeSummary is the per-node slice of the security overview.
type SecurityNodeSummary struct {
	NodeName       string   `json:"nodeName"`
	Reachable      bool     `json:"reachable"`
	Error          string   `json:"error,omitempty"`
	Fail2BanActive bool     `json:"fail2banActive"`
	BannedIPs      []string `json:"bannedIPs"`
	FailedSSHCount int      `json:"failedSSHCount"`
	ListeningPorts int      `json:"listeningPorts"`
	FirewallStatus string   `json:"firewallStatus"`
}

// SecurityActivity is one host-activity event, tagged with its node.
type SecurityActivity struct {
	Node      string    `json:"node"`
	EventTime time.Time `json:"eventTime"`
	Kind      string    `json:"kind"`
	Actor     string    `json:"actor"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	Severity  string    `json:"severity"`
}

// SecurityOverview is the aggregated multi-node security posture.
type SecurityOverview struct {
	GeneratedAt    time.Time             `json:"generatedAt"`
	NodesTotal     int                   `json:"nodesTotal"`
	NodesReachable int                   `json:"nodesReachable"`
	TotalBannedIPs int                   `json:"totalBannedIPs"`
	TotalFailedSSH int                   `json:"totalFailedSSH"`
	Nodes          []SecurityNodeSummary `json:"nodes"`
	RecentFailedSSH []SecuritySSHEntry   `json:"recentFailedSSH"`
}
