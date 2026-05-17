package v2

import (
	"strconv"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/gin-gonic/gin"
)

// @Tags Security
// @Summary Aggregated multi-node security overview
// @Success 200 {object} dto.SecurityOverview
// @Router /core/security/overview [get]
func (b *BaseApi) SecurityOverview(c *gin.Context) {
	info, err := securityService.Overview()
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
	items, err := securityService.Activity(limit, c.Query("kind"))
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, items)
}
