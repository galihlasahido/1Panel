package repo

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
	"gorm.io/gorm"
)

type NodeLabelRepo struct{}

type INodeLabelRepo interface {
	ListByNodeIDs(ids []uint) ([]model.NodeLabel, error)
	ListByNode(nodeID uint) ([]model.NodeLabel, error)
	SetForNode(nodeID uint, labels map[string]string) error
	AddToNodes(nodeIDs []uint, key, value string) error
	RemoveFromNodes(nodeIDs []uint, key string) error
	DeleteByNode(nodeID uint) error
	DistinctKeys() ([]string, error)
	DistinctValues(key string) ([]string, error)
	// NodeIDsMatching returns node IDs that carry ALL of the given
	// key=value selectors (AND semantics) — the basis for label scopes.
	NodeIDsMatching(selectors map[string]string) ([]uint, error)
}

func NewINodeLabelRepo() INodeLabelRepo {
	return &NodeLabelRepo{}
}

func (r *NodeLabelRepo) ListByNodeIDs(ids []uint) ([]model.NodeLabel, error) {
	var out []model.NodeLabel
	if len(ids) == 0 {
		return out, nil
	}
	return out, global.DB.Where("node_id IN ?", ids).Find(&out).Error
}

func (r *NodeLabelRepo) ListByNode(nodeID uint) ([]model.NodeLabel, error) {
	var out []model.NodeLabel
	return out, global.DB.Where("node_id = ?", nodeID).Order("key asc").Find(&out).Error
}

func (r *NodeLabelRepo) SetForNode(nodeID uint, labels map[string]string) error {
	return global.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ?", nodeID).Delete(&model.NodeLabel{}).Error; err != nil {
			return err
		}
		for k, v := range labels {
			if k == "" {
				continue
			}
			if err := tx.Create(&model.NodeLabel{NodeID: nodeID, Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *NodeLabelRepo) AddToNodes(nodeIDs []uint, key, value string) error {
	if key == "" || len(nodeIDs) == 0 {
		return nil
	}
	return global.DB.Transaction(func(tx *gorm.DB) error {
		for _, id := range nodeIDs {
			var existing model.NodeLabel
			err := tx.Where("node_id = ? AND key = ?", id, key).First(&existing).Error
			if err == nil {
				if existing.Value != value {
					if err := tx.Model(&model.NodeLabel{}).Where("id = ?", existing.ID).
						Update("value", value).Error; err != nil {
						return err
					}
				}
				continue
			}
			if err := tx.Create(&model.NodeLabel{NodeID: id, Key: key, Value: value}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *NodeLabelRepo) RemoveFromNodes(nodeIDs []uint, key string) error {
	if key == "" || len(nodeIDs) == 0 {
		return nil
	}
	return global.DB.Where("node_id IN ? AND key = ?", nodeIDs, key).
		Delete(&model.NodeLabel{}).Error
}

func (r *NodeLabelRepo) DeleteByNode(nodeID uint) error {
	return global.DB.Where("node_id = ?", nodeID).Delete(&model.NodeLabel{}).Error
}

func (r *NodeLabelRepo) DistinctKeys() ([]string, error) {
	var out []string
	return out, global.DB.Model(&model.NodeLabel{}).
		Distinct().Order("key asc").Pluck("key", &out).Error
}

func (r *NodeLabelRepo) DistinctValues(key string) ([]string, error) {
	var out []string
	return out, global.DB.Model(&model.NodeLabel{}).
		Where("key = ?", key).Distinct().Order("value asc").Pluck("value", &out).Error
}

func (r *NodeLabelRepo) NodeIDsMatching(selectors map[string]string) ([]uint, error) {
	var ids []uint
	if len(selectors) == 0 {
		return ids, nil
	}
	// A node matches only if it carries every selector — count distinct
	// matching (key,value) rows per node and require == len(selectors).
	q := global.DB.Model(&model.NodeLabel{}).Select("node_id")
	or := global.DB
	first := true
	for k, v := range selectors {
		if first {
			or = global.DB.Where("key = ? AND value = ?", k, v)
			first = false
		} else {
			or = or.Or("key = ? AND value = ?", k, v)
		}
	}
	err := q.Where(or).Group("node_id").
		Having("COUNT(DISTINCT key) = ?", len(selectors)).
		Pluck("node_id", &ids).Error
	return ids, err
}
