package v2

import (
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/gin-gonic/gin"
)

// processStart records the moment the agent process was first asked for its
// health, which approximates server start (the var is captured at import
// time, before Gin is even routing).
var processStart = time.Now()

// HealthResponse is the payload returned to the master's health probe.
// The master uses Version to decide whether to refuse proxy traffic (major
// version mismatch) per Fase 4 of the multi-node plan.
type HealthResponse struct {
	Version    string `json:"version"`
	Scope      string `json:"scope"`
	UptimeSecs int64  `json:"uptimeSecs"`
}

func (b *BaseApi) CheckHealth(c *gin.Context) {
	settingRepo := repo.NewISettingRepo()
	version, _ := settingRepo.GetValueByKey("SystemVersion")
	scope, _ := settingRepo.GetValueByKey("NodeScope")
	if scope == "" {
		if global.IsMaster {
			scope = "master"
		} else {
			scope = "slave"
		}
	}
	helper.SuccessWithData(c, HealthResponse{
		Version:    version,
		Scope:      scope,
		UptimeSecs: int64(time.Since(processStart).Seconds()),
	})
}
