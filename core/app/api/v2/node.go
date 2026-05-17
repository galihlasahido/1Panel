package v2

import (
	"errors"
	"strconv"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/gin-gonic/gin"
)

// @Tags Node
// @Summary Search managed nodes
// @Accept json
// @Param request body dto.NodeSearch true "request"
// @Success 200 {object} dto.PageResult
// @Router /core/nodes/search [post]
func (b *BaseApi) SearchNode(c *gin.Context) {
	var req dto.NodeSearch
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	total, items, err := nodeService.Page(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, dto.PageResult{Total: total, Items: items})
}

// @Tags Node
// @Summary Get one node by ID
// @Param id path int true "node id"
// @Success 200 {object} dto.NodeInfo
// @Router /core/nodes/{id} [get]
func (b *BaseApi) GetNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	info, err := nodeService.Get(uint(id))
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags Node
// @Summary Update node metadata (group, description)
// @Accept json
// @Param request body dto.NodeUpdate true "request"
// @Router /core/nodes/update [post]
func (b *BaseApi) UpdateNode(c *gin.Context) {
	var req dto.NodeUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := nodeService.Update(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Node
// @Summary Remove a node from this master
// @Param id path int true "node id"
// @Router /core/nodes/del/{id} [post]
func (b *BaseApi) DeleteNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	if err := nodeService.Delete(uint(id)); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags Node
// @Summary Trigger an immediate health probe for one node
// @Param id path int true "node id"
// @Success 200 {object} dto.NodeInfo
// @Router /core/nodes/healthcheck/{id} [post]
func (b *BaseApi) RecheckNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	info, err := nodeService.Recheck(uint(id))
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags Node
// @Summary Enroll a new node via SSH-push
// @Accept json
// @Param request body dto.NodeCreate true "request"
// @Success 200 {object} dto.NodeInfo
// @Router /core/nodes/add [post]
func (b *BaseApi) AddNode(c *gin.Context) {
	var req dto.NodeCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	info, err := nodeService.EnrollViaSSH(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags Node
// @Summary List all nodes (incl. the synthetic local node)
// @Success 200 {array} dto.NodeItem
// @Router /core/nodes/all [get]
func (b *BaseApi) ListAllNodes(c *gin.Context) {
	items, err := nodeService.ListItems()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, items)
}

// @Tags Node
// @Summary List all nodes (filtered form used by the node picker)
// @Accept json
// @Param request body dto.NodeListReq true "request"
// @Success 200 {array} dto.NodeItem
// @Router /core/nodes/list [post]
func (b *BaseApi) ListNodes(c *gin.Context) {
	var req dto.NodeListReq
	_ = c.ShouldBindJSON(&req)
	items, err := nodeService.ListItems()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, items)
}

// @Tags Node
// @Summary List all nodes with light host metadata
// @Success 200 {array} dto.SimpleNodeItem
// @Router /core/nodes/simple/all [get]
func (b *BaseApi) ListSimpleNodes(c *gin.Context) {
	items, err := nodeService.ListSimpleItems()
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, items)
}
