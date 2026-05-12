package repo

import (
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

type INodeRepo interface {
	List(opts ...global.DBOption) ([]model.Node, error)
	Page(page, pageSize int, opts ...global.DBOption) (int64, []model.Node, error)
	Get(opts ...global.DBOption) (model.Node, error)
	Create(node *model.Node) error
	Update(id uint, vals map[string]interface{}) error
	Delete(opts ...global.DBOption) error
	MarkChecked(id uint, status, version, message string) error
}

type NodeRepo struct{}

func NewINodeRepo() INodeRepo {
	return &NodeRepo{}
}

func (r *NodeRepo) List(opts ...global.DBOption) ([]model.Node, error) {
	var nodes []model.Node
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	if err := db.Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

func (r *NodeRepo) Page(page, pageSize int, opts ...global.DBOption) (int64, []model.Node, error) {
	var (
		nodes []model.Node
		total int64
	)
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	if err := db.Count(&total).Error; err != nil {
		return 0, nil, err
	}
	if pageSize > 0 {
		db = db.Limit(pageSize).Offset((page - 1) * pageSize)
	}
	if err := db.Find(&nodes).Error; err != nil {
		return 0, nil, err
	}
	return total, nodes, nil
}

func (r *NodeRepo) Get(opts ...global.DBOption) (model.Node, error) {
	var node model.Node
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	return node, db.First(&node).Error
}

func (r *NodeRepo) Create(node *model.Node) error {
	return global.DB.Create(node).Error
}

func (r *NodeRepo) Update(id uint, vals map[string]interface{}) error {
	return global.DB.Model(&model.Node{}).Where("id = ?", id).Updates(vals).Error
}

func (r *NodeRepo) Delete(opts ...global.DBOption) error {
	db := global.DB.Model(&model.Node{})
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.Node{}).Error
}

func (r *NodeRepo) MarkChecked(id uint, status, version, message string) error {
	now := time.Now()
	return global.DB.Model(&model.Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       status,
		"version":      version,
		"last_message": message,
		"last_check":   &now,
	}).Error
}
