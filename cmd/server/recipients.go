package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"watermark-tool/internal/workspace"
)

func registerRecipients(group *gin.RouterGroup, store *workspace.Store) {
	base := group.Group("/teams/:team/recipients")
	base.POST("/search", func(c *gin.Context) {
		input := struct {
			Q     string `json:"q"`
			Group string `json:"group"`
			Page  int    `json:"page"`
			Size  int    `json:"page_size"`
		}{Page: 1, Size: 20}
		if !workspaceJSON(c, &input) {
			return
		}
		found, err := store.SearchContacts(c.GetString("username"), c.Param("team"), input.Q, input.Group, input.Page, input.Size)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, found)
	})
	base.POST("", RateLimitMiddleware(), func(c *gin.Context) {
		var input workspace.ContactValue
		if !workspaceJSON(c, &input) {
			return
		}
		contact, err := store.PutContact(c.GetString("username"), c.Param("team"), "", 0, input)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(201, contact)
	})
	base.PUT("/:id", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			workspace.ContactValue
			Revision uint64 `json:"revision"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		contact, err := store.PutContact(c.GetString("username"), c.Param("team"), c.Param("id"), input.Revision, input.ContactValue)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, contact)
	})
	base.DELETE("/:id", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision uint64 `json:"revision"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		if err := store.DeleteContact(c.GetString("username"), c.Param("team"), c.Param("id"), input.Revision); err != nil {
			workspaceError(c, err)
			return
		}
		c.Status(204)
	})
	base.POST("/import", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			CSV string `json:"csv"`
		}
		if !workspaceJSON(c, &input, 2<<20) {
			return
		}
		contacts, err := store.ImportContacts(c.GetString("username"), c.Param("team"), input.CSV)
		if err != nil {
			var row *workspace.ContactImportError
			if errors.As(err, &row) {
				status := 422
				if errors.Is(err, workspace.ErrDuplicate) {
					status = 409
				}
				c.JSON(status, gin.H{"code": err.Error(), "row": row.Row})
				return
			}
			workspaceError(c, err)
			return
		}
		c.JSON(201, gin.H{"items": contacts, "imported": len(contacts)})
	})
	base.POST("/select", func(c *gin.Context) {
		var input struct {
			IDs []string `json:"ids"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		selected, err := store.SelectContacts(c.GetString("username"), c.Param("team"), input.IDs)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, selected)
	})
	base.GET("/export", func(c *gin.Context) {
		format := c.DefaultQuery("format", "json")
		if format != "json" && format != "csv" {
			workspaceError(c, workspace.ErrInvalid)
			return
		}
		contacts, err := store.ExportContacts(c.GetString("username"), c.Param("team"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=recipients."+format)
		if format == "csv" {
			c.Data(200, "text/csv; charset=utf-8", []byte(workspace.ExportContactCSV(contacts)))
			return
		}
		c.JSON(200, gin.H{"schema_version": 1, "team_id": c.Param("team"), "items": contacts})
	})
}
