package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/workspace"
)

func openWorkspace(managed *distribution.Store) (*workspace.Store, error) {
	if managed == nil || os.Getenv("WATERMARK_ADMIN_USER") == "" || len(os.Getenv("WATERMARK_ADMIN_PASSWORD")) < 12 {
		return nil, nil
	}
	key, err := blind.DecodeKey(os.Getenv("WATERMARK_DATA_KEY"))
	if err != nil {
		return nil, err
	}
	return workspace.Open(filepath.Join(managed.RootDir(), "collaboration"), key, os.Getenv("WATERMARK_ADMIN_USER"))
}
func workspaceError(c *gin.Context, err error) {
	status := 503
	code := workspace.ErrUnavailable.Error()
	switch {
	case errors.Is(err, workspace.ErrDenied):
		status = 403
	case errors.Is(err, workspace.ErrMissing):
		status = 404
	case errors.Is(err, workspace.ErrInvalid):
		status = 422
	case errors.Is(err, workspace.ErrConflict):
		status = 409
	case errors.Is(err, workspace.ErrLimit):
		status = 422
	}
	if status != 503 {
		code = err.Error()
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}
func workspaceJSON(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(new(any)) != io.EOF {
		workspaceError(c, workspace.ErrInvalid)
		return false
	}
	return true
}

// Existing unscoped APIs belong to the configured administrator only.
func requireEnvironmentAdmin(c *gin.Context) {
	if !c.GetBool("administrator") && c.Request.URL.Path != "/api/supported-types" {
		c.AbortWithStatusJSON(403, gin.H{"code": "workspace_denied"})
		return
	}
	c.Next()
}
func registerWorkspace(api *gin.RouterGroup, store *workspace.Store, engines *teamEngines) {
	group := api.Group("/workspace", func(c *gin.Context) {
		if store == nil {
			workspaceError(c, workspace.ErrUnavailable)
			return
		}
		c.Next()
	})
	group.GET("/me", func(c *gin.Context) {
		teams, err := store.Teams(c.GetString("username"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"username": c.GetString("username"), "administrator": store.IsAdmin(c.GetString("username")), "teams": teams})
	})
	group.GET("/users", func(c *gin.Context) {
		users, err := store.Users(c.GetString("username"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": users})
	})
	group.POST("/users", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		user, err := store.CreateUser(c.GetString("username"), input.Username, input.Password)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(201, user)
	})
	group.PUT("/users/:username", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Revision uint64 `json:"revision"`
			Enabled  bool   `json:"enabled"`
			Password string `json:"password"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		user, err := store.UpdateUser(c.GetString("username"), c.Param("username"), input.Revision, input.Enabled, input.Password)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, user)
	})
	group.GET("/teams", func(c *gin.Context) {
		teams, err := store.Teams(c.GetString("username"))
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": teams})
	})
	group.POST("/teams", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Name string `json:"name"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		team, err := store.CreateTeam(c.GetString("username"), input.Name)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(201, team)
	})
	group.PUT("/teams/:team", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Name     string `json:"name"`
			Enabled  bool   `json:"enabled"`
			Revision uint64 `json:"revision"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		team, err := store.UpdateTeam(c.GetString("username"), c.Param("team"), input.Name, input.Enabled, input.Revision)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, team)
	})
	group.PUT("/teams/:team/members/:username", RateLimitMiddleware(), func(c *gin.Context) {
		var input struct {
			Role     string `json:"role"`
			Revision uint64 `json:"revision"`
		}
		if !workspaceJSON(c, &input) {
			return
		}
		team, err := store.SetMember(c.GetString("username"), c.Param("team"), c.Param("username"), input.Role, input.Revision)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, team)
	})
	group.GET("/teams/:team/events", func(c *gin.Context) {
		after, err := strconv.ParseUint(c.DefaultQuery("after", "0"), 10, 64)
		if err != nil {
			workspaceError(c, workspace.ErrInvalid)
			return
		}
		events, err := store.Events(c.GetString("username"), c.Param("team"), after, 100)
		if err != nil {
			workspaceError(c, err)
			return
		}
		c.JSON(200, gin.H{"items": events})
	})
	group.Any("/teams/:team/engine/*rest", func(c *gin.Context) {
		if engines == nil {
			workspaceError(c, workspace.ErrUnavailable)
			return
		}
		engines.proxy(c)
	})
}
