package model

import "time"

// Node represents one remote 1panel-agent the master manages.
//
// AgentCrt / AgentKey are the master's PER-NODE *client* cert + key used to
// dial that node's mTLS endpoint. They are stored encrypted at rest.
//
// ProxyID is a non-secret random tag the master mirrors into every proxied
// request via the `Proxy-Id` header; the agent compares it constant-time
// against /etc/1panel/.nodeProxyID to bind a slave to a specific master and
// reject stray traffic.
type Node struct {
	BaseModel
	Name        string     `gorm:"uniqueIndex;not null" json:"name"`
	Addr        string     `gorm:"not null" json:"addr"`
	Port        uint       `gorm:"default:9999" json:"port"`
	Scope       string     `gorm:"default:'slave'" json:"scope"`
	Status      string     `gorm:"default:'Pending'" json:"status"`
	Version     string     `json:"version"`
	GroupID     uint       `json:"groupID"`
	AgentCrt    string     `json:"-"`
	AgentKey    string     `json:"-"`
	ProxyID     string     `json:"-"`
	LastCheck   *time.Time `json:"lastCheck"`
	LastMessage string     `json:"lastMessage"`
	Description string     `json:"description"`
}

// Node status values written by the health-check cron job.
const (
	NodeStatusPending         = "Pending"
	NodeStatusHealthy         = "Healthy"
	NodeStatusUnhealthy       = "Unhealthy"
	NodeStatusVersionMismatch = "VersionMismatch"
)
