package v2

import (
	"strconv"
	"strings"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/gin-gonic/gin"
)

// scopeParams reads the optional ?scope=<id>&labels=k=v,k2=v2 filter
// shared by the security endpoints.
func scopeParams(c *gin.Context) (uint, []string) {
	id, _ := strconv.Atoi(c.Query("scope"))
	var labels []string
	if raw := strings.TrimSpace(c.Query("labels")); raw != "" {
		for _, l := range strings.Split(raw, ",") {
			if l = strings.TrimSpace(l); l != "" {
				labels = append(labels, l)
			}
		}
	}
	return uint(id), labels
}

// @Tags Security
// @Summary Aggregated multi-node security overview
// @Success 200 {object} dto.SecurityOverview
// @Router /core/security/overview [get]
func (b *BaseApi) SecurityOverview(c *gin.Context) {
	scopeID, labels := scopeParams(c)
	info, err := securityService.Overview(scopeID, labels)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags Security
// @Summary Aggregated multi-node host-activity timeline
// @Param limit query int false "max events"
// @Param kind query string false "filter by kind"
// @Success 200 {array} dto.SecurityActivity
// @Router /core/security/activity [get]
func (b *BaseApi) SecurityActivity(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	scopeID, labels := scopeParams(c)
	items, err := securityService.Activity(limit, c.Query("kind"), scopeID, labels)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, items)
}
