package middleware

import (
	"errors"
	"net/http"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/gin-gonic/gin"
)

// SuperAdmin restricts a route group to the bootstrap superadmin (the
// settings-based UserName). RBAC sub-users — even ones granted the
// "settings" menu — can never manage other users. Must run AFTER
// SessionAuth so a session exists.
func SuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("API_AUTH") {
			// API-token auth is the superadmin's own automation key.
			c.Next()
			return
		}
		su, err := global.SESSION.Get(c)
		if err != nil {
			helper.BadAuth(c, "ErrNotLogin", err)
			c.Abort()
			return
		}
		adminName, err := repo.NewISettingRepo().GetValueByKey("UserName")
		if err != nil || su.Name != adminName {
			helper.ErrorWithDetail(c, http.StatusForbidden, "ErrNotLogin",
				errors.New("superadmin only"))
			c.Abort()
			return
		}
		c.Next()
	}
}
