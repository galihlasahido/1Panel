package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type SecurityRouter struct{}

// Aggregated multi-node security view. Gated by the "security" menu via
// the global RBACEnforce middleware (see core/middleware/rbac.go).
func (s *SecurityRouter) InitRouter(Router *gin.RouterGroup) {
	securityRouter := Router.Group("security").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	baseApi := v2.ApiGroupApp.BaseApi
	{
		securityRouter.GET("/overview", baseApi.SecurityOverview)
		securityRouter.GET("/activity", baseApi.SecurityActivity)
	}
}
