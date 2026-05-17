package v2

import (
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
