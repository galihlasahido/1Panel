package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/1Panel-dev/1Panel/core/app/api/v2/helper"
	"github.com/1Panel-dev/1Panel/core/global"
	"github.com/gin-gonic/gin"
)

// menuRule maps an API path prefix to the sidebar menu key that gates
// it. Checked longest-prefix-first; the first match wins. Anything not
// matched (dashboard, auth, groups, nodes, users, alerts, websockets,
// static, …) is allowed for any authenticated user — node access is
// enforced separately (P4), user management by middleware.SuperAdmin.
//
// This is REAL backend enforcement: even if a sub-user crafts the API
// call directly, a disallowed feature module returns 403. The menu
// keys here are the canonical RBAC keys the frontend also filters on.
type menuRule struct {
	prefix string
	menu   string
}

var menuRules = []menuRule{
	{"/api/v2/core/settings", "settings"},
	{"/api/v2/core/backups", "settings"},
	{"/api/v2/core/commands", "settings"},
	{"/api/v2/core/scripts", "settings"},
	{"/api/v2/apps", "apps"},
	{"/api/v2/websites", "website"},
	{"/api/v2/openresty", "website"},
	{"/api/v2/runtimes", "website"},
	{"/api/v2/databases", "database"},
	{"/api/v2/containers", "container"},
	{"/api/v2/cronjobs", "cron"},
	{"/api/v2/files", "host"},
	{"/api/v2/hosts", "host"},
	{"/api/v2/process", "host"},
	{"/api/v2/settings", "settings"},
	{"/api/v2/toolbox", "toolbox"},
	{"/api/v2/ai", "ai"},
	{"/api/v2/logs", "logs"},
}

func menuForPath(p string) (string, bool) {
	for _, r := range menuRules {
		if strings.HasPrefix(p, r.prefix) {
			return r.menu, true
		}
	}
	return "", false
}

func allowedMenu(menus []string, key string) bool {
	for _, m := range menus {
		if m == "*" || m == key {
			return true
		}
	}
	return false
}

// RBACEnforce gates feature modules by the session user's allowed
// menus. Installed once (before Proxy) so it covers both core routes
// and proxied agent routes. Superadmin and API-token automation pass
// through; unauthenticated requests are left to the existing 401 flow.
func RBACEnforce() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if !strings.HasPrefix(p, "/api/v2/") || strings.HasPrefix(p, "/api/v2/core/auth") {
			c.Next()
			return
		}
		if c.GetBool("API_AUTH") {
			c.Next()
			return
		}
		su, err := global.SESSION.Get(c)
		if err != nil {
			// No/!valid session — let SessionAuth / Proxy emit 401.
			c.Next()
			return
		}
		if su.IsSuper {
			c.Next()
			return
		}
		if key, gated := menuForPath(p); gated && !allowedMenu(su.Menus, key) {
			helper.ErrorWithDetail(c, http.StatusForbidden, "ErrNotLogin",
				errors.New("menu not permitted for this user"))
			c.Abort()
			return
		}
		c.Next()
	}
}
