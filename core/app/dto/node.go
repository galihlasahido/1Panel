package dto

import "time"

type NodeLabelKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type NodeInfo struct {
	ID          uint          `json:"id"`
	Name        string        `json:"name"`
	Addr        string        `json:"addr"`
	Port        uint          `json:"port"`
	Scope       string        `json:"scope"`
	Status      string        `json:"status"`
	Version     string        `json:"version"`
	GroupID     uint          `json:"groupID"`
	Labels      []NodeLabelKV `json:"labels"`
	LastCheck   *time.Time    `json:"lastCheck"`
	LastMessage string        `json:"lastMessage"`
	Description string        `json:"description"`
	CreatedAt   time.Time     `json:"createdAt"`
}

// NodeLabelSet replaces the full label set on one node.
type NodeLabelSet struct {
	NodeID uint          `json:"nodeID" validate:"required"`
	Labels []NodeLabelKV `json:"labels"`
}

// NodeLabelBulk adds or removes one label across many nodes.
type NodeLabelBulk struct {
	NodeIDs []uint `json:"nodeIDs" validate:"required"`
	Key     string `json:"key" validate:"required"`
	Value   string `json:"value"`
	Op      string `json:"op" validate:"required,oneof=add remove"`
}

type NodeLabelValuesReq struct {
	Key string `json:"key" validate:"required"`
}

type NodeCreate struct {
	Name        string `json:"name" validate:"required"`
	Addr        string `json:"addr" validate:"required"`
	Port        uint   `json:"port"`
	GroupID     uint   `json:"groupID"`
	Description string `json:"description"`

	// SSH credentials used for the master to push install + cert bundle
	// to the slave. Populated only by the SSH-push enrollment endpoint.
	SSHUser       string `json:"sshUser"`
	SSHPort       uint   `json:"sshPort"`
	SSHPassword   string `json:"sshPassword"`
	SSHPrivateKey string `json:"sshPrivateKey"`
	SSHPassPhrase string `json:"sshPassPhrase"`
}

type NodeUpdate struct {
	ID          uint   `json:"id" validate:"required"`
	GroupID     uint   `json:"groupID"`
	Description string `json:"description"`
}

type NodeSearch struct {
	SearchWithPage
	Status  string   `json:"status"`
	GroupID uint     `json:"groupID"`
	Labels  []string `json:"labels"` // selectors "key=value", AND semantics
}

// NodeScopeInfo is a saved fleet selector.
type NodeScopeInfo struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Labels      []string `json:"labels"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
}

type NodeScopeCreate struct {
	Name        string   `json:"name" validate:"required"`
	Labels      []string `json:"labels"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
}

type NodeScopeUpdate struct {
	ID          uint     `json:"id" validate:"required"`
	Labels      []string `json:"labels"`
	Status      string   `json:"status"`
	Description string   `json:"description"`
}

// NodeStats is the fleet status rollup for a given filter/scope.
type NodeStats struct {
	Total     int64 `json:"total"`
	Healthy   int64 `json:"healthy"`
	Unhealthy int64 `json:"unhealthy"`
	Pending   int64 `json:"pending"`
	Other     int64 `json:"other"`
}

// NodeListReq is the body of POST /core/nodes/list.
type NodeListReq struct {
	Type string `json:"type"`
}

// NodeItem feeds the node picker (GET /core/nodes/all, /core/nodes/list).
// The list always includes a synthetic "local" entry for the master so
// the picker works in a single-node / local context too.
type NodeItem struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Status  string `json:"status"`
	Version string `json:"version"`
	IsXpack bool   `json:"isXpack"`
	IsBound bool   `json:"isBound"`
}

// SimpleNodeItem feeds GET /core/nodes/simple/all. Live host metrics
// (cpu/memory) are not collected here — reported as 0 rather than
// fabricated; the panel treats them as optional.
type SimpleNodeItem struct {
	ID                uint    `json:"id"`
	Name              string  `json:"name"`
	Addr              string  `json:"addr"`
	Description       string  `json:"description"`
	SystemVersion     string  `json:"systemVersion"`
	SecurityEntrance  string  `json:"securityEntrance"`
	CPUUsedPercent    float64 `json:"cpuUsedPercent"`
	CPUTotal          int     `json:"cpuTotal"`
	MemoryTotal       int64   `json:"memoryTotal"`
	MemoryUsedPercent float64 `json:"memoryUsedPercent"`
}
