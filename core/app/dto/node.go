package dto

import "time"

type NodeInfo struct {
	ID          uint       `json:"id"`
	Name        string     `json:"name"`
	Addr        string     `json:"addr"`
	Port        uint       `json:"port"`
	Scope       string     `json:"scope"`
	Status      string     `json:"status"`
	Version     string     `json:"version"`
	GroupID     uint       `json:"groupID"`
	LastCheck   *time.Time `json:"lastCheck"`
	LastMessage string     `json:"lastMessage"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"createdAt"`
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
	Status  string `json:"status"`
	GroupID uint   `json:"groupID"`
}
