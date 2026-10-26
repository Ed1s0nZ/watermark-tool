// Package auth implements a single configured administrator and revocable sessions.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const cookieName = "watermark_session"
const lifetime = 8 * time.Hour

// Manager holds only session token digests. Restarting the server revokes all sessions.
type Manager struct {
	username   string
	password   [32]byte
	configured bool
	secure     bool
	mu         sync.Mutex
	sessions   map[[32]byte]time.Time
	now        func() time.Time
}

func New(username, password string, secure bool) *Manager {
	return &Manager{username: username, password: sha256.Sum256([]byte(password)), configured: username != "" && len(password) >= 12, secure: secure, sessions: make(map[[32]byte]time.Time), now: time.Now}
}

// SameOrigin rejects browser cross-origin writes while permitting non-browser clients.
func SameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			origin := c.GetHeader("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host != c.Request.Host {
					c.AbortWithStatusJSON(403, gin.H{"code": "origin_rejected", "error": "Cross-origin request rejected"})
					return
				}
			}
			if c.GetHeader("Sec-Fetch-Site") == "cross-site" {
				c.AbortWithStatusJSON(403, gin.H{"code": "origin_rejected", "error": "Cross-origin request rejected"})
				return
			}
		}
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func (m *Manager) available(c *gin.Context) bool {
	if !m.configured {
		c.AbortWithStatusJSON(503, gin.H{"code": "auth_unconfigured", "error": "Configure WATERMARK_ADMIN_USER and WATERMARK_ADMIN_PASSWORD (at least 12 bytes)"})
		return false
	}
	return true
}

func (m *Manager) valid(c *gin.Context) bool {
	token, err := c.Cookie(cookieName)
	if err != nil || len(token) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	m.mu.Lock()
	defer m.mu.Unlock()
	expiry, ok := m.sessions[digest]
	if ok && !m.now().Before(expiry) {
		delete(m.sessions, digest)
		return false
	}
	return ok
}

func (m *Manager) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !m.available(c) {
			return
		}
		if !m.valid(c) {
			c.AbortWithStatusJSON(401, gin.H{"code": "unauthorized", "error": "Authentication required"})
			return
		}
		c.Set("username", m.username)
		c.Next()
	}
}

func (m *Manager) Session(c *gin.Context) {
	if !m.available(c) {
		return
	}
	if !m.valid(c) {
		c.JSON(401, gin.H{"code": "unauthorized"})
		return
	}
	c.JSON(200, gin.H{"user": gin.H{"username": m.username}})
}

func (m *Manager) cookie(c *gin.Context, token string, age int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: cookieName, Value: token, Path: "/", MaxAge: age, HttpOnly: true, Secure: m.secure, SameSite: http.SameSiteStrictMode})
}

func (m *Manager) Login(c *gin.Context) {
	if !m.available(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(400, gin.H{"code": "invalid_request"})
		return
	}
	username := sha256.Sum256([]byte(input.Username))
	expected := sha256.Sum256([]byte(m.username))
	password := sha256.Sum256([]byte(input.Password))
	if subtle.ConstantTimeCompare(username[:], expected[:])&subtle.ConstantTimeCompare(password[:], m.password[:]) != 1 {
		c.JSON(401, gin.H{"code": "invalid_credentials"})
		return
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(500, gin.H{"code": "session_failed"})
		return
	}
	token := hex.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	m.mu.Lock()
	for key, expiry := range m.sessions {
		if !m.now().Before(expiry) {
			delete(m.sessions, key)
		}
	}
	// Rotate an existing session on login.
	if old, err := c.Cookie(cookieName); err == nil {
		delete(m.sessions, sha256.Sum256([]byte(old)))
	}
	if len(m.sessions) >= 1000 {
		m.mu.Unlock()
		c.JSON(503, gin.H{"code": "session_capacity"})
		return
	}
	m.sessions[digest] = m.now().Add(lifetime)
	m.mu.Unlock()
	m.cookie(c, token, int(lifetime.Seconds()))
	c.JSON(200, gin.H{"user": gin.H{"username": m.username}})
}

func (m *Manager) Logout(c *gin.Context) {
	if token, err := c.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, sha256.Sum256([]byte(token)))
		m.mu.Unlock()
	}
	m.cookie(c, "", -1)
	c.Status(204)
}
