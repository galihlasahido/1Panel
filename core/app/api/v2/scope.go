package v2

import (
	"errors"
	"strconv"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/gin-gonic/gin"
)

// @Tags Scope
// @Summary List saved fleet scopes
// @Router /core/scopes/search [get]
func (b *BaseApi) ListScopes(c *gin.Context) {
	out, err := scopeService.List()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, out)
}

// @Tags Scope
// @Summary Create a saved fleet scope
// @Router /core/scopes/create [post]
func (b *BaseApi) CreateScope(c *gin.Context) {
	var req dto.NodeScopeCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	info, err := scopeService.Create(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags Scope
// @Summary Update a saved fleet scope
// @Router /core/scopes/update [post]
func (b *BaseApi) UpdateScope(c *gin.Context) {
	var req dto.NodeScopeUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := scopeService.Update(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Scope
// @Summary Delete a saved fleet scope
// @Router /core/scopes/del/{id} [post]
func (b *BaseApi) DeleteScope(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	if err := scopeService.Delete(uint(id)); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Node
// @Summary Run a bulk action across a scope/label/id set
// @Accept json
// @Param request body dto.BulkOpRequest true "request"
// @Success 200 {object} dto.BulkOpResponse
// @Router /core/nodes/bulk [post]
func (b *BaseApi) BulkNodeOp(c *gin.Context) {
	var req dto.BulkOpRequest
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	resp, err := bulkService.Run(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, resp)
}

// @Tags Node
// @Summary Fleet status rollup for a filter/scope
// @Accept json
// @Param request body dto.NodeSearch true "request"
// @Success 200 {object} dto.NodeStats
// @Router /core/nodes/stats [post]
func (b *BaseApi) NodeStats(c *gin.Context) {
	var req dto.NodeSearch
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	st, err := nodeService.NodeStats(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, st)
}
