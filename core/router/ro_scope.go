package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type ScopeRouter struct{}

// Saved fleet scopes. Reads are available to any node-access user;
// mutations are superadmin-only (they shape fleet topology).
func (s *ScopeRouter) InitRouter(Router *gin.RouterGroup) {
	scopeRouter := Router.Group("scopes").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	baseApi := v2.ApiGroupApp.BaseApi
	{
		scopeRouter.GET("/search", baseApi.ListScopes)
		scopeRouter.POST("/create", middleware.SuperAdmin(), baseApi.CreateScope)
		scopeRouter.POST("/update", middleware.SuperAdmin(), baseApi.UpdateScope)
		scopeRouter.POST("/del/:id", middleware.SuperAdmin(), baseApi.DeleteScope)
	}
}
