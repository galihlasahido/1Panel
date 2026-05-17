package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type UserRouter struct{}

// RBAC user management — superadmin only (settings UserName). Even a
// sub-user granted the "settings" menu cannot reach these routes.
func (s *UserRouter) InitRouter(Router *gin.RouterGroup) {
	userRouter := Router.Group("users").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired()).
		Use(middleware.SuperAdmin())
	baseApi := v2.ApiGroupApp.BaseApi
	{
		userRouter.POST("/search", baseApi.SearchUser)
		userRouter.POST("/create", baseApi.CreateUser)
		userRouter.POST("/update", baseApi.UpdateUser)
		userRouter.POST("/update/password", baseApi.UpdateUserPassword)
		userRouter.POST("/del/:id", baseApi.DeleteUser)
		userRouter.GET("/:id", baseApi.GetUser)
	}
}
