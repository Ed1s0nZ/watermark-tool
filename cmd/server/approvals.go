package main

import (
	"github.com/gin-gonic/gin"
	"watermark-tool/internal/workspace"
)

func registerApprovals(group *gin.RouterGroup, store *workspace.Store, engines *teamEngines) {
	base := group.Group("/teams/:team")
	base.PUT("/approval-settings", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision uint64 `json:"revision"`
			Required *bool  `json:"require_approval"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		if input.Required == nil {
			workspaceError(c, workspace.ErrInvalid)
			return
		}
		result, err := store.SetApprovalRequired(c.GetString("username"), c.Param("team"), input.Revision, *input.Required)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, result)
	})
	base.POST("/approvals/search", func(c *gin.Context) {
		input := struct {
			Page   int    `json:"page"`
			Size   int    `json:"page_size"`
			Status string `json:"status"`
		}{Page: 1, Size: 20}
		if !workspaceJSON(c, &input) {
			return
		}
		if input.Page < 1 || input.Page > 1000000 || input.Size < 1 || input.Size > 50 || (input.Status != "" && input.Status != "pending" && input.Status != "approved" && input.Status != "rejected" && input.Status != "withdrawn") {
			workspaceError(c, workspace.ErrInvalid)
			return
		}
		items, err := store.Approvals(c.GetString("username"), c.Param("team"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		filtered := []workspace.Approval{}
		for _, item := range items {
			if input.Status == "" || item.Status == input.Status {
				filtered = append(filtered, item)
			}
		}
		total := len(filtered)
		start := (input.Page - 1) * input.Size
		if start >= total {
			filtered = []workspace.Approval{}
		} else {
			end := start + input.Size
			if end > total {
				end = total
			}
			filtered = filtered[start:end]
		}
		c.JSON(200, gin.H{"items": filtered, "total": total})
	})
	jobs := base.Group("/approvals/:job", func(c *gin.Context) {
		if err := store.Authorize(c.GetString("username"), c.Param("team"), "read"); err != nil {
			workspaceError(c, err)
			return
		}
		if engines == nil {
			workspaceError(c, workspace.ErrUnavailable)
			return
		}
		c.Next()
	})
	jobs.GET("", func(c *gin.Context) {
		result, err := store.ApprovalFor(c.GetString("username"), c.Param("team"), c.Param("job"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"approval": result})
	})
	jobs.POST("/request", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision uint64 `json:"revision"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		actor, team := c.GetString("username"), c.Param("team")
		if err := store.Authorize(actor, team, "produce"); err != nil {
			workspaceError(c, err)
			return
		}
		engine, err := engines.get(actor, team)
		if err != nil {
			workspaceError(c, err)
			return
		}
		j, err := engine.manager.ApprovalInputs(c.Param("job"))
		if err != nil {
			platformError(c, err)
			return
		}
		result, err := store.RequestApproval(actor, team, j, input.Revision)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, result)
	})
	jobs.POST("/decision", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision    uint64 `json:"revision"`
			Status      string `json:"status"`
			Reason      string `json:"reason"`
			ReviewToken string `json:"review_token"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		actor, team := c.GetString("username"), c.Param("team")
		engine, err := engines.get(actor, team)
		if err != nil {
			workspaceError(c, err)
			return
		}
		j, err := engine.store.Job(c.Param("job"))
		if err != nil {
			platformError(c, err)
			return
		}
		if input.Status == "approved" {
			j, err = engine.manager.ApprovalInputs(j.ID)
			if err != nil {
				platformError(c, err)
				return
			}
		}
		result, err := store.DecideApproval(actor, team, j, input.Revision, input.Status, input.Reason, input.ReviewToken)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, result)
	})
	jobs.GET("/thumbnail", rateLimitBucket("approval-thumbnail/", 120), func(c *gin.Context) {
		engine, err := engines.get(c.GetString("username"), c.Param("team"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		select {
		case blindSlots <- struct{}{}:
			defer func() { <-blindSlots }()
		default:
			c.JSON(429, gin.H{"code": "tooMany"})
			return
		}
		data, err := engine.manager.ApprovalThumbnail(c.Param("job"), c.Query("source_id"), c.Query("copy_id"))
		if err != nil {
			platformError(c, err)
			return
		}
		if j, e := engine.store.Job(c.Param("job")); e == nil {
			if receipt, e := store.ReviewReceipt(c.GetString("username"), c.Param("team"), j); e == nil {
				c.Header("X-Review-Token", receipt)
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Header("Content-Disposition", "inline; filename=preview.jpg")
		c.Data(200, "image/jpeg", data)
	})
}
