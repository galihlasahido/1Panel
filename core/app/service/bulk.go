package service

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/utils/xpack"
)

type IBulkService interface {
	Run(req dto.BulkOpRequest) (*dto.BulkOpResponse, error)
}

type BulkService struct{}

func NewIBulkService() IBulkService {
	return &BulkService{}
}

const bulkConcurrency = 16

// resolveTargets turns explicit IDs / a saved scope / ad-hoc labels
// into the concrete registered-node id set the bulk op acts on. The
// synthetic local node is never a bulk target (scopes are label-based).
func resolveTargets(req dto.BulkOpRequest) []uint {
	if len(req.NodeIDs) > 0 {
		return req.NodeIDs
	}
	sel := map[string]string{}
	add := func(list []string) {
		for _, l := range list {
			kv := strings.SplitN(strings.TrimSpace(l), "=", 2)
			if len(kv) == 2 && kv[0] != "" {
				sel[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}
	if req.ScopeID > 0 {
		if sc, err := repo.NewINodeScopeRepo().Get(repo.WithByID(req.ScopeID)); err == nil {
			add(scopeUnmarshal(sc.Labels))
		}
	}
	add(req.Labels)
	if len(sel) == 0 {
		return nil
	}
	ids, err := repo.NewINodeLabelRepo().NodeIDsMatching(sel)
	if err != nil {
		return []uint{}
	}
	return ids
}

func (s *BulkService) Run(req dto.BulkOpRequest) (*dto.BulkOpResponse, error) {
	ids := resolveTargets(req)
	resp := &dto.BulkOpResponse{Results: []dto.BulkOpResult{}}
	if len(ids) == 0 {
		return resp, nil
	}

	switch req.Action {
	case "security-collect":
		// CollectFromScope already fans out concurrently + bounded.
		for _, r := range xpack.CollectFromScope("POST", "/api/v2/hosts/activity/collect",
			[]byte(`{}`), 15*time.Second, ids) {
			res := dto.BulkOpResult{Node: r.NodeName, OK: r.Err == "" && r.Status == 200}
			if r.Err != "" {
				res.Message = r.Err
			} else {
				res.Message = "collected"
			}
			resp.Results = append(resp.Results, res)
		}

	case "healthcheck":
		var (
			mu  sync.Mutex
			wg  sync.WaitGroup
			sem = make(chan struct{}, bulkConcurrency)
		)
		ns := &NodeService{}
		for _, id := range ids {
			wg.Add(1)
			go func(nid uint) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				res := dto.BulkOpResult{}
				info, err := ns.Recheck(nid)
				if err != nil {
					res.OK = false
					res.Message = err.Error()
				} else {
					res.Node = info.Name
					res.OK = info.Status == "Healthy"
					res.Message = info.Status
					if info.LastMessage != "" {
						res.Message = info.Status + ": " + info.LastMessage
					}
				}
				mu.Lock()
				resp.Results = append(resp.Results, res)
				mu.Unlock()
			}(id)
		}
		wg.Wait()
	}

	for _, r := range resp.Results {
		if r.OK {
			resp.Succeeded++
		} else {
			resp.Failed++
		}
	}
	resp.Total = len(resp.Results)
	sort.Slice(resp.Results, func(i, j int) bool { return resp.Results[i].Node < resp.Results[j].Node })
	return resp, nil
}
