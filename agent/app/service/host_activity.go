package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/constant"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/gin-gonic/gin"
)

type IHostActivityService interface {
	Collect() error
	Page(req dto.SearchHostActivity) (int64, []model.HostActivity, error)
	RebuildBaseline() error
}

type HostActivityService struct{}

func NewIHostActivityService() IHostActivityService {
	return &HostActivityService{}
}

var hostActivityRepo = repo.NewIHostActivityRepo()
var hostActivitySetting = repo.NewISettingRepo()

func setting(key string) string {
	v, _ := hostActivitySetting.GetValueByKey(key)
	return v
}

// Collect snapshots SSH history, Fail2Ban bans and watched-file hashes
// into the host_activity timeline. Best-effort: a failing source is
// logged and skipped so one missing tool never blocks the rest.
func (s *HostActivityService) Collect() error {
	if setting("HostActivityStatus") != constant.StatusEnable {
		return nil
	}
	s.collectSSH()
	s.detectBruteForce()
	s.collectFail2Ban()
	if setting("FileIntegrityStatus") == constant.StatusEnable {
		s.collectFileIntegrity(false)
	}
	s.prune()
	return nil
}

func newSafeCtx() *gin.Context {
	return &gin.Context{Request: &http.Request{Header: http.Header{}}}
}

func (s *HostActivityService) collectSSH() {
	defer func() {
		if r := recover(); r != nil {
			global.LOG.Errorf("host-activity: ssh collect panic: %v", r)
		}
	}()
	_, hist, err := NewISSHService().LoadLog(newSafeCtx(), dto.SearchSSHLog{
		PageInfo: dto.PageInfo{Page: 1, PageSize: 500},
		Status:   "All",
	})
	if err != nil {
		global.LOG.Debugf("host-activity: ssh log unavailable: %v", err)
		return
	}
	for _, h := range hist {
		kind, sev := "ssh_login", "info"
		if h.Status != constant.StatusSuccess {
			kind, sev = "ssh_failed", "warn"
		}
		fp := fmt.Sprintf("%s|%s|%s|%s", kind, h.DateStr, h.User, h.Address)
		_, _ = hostActivityRepo.CreateIfNew(&model.HostActivity{
			EventTime:   h.Date,
			Kind:        kind,
			Actor:       h.User,
			Source:      h.Address,
			Detail:      strings.TrimSpace(h.AuthMode + " " + h.Message),
			Severity:    sev,
			Fingerprint: fp,
		})
	}
}

// detectBruteForce flags any source with >=8 failed SSH logins in the
// last 15 minutes as a crit ssh_bruteforce event (one per source per
// 15-min bucket), so an active attack stands out in the timeline.
func (s *HostActivityService) detectBruteForce() {
	defer func() {
		if r := recover(); r != nil {
			global.LOG.Errorf("host-activity: bruteforce detect panic: %v", r)
		}
	}()
	const threshold = 8
	now := time.Now()
	counts, err := hostActivityRepo.FailedCountsBySource(now.Add(-15 * time.Minute))
	if err != nil {
		return
	}
	bucket := now.Truncate(15 * time.Minute).Format("2006-01-02T15:04")
	for src, n := range counts {
		if n < threshold {
			continue
		}
		_, _ = hostActivityRepo.CreateIfNew(&model.HostActivity{
			EventTime:   now,
			Kind:        "ssh_bruteforce",
			Source:      src,
			Detail:      fmt.Sprintf("%d failed SSH logins in 15m", n),
			Severity:    "crit",
			Fingerprint: "ssh_bruteforce|" + src + "|" + bucket,
		})
	}
}

func (s *HostActivityService) collectFail2Ban() {
	defer func() {
		if r := recover(); r != nil {
			global.LOG.Errorf("host-activity: fail2ban collect panic: %v", r)
		}
	}()
	banned, err := NewIFail2BanService().Search(dto.Fail2BanSearch{Status: "banned"})
	if err != nil {
		global.LOG.Debugf("host-activity: fail2ban unavailable: %v", err)
		return
	}
	now := time.Now()
	current := map[string]bool{}
	for _, ip := range banned {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		current[ip] = true
		_, _ = hostActivityRepo.CreateIfNew(&model.HostActivity{
			EventTime:   now,
			Kind:        "fail2ban_ban",
			Source:      ip,
			Detail:      "IP banned by Fail2Ban",
			Severity:    "warn",
			Fingerprint: "fail2ban_ban|" + ip,
		})
	}
	prev, _ := hostActivityRepo.DistinctSources("fail2ban_ban", now.AddDate(0, 0, -7))
	day := now.Format("2006-01-02")
	for _, ip := range prev {
		if ip == "" || current[ip] {
			continue
		}
		_, _ = hostActivityRepo.CreateIfNew(&model.HostActivity{
			EventTime:   now,
			Kind:        "fail2ban_unban",
			Source:      ip,
			Detail:      "IP no longer banned",
			Severity:    "info",
			Fingerprint: "fail2ban_unban|" + ip + "|" + day,
		})
	}
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), st.Size(), nil
}

func (s *HostActivityService) collectFileIntegrity(silent bool) {
	defer func() {
		if r := recover(); r != nil {
			global.LOG.Errorf("host-activity: file-integrity panic: %v", r)
		}
	}()
	for _, p := range strings.Split(setting("FileIntegrityPaths"), ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		hash, size, err := hashFile(p)
		if err != nil {
			continue // path absent on this host — skip silently
		}
		base, gerr := hostActivityRepo.GetBaseline(p)
		if gerr != nil {
			_ = hostActivityRepo.UpsertBaseline(&model.FileIntegrityBaseline{Path: p, Hash: hash, Size: size})
			continue
		}
		if base.Hash != hash {
			if !silent {
				_, _ = hostActivityRepo.CreateIfNew(&model.HostActivity{
					EventTime:   time.Now(),
					Kind:        "file_change",
					Target:      p,
					Detail:      fmt.Sprintf("content changed (size %d -> %d)", base.Size, size),
					Severity:    "crit",
					Fingerprint: "file_change|" + p + "|" + hash,
				})
			}
			_ = hostActivityRepo.UpsertBaseline(&model.FileIntegrityBaseline{Path: p, Hash: hash, Size: size})
		}
	}
}

func (s *HostActivityService) prune() {
	days, err := strconv.Atoi(setting("HostActivityRetentionDays"))
	if err != nil || days <= 0 {
		days = 30
	}
	if n, err := hostActivityRepo.Prune(time.Now().AddDate(0, 0, -days)); err == nil && n > 0 {
		global.LOG.Debugf("host-activity: pruned %d old events", n)
	}
}

func (s *HostActivityService) Page(req dto.SearchHostActivity) (int64, []model.HostActivity, error) {
	var opts []repo.DBOption
	if req.Kind != "" {
		opts = append(opts, hostActivityRepo.WithByKind(req.Kind))
	}
	if req.Severity != "" {
		opts = append(opts, hostActivityRepo.WithBySeverity(req.Severity))
	}
	page, size := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 100
	}
	return hostActivityRepo.Page(size, (page-1)*size, opts...)
}

// RebuildBaseline re-snapshots every watched file's hash without
// emitting change events — call after a legitimate edit so future
// drift is measured from the new known-good state.
func (s *HostActivityService) RebuildBaseline() error {
	s.collectFileIntegrity(true)
	return nil
}
