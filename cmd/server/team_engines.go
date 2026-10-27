package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/service"
	"watermark-tool/internal/workspace"
)

type enginePrincipal struct{ Actor, Team string }
type principalKey struct{}
type scopedEngine struct {
	router  *gin.Engine
	store   *distribution.Store
	manager *delivery.Manager
	owned   bool
}
type teamEngines struct {
	mu           sync.Mutex
	workspace    *workspace.Store
	root         *distribution.Store
	manager      *delivery.Manager
	options      delivery.Options
	entries      map[string]*scopedEngine
	closed       bool
	stop         context.CancelFunc
	done         chan struct{}
	cleanupFault map[string]bool
}

func newTeamEngines(store *workspace.Store, root *distribution.Store, manager *delivery.Manager) (*teamEngines, error) {
	options, err := deliveryOptions()
	if err != nil {
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	r := &teamEngines{workspace: store, root: root, manager: manager, options: options, entries: map[string]*scopedEngine{}, stop: stop, done: make(chan struct{}), cleanupFault: map[string]bool{}}
	go r.maintain(ctx)
	return r, nil
}
func enginePermission(method, path string) string {
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}
	path = strings.TrimPrefix(path, "/api")
	if method == http.MethodGet {
		if strings.HasPrefix(path, "/jobs/") && strings.HasSuffix(path, "/archive") {
			return "produce"
		}
		if path == "/supported-types" || path == "/platform/keys" || path == "/audit/status" || strings.HasPrefix(path, "/jobs/") {
			return "read"
		}
	}
	if method == http.MethodPost {
		switch path {
		case "/jobs/search", "/platform/batches/search", "/platform/records/search", "/audit/search", "/audit/export", "/workbench/verify", "/platform/trace", "/workbench/trace-batch", "/extract-watermark":
			return "read"
		case "/jobs", "/jobs/preview-plan", "/platform/batches", "/add-watermark", "/workbench/dual", "/workbench/lab":
			return "produce"
		}
		if path == "/platform/keys" || strings.HasPrefix(path, "/platform/keys/") {
			return "manage"
		}
		if strings.HasPrefix(path, "/jobs/") || strings.HasPrefix(path, "/platform/batches/") {
			return "produce"
		}
	}
	return ""
}
func (r *teamEngines) get(actor, team string) (*scopedEngine, error) {
	if r.workspace == nil || r.root == nil || r.manager == nil {
		return nil, workspace.ErrUnavailable
	}
	if err := r.workspace.Authorize(actor, team, "read"); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, workspace.ErrUnavailable
	}
	if current := r.entries[team]; current != nil {
		return current, nil
	}
	scoped := &scopedEngine{store: r.root, manager: r.manager}
	if team != workspace.PersonalTeam {
		key, err := r.workspace.EngineKey(team)
		if err != nil {
			return nil, err
		}
		base := filepath.Join(r.root.RootDir(), "team-engines")
		if err = os.MkdirAll(base, 0700); err != nil {
			return nil, workspace.ErrUnavailable
		}
		info, err := os.Lstat(base)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return nil, workspace.ErrUnavailable
		}
		scoped.store, err = distribution.Open(filepath.Join(base, team), key, nil)
		if err != nil {
			return nil, err
		}
		options := r.options
		options.Authorize = func(j distribution.Job) error {
			err := r.workspace.CheckJobApproval(team, j)
			if errors.Is(err, workspace.ErrUnavailable) {
				return distribution.ErrStore
			}
			return err
		}
		scoped.manager, err = delivery.New(scoped.store, blindSlots, options)
		if err != nil {
			scoped.store.Close()
			return nil, err
		}
		scoped.owned = true
	}
	engine := gin.New()
	engine.SetTrustedProxies(nil)
	engine.Use(gin.Recovery())
	api := engine.Group("/api", func(c *gin.Context) {
		who, ok := c.Request.Context().Value(principalKey{}).(enginePrincipal)
		if !ok || who.Team != team {
			workspaceError(c, workspace.ErrDenied)
			return
		}
		permission := enginePermission(c.Request.Method, c.Request.URL.Path)
		if permission == "" {
			workspaceError(c, workspace.ErrDenied)
			return
		}
		if err := r.workspace.Authorize(who.Actor, team, permission); err != nil {
			workspaceError(c, err)
			return
		}
		if !teamApprovalGate(c, r.workspace, who.Actor, team) {
			return
		}
		c.Set("username", who.Actor)
		c.Next()
	}, auditMiddleware(scoped.store))
	registerJobs(api, scoped.store, scoped.manager, scoped.store.RuntimeDir())
	registerAudit(api, scoped.store, scoped.manager)
	registerBatchTrace(api, scoped.store, scoped.store.RuntimeDir())
	registerWorkbench(api, scoped.store, scoped.store.RuntimeDir())
	api.Use(func(c *gin.Context) {
		if c.Request.Method == http.MethodPost {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxFileSize+(1<<20))
			defer func() {
				if c.Request.MultipartForm != nil {
					c.Request.MultipartForm.RemoveAll()
				}
			}()
		}
		c.Next()
	})
	registerPlatform(api, scoped.store, scoped.store.RuntimeDir())
	registerLegacy(api, scoped.store, service.NewWatermarkService(), scoped.store.RuntimeDir())
	scoped.router = engine
	r.entries[team] = scoped
	return scoped, nil
}
func (r *teamEngines) proxy(c *gin.Context) {
	team := c.Param("team")
	actor := c.GetString("username")
	permission := enginePermission(c.Request.Method, c.Param("rest"))
	if permission == "" {
		workspaceError(c, workspace.ErrDenied)
		return
	}
	if err := r.workspace.Authorize(actor, team, permission); err != nil {
		workspaceError(c, err)
		return
	}
	engine, err := r.get(actor, team)
	if err != nil {
		workspaceError(c, err)
		return
	}
	request := c.Request.Clone(context.WithValue(c.Request.Context(), principalKey{}, enginePrincipal{Actor: actor, Team: team}))
	request.URL.Path = c.Param("rest")
	request.URL.RawPath = ""
	engine.router.ServeHTTP(c.Writer, request)
}
func (r *teamEngines) Close() {
	r.stop()
	<-r.done
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	for _, engine := range r.entries {
		if engine.owned {
			engine.manager.Close()
			engine.store.Close()
		}
	}
}
