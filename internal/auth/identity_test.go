package auth

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"watermark-tool/internal/workspace"
)

func TestVersionedMemberSessions(t *testing.T) {
	store, err := workspace.Open(filepath.Join(t.TempDir(), "private"), bytes.Repeat([]byte{41}, 32), "admin")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.CreateUser("admin", "alice", "alice-password-123")
	if err != nil {
		t.Fatal(err)
	}
	manager := New("admin", "test-password-long", false)
	manager.UseIdentityProvider(IdentityProvider{store.Authenticate, store.CredentialVersion})
	r := router(manager)
	login := request(r, "POST", "/login", `{"username":"alice","password":"alice-password-123"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	assertIdentity := func(w *httptest.ResponseRecorder, name string, admin bool) {
		t.Helper()
		var response struct {
			User struct {
				Username string `json:"username"`
				Admin    bool   `json:"administrator"`
			} `json:"user"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.User.Username != name || response.User.Admin != admin {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	assertIdentity(request(r, "GET", "/session", "", cookie), "alice", false)
	if w := request(r, "POST", "/protected", "", cookie); w.Code != 204 {
		t.Fatal(w.Code)
	}
	user, err = store.UpdateUser("admin", "alice", user.Revision, true, "new-password-alice")
	if err != nil {
		t.Fatal(err)
	}
	if w := request(r, "GET", "/session", "", cookie); w.Code != 401 {
		t.Fatal("old password session survived reset", w.Code)
	}
	login = request(r, "POST", "/login", `{"username":"alice","password":"new-password-alice"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	cookie = login.Result().Cookies()[0]
	_, err = store.UpdateUser("admin", "alice", user.Revision, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if w := request(r, "POST", "/protected", "", cookie); w.Code != 401 {
		t.Fatal("disabled user session accepted", w.Code)
	}
	if w := request(r, "POST", "/login", `{"username":"alice","password":"new-password-alice"}`, nil); w.Code != 401 {
		t.Fatal("disabled login", w.Code)
	}
	root := request(r, "POST", "/login", `{"username":"admin","password":"test-password-long"}`, nil)
	if root.Code != 200 {
		t.Fatal(root.Code)
	}
	assertIdentity(request(r, "GET", "/session", "", root.Result().Cookies()[0]), "admin", true)
	if w := request(r, "POST", "/login", `{"username":"admin","password":"new-password-alice"}`, nil); w.Code != 401 {
		t.Fatal("member password used for admin")
	}
}
func TestProviderRevisionCannotRacePasswordReset(t *testing.T) {
	m := New("admin", "test-password-long", false)
	m.UseIdentityProvider(IdentityProvider{
		Authenticate: func(string, string) (uint64, bool) { return 1, true },
		Version:      func(string) (uint64, bool) { return 2, true },
	})
	r := router(m)
	login := request(r, "POST", "/login", `{"username":"alice","password":"old-password"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Code)
	}
	if w := request(r, "GET", "/session", "", login.Result().Cookies()[0]); w.Code != 401 {
		t.Fatal("credential reset race stamped a fresh revision")
	}
}
