package v2

import (
	"github.com/1Panel-dev/1Panel/agent/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/gin-gonic/gin"
)

// @Tags Host
// @Summary Page the host-activity security timeline
// @Accept json
// @Param request body dto.SearchHostActivity true "request"
// @Success 200 {object} dto.PageResult
// @Router /hosts/activity/search [post]
func (b *BaseApi) SearchHostActivity(c *gin.Context) {
	var req dto.SearchHostActivity
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	total, items, err := hostActivityService.Page(req)
	if err != nil {
		helper.ErrorWithDetail(c, 500, "ErrInternalServer", err)
		return
	}
	helper.SuccessWithData(c, dto.PageResult{Total: total, Items: items})
}

// @Tags Host
// @Summary Trigger an immediate host-activity collection
// @Router /hosts/activity/collect [post]
func (b *BaseApi) CollectHostActivity(c *gin.Context) {
	if err := hostActivityService.Collect(); err != nil {
		helper.ErrorWithDetail(c, 500, "ErrInternalServer", err)
		return
	}
	helper.SuccessWithData(c, nil)
}

// @Tags Host
// @Summary Re-snapshot file-integrity baselines (no change events)
// @Router /hosts/activity/integrity/rebuild [post]
func (b *BaseApi) RebuildFileBaseline(c *gin.Context) {
	if err := hostActivityService.RebuildBaseline(); err != nil {
		helper.ErrorWithDetail(c, 500, "ErrInternalServer", err)
		return
	}
	helper.SuccessWithData(c, nil)
}
