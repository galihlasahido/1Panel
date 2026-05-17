package repo

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
)

type IUserRepo interface {
	List(opts ...global.DBOption) ([]model.User, error)
	Page(page, pageSize int, opts ...global.DBOption) (int64, []model.User, error)
	Get(opts ...global.DBOption) (model.User, error)
	Create(user *model.User) error
	Update(id uint, vals map[string]interface{}) error
	Delete(opts ...global.DBOption) error
}

type UserRepo struct{}

func NewIUserRepo() IUserRepo {
	return &UserRepo{}
}

func (r *UserRepo) List(opts ...global.DBOption) ([]model.User, error) {
	var users []model.User
	db := global.DB.Model(&model.User{})
	for _, opt := range opts {
		db = opt(db)
	}
	if err := db.Order("id desc").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepo) Page(page, pageSize int, opts ...global.DBOption) (int64, []model.User, error) {
	var users []model.User
	db := global.DB.Model(&model.User{})
	for _, opt := range opts {
		db = opt(db)
	}
	var total int64
	db = db.Count(&total)
	err := db.Order("id desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&users).Error
	return total, users, err
}

func (r *UserRepo) Get(opts ...global.DBOption) (model.User, error) {
	var user model.User
	db := global.DB.Model(&model.User{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&user).Error
	return user, err
}

func (r *UserRepo) Create(user *model.User) error {
	return global.DB.Create(user).Error
}

func (r *UserRepo) Update(id uint, vals map[string]interface{}) error {
	return global.DB.Model(&model.User{}).Where("id = ?", id).Updates(vals).Error
}

func (r *UserRepo) Delete(opts ...global.DBOption) error {
	db := global.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.User{}).Error
}
