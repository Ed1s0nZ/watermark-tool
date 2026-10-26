package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func router(m *Manager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOrigin())
	r.POST("/login", m.Login)
	r.GET("/session", m.Session)
	r.POST("/logout", m.Logout)
	r.POST("/protected", m.Require(), func(c *gin.Context) { c.Status(204) })
	return r
}
func request(r http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestSessionLifecycle(t *testing.T) {
	m := New("admin", "test-password-long", true)
	r := router(m)
	if w := request(r, "POST", "/protected", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := request(r, "POST", "/login", `{"username":"admin","password":"wrong"}`, nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
	w := request(r, "POST", "/login", `{"username":"admin","password":"test-password-long"}`, nil)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge != 28800 {
		t.Fatal(cookie)
	}
	for _, path := range []string{"/session", "/protected"} {
		method := "GET"
		expected := 200
		if path == "/protected" {
			method = "POST"
			expected = 204
		}
		if w := request(r, method, path, "", cookie); w.Code != expected {
			t.Fatal(path, w.Code)
		}
	}
	if w := request(r, "POST", "/logout", "", cookie); w.Code != 204 || w.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal(w)
	}
	if w := request(r, "POST", "/protected", "", cookie); w.Code != 401 {
		t.Fatal("revoked session accepted")
	}
}
func TestExpiryRotationAndUnconfigured(t *testing.T) {
	m := New("admin", "test-password-long", false)
	r := router(m)
	login := func(cookie *http.Cookie) *http.Cookie {
		return request(r, "POST", "/login", `{"username":"admin","password":"test-password-long"}`, cookie).Result().Cookies()[0]
	}
	old := login(nil)
	current := login(old)
	if w := request(r, "GET", "/session", "", old); w.Code != 401 {
		t.Fatal("rotation failed")
	}
	m.now = func() time.Time { return time.Now().Add(9 * time.Hour) }
	if w := request(r, "GET", "/session", "", current); w.Code != 401 {
		t.Fatal("expiry failed")
	}
	for _, password := range []string{"", "short"} {
		if w := request(router(New("admin", password, false)), "POST", "/login", `{}`, nil); w.Code != 503 {
			t.Fatal("missing config not rejected")
		}
	}
}
func TestCrossOriginAndMalformedLogin(t *testing.T) {
	r := router(New("admin", "test-password-long", false))
	for _, origin := range []string{"https://evil.example", "null"} {
		req := httptest.NewRequest("POST", "/login", strings.NewReader(`{}`))
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal(origin, w.Code)
		}
	}
	if w := request(r, "POST", "/login", strings.Repeat("x", 5000), nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
	req := httptest.NewRequest("POST", "/logout", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}

func TestConcurrentSessions(t *testing.T) {
	r := router(New("admin", "test-password-long", false))
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			login := request(r, "POST", "/login", `{"username":"admin","password":"test-password-long"}`, nil)
			if login.Code != 200 {
				t.Errorf("login: %d", login.Code)
				return
			}
			cookie := login.Result().Cookies()[0]
			if w := request(r, "GET", "/session", "", cookie); w.Code != 200 {
				t.Errorf("session: %d", w.Code)
			}
			request(r, "POST", "/logout", "", cookie)
			if w := request(r, "GET", "/session", "", cookie); w.Code != 401 {
				t.Errorf("revoked: %d", w.Code)
			}
		}()
	}
	group.Wait()
}
