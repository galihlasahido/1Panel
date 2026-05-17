package v2

import (
	"errors"
	"strconv"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/dto"
	"github.com/1Panel-dev/1Panel/core/app/service"
	"github.com/gin-gonic/gin"
)

// @Tags User
// @Summary Page RBAC users
// @Router /core/users/search [post]
func (b *BaseApi) SearchUser(c *gin.Context) {
	var req dto.UserSearch
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	total, items, err := userService.Page(req)
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, dto.PageResult{Total: total, Items: items})
}

// @Tags User
// @Summary Get one RBAC user
// @Router /core/users/{id} [get]
func (b *BaseApi) GetUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	info, err := userService.Get(uint(id))
	if err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags User
// @Summary Create an RBAC user
// @Router /core/users/create [post]
func (b *BaseApi) CreateUser(c *gin.Context) {
	var req dto.UserCreate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	info, err := userService.Create(req)
	if err != nil {
		if errors.Is(err, service.ErrUserConflict) {
			helper.BadRequest(c, err)
			return
		}
		helper.InternalServer(c, err)
		return
	}
	helper.SuccessWithData(c, info)
}

// @Tags User
// @Summary Update an RBAC user (status, language, menus, nodes)
// @Router /core/users/update [post]
func (b *BaseApi) UpdateUser(c *gin.Context) {
	var req dto.UserUpdate
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := userService.Update(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags User
// @Summary Reset an RBAC user's password
// @Router /core/users/update/password [post]
func (b *BaseApi) UpdateUserPassword(c *gin.Context) {
	var req dto.UserUpdatePassword
	if err := helper.CheckBindAndValidate(&req, c); err != nil {
		return
	}
	if err := userService.UpdatePassword(req); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}

// @Tags User
// @Summary Delete an RBAC user
// @Router /core/users/del/{id} [post]
func (b *BaseApi) DeleteUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		helper.BadRequest(c, errors.New("invalid id"))
		return
	}
	if err := userService.Delete(uint(id)); err != nil {
		helper.InternalServer(c, err)
		return
	}
	helper.Success(c)
}
