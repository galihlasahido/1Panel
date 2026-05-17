package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/libpanel/encrypt"
)

type IUserService interface {
	Page(req dto.UserSearch) (int64, []dto.UserInfo, error)
	Get(id uint) (*dto.UserInfo, error)
	Create(req dto.UserCreate) (*dto.UserInfo, error)
	Update(req dto.UserUpdate) error
	UpdatePassword(req dto.UserUpdatePassword) error
	Delete(id uint) error
}

type UserService struct{}

func NewIUserService() IUserService {
	return &UserService{}
}

func marshalList(in []string) string {
	if in == nil {
		in = []string{}
	}
	b, _ := json.Marshal(in)
	return string(b)
}

func unmarshalList(s string) []string {
	out := []string{}
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func toUserInfo(u model.User) dto.UserInfo {
	return dto.UserInfo{
		ID:          u.ID,
		Name:        u.Name,
		Status:      u.Status,
		Language:    u.Language,
		Menus:       unmarshalList(u.Menus),
		Nodes:       unmarshalList(u.Nodes),
		Description: u.Description,
		CreatedAt:   u.CreatedAt,
	}
}

func (s *UserService) Page(req dto.UserSearch) (int64, []dto.UserInfo, error) {
	total, users, err := repo.NewIUserRepo().Page(req.Page, req.PageSize)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.UserInfo, 0, len(users))
	for _, u := range users {
		if req.Info != "" && !strings.Contains(u.Name, req.Info) {
			continue
		}
		if req.Status != "" && u.Status != req.Status {
			continue
		}
		items = append(items, toUserInfo(u))
	}
	return total, items, nil
}

func (s *UserService) Get(id uint) (*dto.UserInfo, error) {
	u, err := repo.NewIUserRepo().Get(repo.WithByID(id))
	if err != nil {
		return nil, err
	}
	info := toUserInfo(u)
	return &info, nil
}

func (s *UserService) Create(req dto.UserCreate) (*dto.UserInfo, error) {
	if _, err := repo.NewIUserRepo().Get(repo.WithByName(req.Name)); err == nil {
		return nil, errors.New("a user with this name already exists")
	}
	if name, _ := repo.NewISettingRepo().GetValueByKey("UserName"); name == req.Name {
		return nil, errors.New("name collides with the superadmin account")
	}
	enc, err := encrypt.StringEncrypt(req.Password)
	if err != nil {
		return nil, err
	}
	status := req.Status
	if status == "" {
		status = model.UserStatusEnable
	}
	lang := req.Language
	if lang == "" {
		lang = "en"
	}
	u := &model.User{
		Name:        req.Name,
		Password:    enc,
		Status:      status,
		Language:    lang,
		Menus:       marshalList(req.Menus),
		Nodes:       marshalList(req.Nodes),
		Description: req.Description,
	}
	if err := repo.NewIUserRepo().Create(u); err != nil {
		return nil, err
	}
	info := toUserInfo(*u)
	return &info, nil
}

func (s *UserService) Update(req dto.UserUpdate) error {
	if _, err := repo.NewIUserRepo().Get(repo.WithByID(req.ID)); err != nil {
		return errors.New("user not found")
	}
	vals := map[string]interface{}{
		"status":      req.Status,
		"language":    req.Language,
		"menus":       marshalList(req.Menus),
		"nodes":       marshalList(req.Nodes),
		"description": req.Description,
	}
	if req.Status == "" {
		delete(vals, "status")
	}
	if req.Language == "" {
		delete(vals, "language")
	}
	return repo.NewIUserRepo().Update(req.ID, vals)
}

func (s *UserService) UpdatePassword(req dto.UserUpdatePassword) error {
	if _, err := repo.NewIUserRepo().Get(repo.WithByID(req.ID)); err != nil {
		return errors.New("user not found")
	}
	enc, err := encrypt.StringEncrypt(req.Password)
	if err != nil {
		return err
	}
	return repo.NewIUserRepo().Update(req.ID, map[string]interface{}{"password": enc})
}

func (s *UserService) Delete(id uint) error {
	if _, err := repo.NewIUserRepo().Get(repo.WithByID(id)); err != nil {
		return errors.New("user not found")
	}
	return repo.NewIUserRepo().Delete(repo.WithByID(id))
}
