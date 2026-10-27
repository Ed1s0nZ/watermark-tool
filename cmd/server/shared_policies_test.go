package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/workspace"
)

func TestSharedPolicyHTTPStrictContentAndPublish(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	store, err := workspace.Open(filepath.Join(t.TempDir(), "private"), bytes.Repeat([]byte{21}, 32), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sessions := auth.New("admin", "test-password-long", false)
	r := gin.New()
	r.POST("/login", sessions.Login)
	registerWorkspace(r.Group("/api", sessions.Require()), store, nil)
	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`)))
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	cookie := login.Result().Cookies()[0]
	call := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.AddCookie(cookie)
		request.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, request)
		return out
	}
	base := "/api/workspace/teams/personal/policies"
	content := `{"mode":"dual","template":"{name}","style":{"mode":"single","position":"center","size":28,"opacity":0.3,"color":"#123456","angle":0},"quality":{}}`
	created := call("POST", base, `{"name":"Shared","content":`+content+`}`)
	var p workspace.SharedPolicy
	if created.Code != 200 || json.Unmarshal(created.Body.Bytes(), &p) != nil {
		t.Fatal(created.Code, created.Body.String())
	}
	for _, body := range []string{
		`{"name":"Forbidden","content":` + strings.Replace(content, `"quality":{}`, `"quality":{"arbitrary":1}`, 1) + `}`,
		`{"name":"Forbidden","content":` + strings.TrimSuffix(content, "}") + `,"layout":{"layers":"logo","logo":{"position":"center","width_ratio":0.1,"opacity":0.3,"sha256":"private"}}}}`,
		`{"name":"Forbidden","team_id":"other","content":` + content + `}`,
	} {
		if out := call("POST", base, body); out.Code != 422 {
			t.Fatal("accepted extra/private fields", out.Code, out.Body.String())
		}
	}
	out := call("POST", base+"/"+p.ID+"/publish", `{"revision":1,"version":1}`)
	if out.Code != 200 {
		t.Fatal(out.Code, out.Body.String())
	}
	stale := call("PUT", base+"/"+p.ID, `{"name":"Stale","revision":1,"content":`+content+`}`)
	if stale.Code != 409 {
		t.Fatal(stale.Code)
	}
	list := call("GET", base, "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"published":1`) {
		t.Fatal(list.Code, list.Body.String())
	}
}
