// Package auth implements the configured administrator and revocable, versioned sessions.
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
type IdentityProvider struct {
	Authenticate func(username, password string) (revision uint64, ok bool)
	Version      func(username string) (revision uint64, active bool)
}
type identity struct {
	Username string
	Revision uint64
}
type Manager struct {
	username   string
	password   [32]byte
	configured bool
	secure     bool
	mu         sync.Mutex
	sessions   map[[32]byte]time.Time
	identities map[[32]byte]identity
	provider   *IdentityProvider
	now        func() time.Time
}

func New(username, password string, secure bool) *Manager {
	return &Manager{username: username, password: sha256.Sum256([]byte(password)), configured: username != "" && len(password) >= 12, secure: secure, sessions: make(map[[32]byte]time.Time), identities: make(map[[32]byte]identity), now: time.Now}
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

// UseIdentityProvider is optional; the environment administrator remains available.
func (m *Manager) UseIdentityProvider(provider IdentityProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.provider = &provider
}
func (m *Manager) valid(c *gin.Context) bool {
	token, err := c.Cookie(cookieName)
	if err != nil || len(token) != 64 {
		return false
	}
	digest := sha256.Sum256([]byte(token))
	m.mu.Lock()
	expiry, ok := m.sessions[digest]
	who := m.identities[digest]
	provider := m.provider
	if ok && !m.now().Before(expiry) {
		delete(m.sessions, digest)
		delete(m.identities, digest)
		ok = false
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	if who.Username == "" {
		return false
	}
	if who.Username != m.username {
		if provider == nil || provider.Version == nil {
			return false
		}
		revision, active := provider.Version(who.Username)
		if !active || revision != who.Revision {
			return false
		}
	}
	c.Set("username", who.Username)
	c.Set("administrator", who.Username == m.username)
	return true
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
	c.JSON(200, gin.H{"user": gin.H{"username": c.GetString("username"), "administrator": c.GetBool("administrator")}})
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
	verified := subtle.ConstantTimeCompare(username[:], expected[:])&subtle.ConstantTimeCompare(password[:], m.password[:]) == 1
	who := identity{Username: m.username}
	if !verified && subtle.ConstantTimeCompare(username[:], expected[:]) != 1 {
		m.mu.Lock()
		provider := m.provider
		m.mu.Unlock()
		if provider != nil && provider.Authenticate != nil {
			who.Revision, verified = provider.Authenticate(input.Username, input.Password)
			who.Username = input.Username
		}
	}
	if !verified {
		c.JSON(401, gin.H{"code": "invalid_credentials"})
		return
	}
	c.Set("username", who.Username)
	c.Set("administrator", who.Username == m.username)
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
			delete(m.identities, key)
		}
	}
	// Rotate an existing session on login.
	if old, err := c.Cookie(cookieName); err == nil {
		delete(m.sessions, sha256.Sum256([]byte(old)))
		delete(m.identities, sha256.Sum256([]byte(old)))
	}
	if len(m.sessions) >= 1000 {
		m.mu.Unlock()
		c.JSON(503, gin.H{"code": "session_capacity"})
		return
	}
	m.sessions[digest] = m.now().Add(lifetime)
	m.identities[digest] = who
	m.mu.Unlock()
	m.cookie(c, token, int(lifetime.Seconds()))
	c.JSON(200, gin.H{"user": gin.H{"username": c.GetString("username"), "administrator": c.GetBool("administrator")}})
}

func (m *Manager) Logout(c *gin.Context) {
	_ = m.valid(c)
	if token, err := c.Cookie(cookieName); err == nil {
		m.mu.Lock()
		delete(m.sessions, sha256.Sum256([]byte(token)))
		delete(m.identities, sha256.Sum256([]byte(token)))
		m.mu.Unlock()
	}
	m.cookie(c, "", -1)
	c.Status(204)
}
