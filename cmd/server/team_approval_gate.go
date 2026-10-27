package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"watermark-tool/internal/workspace"
)

func teamApprovalGate(c *gin.Context, store *workspace.Store, actor, team string) bool {
	t, err := store.Team(actor, team)
	if err != nil {
		workspaceError(c, err)
		return false
	}
	if !t.RequireApproval {
		return true
	}
	if c.Request.Method == http.MethodPost {
		if strings.HasPrefix(c.Request.URL.Path, "/api/platform/batches/") && c.Request.URL.Path != "/api/platform/batches/search" {
			workspaceError(c, workspace.ErrApproval)
			return false
		}
		switch c.Request.URL.Path {
		case "/api/platform/batches", "/api/add-watermark", "/api/workbench/dual", "/api/workbench/lab":
			workspaceError(c, workspace.ErrApproval)
			return false
		}
	}
	return true
}
