package model

// NodeLabel is one key=value tag on a node. Many labels per node
// (one row each); unique per (NodeID, Key) so a key has a single value
// on a given node. Labels are the scoping primitive for the fleet —
// cross-cutting selectors like env=prod, region=us, role=db.
type NodeLabel struct {
	ID     uint   `gorm:"primarykey;AUTO_INCREMENT" json:"id"`
	NodeID uint   `gorm:"uniqueIndex:idx_node_label_key;not null" json:"nodeID"`
	Key    string `gorm:"uniqueIndex:idx_node_label_key;not null" json:"key"`
	Value  string `gorm:"not null" json:"value"`
}

func (NodeLabel) TableName() string { return "node_labels" }
