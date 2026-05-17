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
		// Static list routes MUST be registered before the `/:id`
		// param route — the frontend node picker calls GET /nodes/all
		// & /nodes/simple/all, which otherwise fall into GetNode and
		// 400 with "invalid id".
		nodeRouter.GET("/all", baseApi.ListAllNodes)
		nodeRouter.GET("/simple/all", baseApi.ListSimpleNodes)
		nodeRouter.POST("/list", baseApi.ListNodes)
		nodeRouter.GET("/:id", baseApi.GetNode)
		// Node lifecycle is superadmin-only — a sub-user may select
		// among its allowed nodes (read/list, scoped in the handler)
		// but never enroll/modify/delete or force health checks.
		nodeRouter.POST("/update", middleware.SuperAdmin(), baseApi.UpdateNode)
		nodeRouter.POST("/del/:id", middleware.SuperAdmin(), baseApi.DeleteNode)
		nodeRouter.POST("/healthcheck/:id", middleware.SuperAdmin(), baseApi.RecheckNode)
		nodeRouter.POST("/add", middleware.SuperAdmin(), baseApi.AddNode)
	}
}
