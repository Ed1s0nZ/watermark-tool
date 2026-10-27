package main

import (
	"github.com/gin-gonic/gin"
	"watermark-tool/internal/workspace"
)

func registerSharedPolicies(group *gin.RouterGroup, store *workspace.Store) {
	base := group.Group("/teams/:team/policies")
	base.GET("", func(c *gin.Context) {
		items, err := store.SharedPolicies(c.GetString("username"), c.Param("team"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": items})
	})
	save := func(c *gin.Context) {
		var input struct {
			Name     string                  `json:"name"`
			Revision uint64                  `json:"revision"`
			Content  workspace.PolicyContent `json:"content"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		result, err := store.SavePolicy(c.GetString("username"), c.Param("team"), c.Param("id"), input.Name, input.Revision, input.Content)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, result)
	}
	base.POST("", RateLimitMiddleware(), save)
	base.PUT("/:id", RateLimitMiddleware(), save)
	base.POST("/:id/publish", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision uint64 `json:"revision"`
			Version  uint64 `json:"version"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		result, err := store.PublishPolicy(c.GetString("username"), c.Param("team"), c.Param("id"), input.Revision, input.Version)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, result)
	})
}
