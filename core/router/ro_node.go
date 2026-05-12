package router

import (
	v2 "github.com/1Panel-dev/1Panel/core/app/api/v2"
	"github.com/1Panel-dev/1Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type NodeRouter struct{}

func (s *NodeRouter) InitRouter(Router *gin.RouterGroup) {
	nodeRouter := Router.Group("nodes").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())
	baseApi := v2.ApiGroupApp.BaseApi
	{
		nodeRouter.POST("/search", baseApi.SearchNode)
		nodeRouter.GET("/:id", baseApi.GetNode)
		nodeRouter.POST("/update", baseApi.UpdateNode)
		nodeRouter.POST("/del/:id", baseApi.DeleteNode)
		nodeRouter.POST("/healthcheck/:id", baseApi.RecheckNode)
		nodeRouter.POST("/add", baseApi.AddNode)
	}
}
