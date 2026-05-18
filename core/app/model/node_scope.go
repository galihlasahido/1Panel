package model

// NodeScope is a named, reusable fleet selector — a saved label/status
// query (e.g. "prod-db" = labels[env=prod,role=db]). It's how an
// operator targets a set of nodes without re-typing selectors; P4/P6
// resolve a scope to a concrete node set for fan-out / bulk ops.
type NodeScope struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Labels      string `gorm:"type:text" json:"-"` // JSON []string of "key=value"
	Status      string `json:"status"`
	Description string `json:"description"`
}

func (NodeScope) TableName() string { return "node_scopes" }
