package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/service"
	"watermark-tool/internal/workspace"
)

func TestWorkspaceHTTPIsolationAndRoles(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	key := bytes.Repeat([]byte{32}, 32)
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
	alice, err := ws.CreateUser("admin", "alice", "alice-password-2026")
	if err != nil {
		t.Fatal(err)
	}
	ws.CreateUser("admin", "bob", "bob-password-2026")
	a, _ := ws.CreateTeam("admin", "A")
	b, _ := ws.CreateTeam("admin", "B")
	a, err = ws.SetMember("admin", a.ID, "alice", workspace.RoleOperator, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	b, err = ws.SetMember("admin", b.ID, "bob", workspace.RoleOperator, b.Revision)
	if err != nil {
		t.Fatal(err)
	}
	engines, err := newTeamEngines(ws, root, tasks)
	if err != nil {
		t.Fatal(err)
	}
	defer engines.Close()
	sessions := auth.New("admin", "admin-password-2026", false)
	sessions.UseIdentityProvider(auth.IdentityProvider{Authenticate: ws.Authenticate, Version: ws.CredentialVersion})
	router := gin.New()
	router.POST("/login", sessions.Login)
	api := router.Group("/api", auth.SameOrigin(), sessions.Require())
	registerWorkspace(api, ws, engines)
	api.Use(requireEnvironmentAdmin)
	registerPlatform(api, root, root.RuntimeDir())
	registerLegacy(api, root, service.NewWatermarkService(), root.RuntimeDir())
	call := func(method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
		var encoded bytes.Buffer
		if body != nil {
			json.NewEncoder(&encoded).Encode(body)
		}
		request := httptest.NewRequest(method, path, &encoded)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Username", "admin")
		request.Header.Set("X-Team-ID", a.ID)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, request)
		return out
	}
	login := func(username, password string) *http.Cookie {
		out := call("POST", "/login", map[string]string{"username": username, "password": password}, nil)
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
		return out.Result().Cookies()[0]
	}
	ac := login("alice", "alice-password-2026")
	bc := login("bob", "bob-password-2026")
	admin := login("admin", "admin-password-2026")
	prefix := func(team string) string { return "/api/workspace/teams/" + team + "/engine/api" }
	if out := call("GET", "/api/workspace/me", nil, nil); out.Code != 401 {
		t.Fatal("anonymous", out.Code)
	}
	if out := call("GET", "/api/workspace/users", nil, ac); out.Code != 403 {
		t.Fatal("user directory leaked", out.Code)
	}
	if out := call("GET", "/api/workspace/teams", nil, ac); out.Code != 200 || strings.Contains(out.Body.String(), b.ID) || strings.Contains(out.Body.String(), workspace.PersonalTeam) {
		t.Fatal("membership enumeration", out.Code, out.Body.String())
	}
	if out := call("POST", "/api/platform/records/search", map[string]any{}, ac); out.Code != 403 {
		t.Fatal("legacy records exposed", out.Code)
	}
	if out := call("GET", prefix(b.ID)+"/platform/keys", nil, ac); out.Code != 403 {
		t.Fatal("cross-team keys", out.Code)
	}
	getKeys := func(team string, cookie *http.Cookie) string {
		out := call("GET", prefix(team)+"/platform/keys", nil, cookie)
		if out.Code != 200 {
			t.Fatal(out.Code, out.Body.String())
		}
		return out.Body.String()
	}
	keysA := getKeys(a.ID, ac)
	keysB := getKeys(b.ID, bc)
	if keysA == keysB {
		t.Fatal("team keys reused")
	}
	source := imageFixture()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	writer.WriteField("recipients_csv", "recipient_id,name\n001,Alice")
	file, _ := writer.CreateFormFile("file", "original.png")
	file.Write(source)
	writer.Close()
	request := httptest.NewRequest("POST", prefix(a.ID)+"/platform/batches", &form)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "team-a-real-batch-0001")
	request.AddCookie(ac)
	generated := httptest.NewRecorder()
	router.ServeHTTP(generated, request)
	if generated.Code != 200 {
		t.Fatal("team generation", generated.Code, generated.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(generated.Body.Bytes()), int64(generated.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var marked []byte
	for _, f := range archive.File {
		if strings.HasSuffix(f.Name, ".png") {
			r, _ := f.Open()
			marked, _ = io.ReadAll(r)
			r.Close()
			break
		}
	}
	if len(marked) == 0 {
		t.Fatal("no actual image")
	}
	for _, tc := range []struct {
		team    string
		cookie  *http.Cookie
		matched bool
	}{{a.ID, ac, true}, {b.ID, bc, false}} {
		var input bytes.Buffer
		mw := multipart.NewWriter(&input)
		part, _ := mw.CreateFormFile("file", "copy.png")
		part.Write(marked)
		mw.Close()
		req := httptest.NewRequest("POST", prefix(tc.team)+"/platform/trace", &input)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.AddCookie(tc.cookie)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		if tc.matched && (out.Code != 200 || !strings.Contains(out.Body.String(), `"match":"registered"`)) {
			t.Fatal("own team trace", out.Code, out.Body.String())
		}
		if !tc.matched && out.Code == 200 && strings.Contains(out.Body.String(), `"match":"registered"`) {
			t.Fatal("cross team matched")
		}
	}
	out := call("POST", prefix(b.ID)+"/platform/records/search", map[string]any{}, bc)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"total":0`) {
		t.Fatal("team records mixed", out.Code, out.Body.String())
	}
	if out := call("POST", prefix(a.ID)+"/platform/keys", map[string]any{}, ac); out.Code != 403 {
		t.Fatal("operator key management", out.Code)
	}
	a, err = ws.SetMember("admin", a.ID, "alice", workspace.RoleReviewer, a.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if out := call("POST", prefix(a.ID)+"/jobs", map[string]any{}, ac); out.Code != 403 {
		t.Fatal("live role downgrade", out.Code)
	}
	if out := call("POST", prefix(a.ID)+"/platform/records/search", map[string]any{}, ac); out.Code != 200 {
		t.Fatal("review read", out.Code)
	}
	if out := call("PUT", "/api/workspace/teams/"+a.ID+"/members/alice", map[string]any{"revision": a.Revision, "role": "owner"}, ac); out.Code != 403 {
		t.Fatal("self escalation", out.Code)
	}
	if out := call("POST", "/api/workspace/users", map[string]any{"username": "carol", "password": "carol-password-2026", "role": "admin"}, admin); out.Code != 422 {
		t.Fatal("unknown fields", out.Code)
	}
	scoped, err := engines.get("admin", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(scoped.store.RuntimeDir(), "orphan.tmp")
	freshPath := filepath.Join(scoped.store.RuntimeDir(), "fresh.tmp")
	if err = os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(freshPath, []byte("fresh"), 0600)
	oldTime := time.Now().Add(-2 * time.Hour)
	os.Chtimes(oldPath, oldTime, oldTime)
	engines.cleanRuntime(time.Now())
	if _, err = os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expired runtime file retained", err)
	}
	if _, err = os.Stat(freshPath); err != nil {
		t.Fatal("fresh input removed", err)
	}
	_, err = ws.UpdateUser("admin", "alice", alice.Revision, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if out := call("GET", prefix(a.ID)+"/platform/keys", nil, ac); out.Code != 401 {
		t.Fatal("disabled identity", out.Code)
	}
}
