package repo

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

type NodeScopeRepo struct{}

type INodeScopeRepo interface {
	List() ([]model.NodeScope, error)
	Get(opts ...global.DBOption) (model.NodeScope, error)
	Create(s *model.NodeScope) error
	Update(id uint, vals map[string]interface{}) error
	Delete(opts ...global.DBOption) error
}

func NewINodeScopeRepo() INodeScopeRepo {
	return &NodeScopeRepo{}
}

func (r *NodeScopeRepo) List() ([]model.NodeScope, error) {
	var out []model.NodeScope
	return out, global.DB.Model(&model.NodeScope{}).Order("name asc").Find(&out).Error
}

func (r *NodeScopeRepo) Get(opts ...global.DBOption) (model.NodeScope, error) {
	var s model.NodeScope
	db := global.DB.Model(&model.NodeScope{})
	for _, opt := range opts {
		db = opt(db)
	}
	return s, db.First(&s).Error
}

func (r *NodeScopeRepo) Create(s *model.NodeScope) error {
	return global.DB.Create(s).Error
}

func (r *NodeScopeRepo) Update(id uint, vals map[string]interface{}) error {
	return global.DB.Model(&model.NodeScope{}).Where("id = ?", id).Updates(vals).Error
}

func (r *NodeScopeRepo) Delete(opts ...global.DBOption) error {
	db := global.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.NodeScope{}).Error
}
