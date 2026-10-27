package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/workspace"
)

func TestTeamEngineIgnoresForwardedIPs(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	key := bytes.Repeat([]byte{19}, 32)
	root, err := distribution.Open(filepath.Join(t.TempDir(), "private"), key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	tasks, err := delivery.New(root, blindSlots)
	if err != nil {
		t.Fatal(err)
	}
	defer tasks.Close()
	ws, err := workspace.Open(filepath.Join(root.RootDir(), "collaboration"), key, "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	team, err := ws.CreateTeam("admin", "A")
	if err != nil {
		t.Fatal(err)
	}
	engines, err := newTeamEngines(ws, root, tasks)
	if err != nil {
		t.Fatal(err)
	}
	defer engines.Close()
	engine, err := engines.get("admin", team.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		request := httptest.NewRequest("POST", "/api/jobs/00000000000000000000000000000000/cancel", nil)
		request.RemoteAddr = "203.0.113.10:12345"
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		request = request.WithContext(context.WithValue(request.Context(), principalKey{}, enginePrincipal{Actor: "admin", Team: team.ID}))
		out := httptest.NewRecorder()
		engine.router.ServeHTTP(out, request)
		expected := 404
		if i == 10 {
			expected = 429
		}
		if out.Code != expected {
			t.Fatalf("request %d: %d %s", i, out.Code, out.Body.String())
		}
	}
}
func TestRequestLogsUsePatternsNotPrivateIdentifiers(t *testing.T) {
	var captured bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&captured)
	defer log.SetOutput(previous)
	router := gin.New()
	router.Use(LoggingMiddleware())
	router.PUT("/users/:username", func(c *gin.Context) { c.Status(204) })
	for _, path := range []string{"/users/private-account@example.test", "/unknown-capability-secret-token"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("PUT", path, nil))
	}
	text := captured.String()
	if strings.Contains(text, "private-account") || strings.Contains(text, "secret-token") || !strings.Contains(text, "/users/:username") || !strings.Contains(text, "<unmatched>") {
		t.Fatal(text)
	}
}
