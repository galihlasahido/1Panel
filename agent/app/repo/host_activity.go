package repo

import (
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/global"
	"gorm.io/gorm"
)

type HostActivityRepo struct{}

type IHostActivityRepo interface {
	// CreateIfNew inserts the event unless its fingerprint already
	// exists; returns true if a row was actually written.
	CreateIfNew(a *model.HostActivity) (bool, error)
	Page(limit, offset int, opts ...DBOption) (int64, []model.HostActivity, error)
	DistinctSources(kind string, since time.Time) ([]string, error)
	FailedCountsBySource(since time.Time) (map[string]int, error)
	Prune(before time.Time) (int64, error)

	GetBaseline(path string) (model.FileIntegrityBaseline, error)
	UpsertBaseline(b *model.FileIntegrityBaseline) error

	WithByKind(kind string) DBOption
	WithBySeverity(s string) DBOption
}

func NewIHostActivityRepo() IHostActivityRepo {
	return &HostActivityRepo{}
}

func (r *HostActivityRepo) CreateIfNew(a *model.HostActivity) (bool, error) {
	var count int64
	if err := global.DB.Model(&model.HostActivity{}).
		Where("fingerprint = ?", a.Fingerprint).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := global.DB.Create(a).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *HostActivityRepo) Page(limit, offset int, opts ...DBOption) (int64, []model.HostActivity, error) {
	var (
		items []model.HostActivity
		count int64
	)
	db := global.DB.Model(&model.HostActivity{})
	for _, opt := range opts {
		db = opt(db)
	}
	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	err := db.Order("event_time desc").Limit(limit).Offset(offset).Find(&items).Error
	return count, items, err
}

func (r *HostActivityRepo) DistinctSources(kind string, since time.Time) ([]string, error) {
	var out []string
	err := global.DB.Model(&model.HostActivity{}).
		Where("kind = ? AND created_at >= ?", kind, since).
		Distinct().Pluck("source", &out).Error
	return out, err
}

func (r *HostActivityRepo) FailedCountsBySource(since time.Time) (map[string]int, error) {
	type row struct {
		Source string
		N      int
	}
	var rows []row
	err := global.DB.Model(&model.HostActivity{}).
		Select("source, count(*) as n").
		Where("kind = ? AND event_time >= ? AND source <> ''", "ssh_failed", since).
		Group("source").Scan(&rows).Error
	out := map[string]int{}
	for _, x := range rows {
		out[x.Source] = x.N
	}
	return out, err
}

func (r *HostActivityRepo) Prune(before time.Time) (int64, error) {
	res := global.DB.Where("created_at < ?", before).Delete(&model.HostActivity{})
	return res.RowsAffected, res.Error
}

func (r *HostActivityRepo) GetBaseline(path string) (model.FileIntegrityBaseline, error) {
	var b model.FileIntegrityBaseline
	err := global.DB.Where("path = ?", path).First(&b).Error
	return b, err
}

func (r *HostActivityRepo) UpsertBaseline(b *model.FileIntegrityBaseline) error {
	var existing model.FileIntegrityBaseline
	if err := global.DB.Where("path = ?", b.Path).First(&existing).Error; err != nil {
		return global.DB.Create(b).Error
	}
	return global.DB.Model(&model.FileIntegrityBaseline{}).
		Where("id = ?", existing.ID).
		Updates(map[string]interface{}{"hash": b.Hash, "size": b.Size}).Error
}

func (r *HostActivityRepo) WithByKind(kind string) DBOption {
	return func(g *gorm.DB) *gorm.DB { return g.Where("kind = ?", kind) }
}

func (r *HostActivityRepo) WithBySeverity(s string) DBOption {
	return func(g *gorm.DB) *gorm.DB { return g.Where("severity = ?", s) }
}
