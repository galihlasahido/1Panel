package service

import (
	"encoding/json"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/app/repo"
)

type IScopeService interface {
	List() ([]dto.NodeScopeInfo, error)
	Create(req dto.NodeScopeCreate) (*dto.NodeScopeInfo, error)
	Update(req dto.NodeScopeUpdate) error
	Delete(id uint) error
}

type ScopeService struct{}

func NewIScopeService() IScopeService {
	return &ScopeService{}
}

func scopeMarshal(in []string) string {
	if in == nil {
		in = []string{}
	}
	b, _ := json.Marshal(in)
	return string(b)
}

func scopeUnmarshal(s string) []string {
	out := []string{}
	if s == "" {
		return out
	}
	_ = json.Unmarshal([]byte(s), &out)
	return out
}

func toScopeInfo(s model.NodeScope) dto.NodeScopeInfo {
	return dto.NodeScopeInfo{
		ID:          s.ID,
		Name:        s.Name,
		Labels:      scopeUnmarshal(s.Labels),
		Status:      s.Status,
		Description: s.Description,
	}
}

func (s *ScopeService) List() ([]dto.NodeScopeInfo, error) {
	rows, err := repo.NewINodeScopeRepo().List()
	if err != nil {
		return nil, err
	}
	out := make([]dto.NodeScopeInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, toScopeInfo(r))
	}
	return out, nil
}

func (s *ScopeService) Create(req dto.NodeScopeCreate) (*dto.NodeScopeInfo, error) {
	m := &model.NodeScope{
		Name:        req.Name,
		Labels:      scopeMarshal(req.Labels),
		Status:      req.Status,
		Description: req.Description,
	}
	if err := repo.NewINodeScopeRepo().Create(m); err != nil {
		return nil, err
	}
	info := toScopeInfo(*m)
	return &info, nil
}

func (s *ScopeService) Update(req dto.NodeScopeUpdate) error {
	return repo.NewINodeScopeRepo().Update(req.ID, map[string]interface{}{
		"labels":      scopeMarshal(req.Labels),
		"status":      req.Status,
		"description": req.Description,
	})
}

func (s *ScopeService) Delete(id uint) error {
	return repo.NewINodeScopeRepo().Delete(repo.WithByID(id))
}
