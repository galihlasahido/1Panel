package v2

import (
	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/gin-gonic/gin"
)

// GetCurrentUser returns the logged-in identity + RBAC allowlists so
// the frontend can filter the sidebar/router and node picker after a
// reload. Lives under /core/auth (SessionAuth-exempt), so the session
// is read manually here.
func (b *BaseApi) GetCurrentUser(c *gin.Context) {
	su, err := global.SESSION.Get(c)
	if err != nil {
		helper.BadAuth(c, "ErrNotLogin", err)
		return
	}
	helper.SuccessWithData(c, gin.H{
		"name":    su.Name,
		"isSuper": su.IsSuper,
		"menus":   su.Menus,
		"nodes":   su.Nodes,
	})
}
