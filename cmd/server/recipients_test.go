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

func TestRecipientHTTPAtomicSelectionAndExport(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	store, err := workspace.Open(filepath.Join(t.TempDir(), "private"), bytes.Repeat([]byte{9}, 32), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	sessions := auth.New("admin", "test-password-long", false)
	router := gin.New()
	router.POST("/login", sessions.Login)
	registerWorkspace(router.Group("/api", auth.SameOrigin(), sessions.Require()), store, nil)
	login := httptest.NewRecorder()
	router.ServeHTTP(login, httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`)))
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	cookie := login.Result().Cookies()[0]
	base := "/api/workspace/teams/personal/recipients"
	call := func(method, path string, value any) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if value != nil {
			json.NewEncoder(&body).Encode(value)
		}
		request := httptest.NewRequest(method, path, &body)
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, request)
		return out
	}
	created := call("POST", base, map[string]any{"recipient_id": "001", "name": "Client, A", "groups": []string{"Project X"}})
	var contact workspace.Contact
	if created.Code != 201 || json.Unmarshal(created.Body.Bytes(), &contact) != nil {
		t.Fatal(created.Code, created.Body.String())
	}
	duplicate := call("POST", base+"/import", map[string]string{"csv": "recipient_id,name\n002,New\n001,Conflict"})
	if duplicate.Code != 409 || !strings.Contains(duplicate.Body.String(), `"row":3`) {
		t.Fatal(duplicate.Code, duplicate.Body.String())
	}
	found := call("POST", base+"/search", map[string]string{"group": "Project X"})
	if found.Code != 200 || !strings.Contains(found.Body.String(), `"team_total":1`) {
		t.Fatal(found.Code, found.Body.String())
	}
	selected := call("POST", base+"/select", map[string]any{"ids": []string{contact.ID}})
	var snapshot workspace.ContactSelection
	if selected.Code != 200 || json.Unmarshal(selected.Body.Bytes(), &snapshot) != nil || !strings.Contains(snapshot.CSV, `"Client, A"`) {
		t.Fatal(selected.Code, selected.Body.String())
	}
	forged := call("PUT", base+"/"+contact.ID, map[string]any{"recipient_id": "001", "name": "bad", "revision": contact.Revision, "team_id": "other"})
	if forged.Code != 422 {
		t.Fatal("client moved a contact across teams", forged.Code)
	}
	changed := call("PUT", base+"/"+contact.ID, map[string]any{"recipient_id": "001", "name": "Renamed", "revision": contact.Revision, "groups": []string{}})
	if changed.Code != 200 || !strings.Contains(snapshot.CSV, "Client, A") {
		t.Fatal(changed.Code, "snapshot mutated")
	}
	exported := call("GET", base+"/export?format=json", nil)
	if exported.Code != 200 || !strings.Contains(exported.Body.String(), "Renamed") || !strings.Contains(exported.Header().Get("Content-Disposition"), "recipients.json") {
		t.Fatal(exported.Code, exported.Body.String())
	}
	if out := call("GET", base+"/export?format=exe", nil); out.Code != 422 {
		t.Fatal(out.Code)
	}
	if out := call("DELETE", base+"/"+contact.ID, map[string]any{"revision": contact.Revision}); out.Code != 409 {
		t.Fatal("stale deletion accepted", out.Code)
	}
	if out := call("DELETE", base+"/"+contact.ID, map[string]any{"revision": contact.Revision + 1}); out.Code != 204 {
		t.Fatal(out.Code, out.Body.String())
	}
}
