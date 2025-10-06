package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// CSRFTokenLength is the length of CSRF tokens in bytes
	CSRFTokenLength = 32

	// CSRFTokenLifetime is how long a CSRF token is valid
	CSRFTokenLifetime = 1 * time.Hour
)

// CSRFManager manages CSRF tokens
type CSRFManager struct {
	tokens map[string]time.Time
	mu     sync.RWMutex
}

// NewCSRFManager creates a new CSRF token manager
func NewCSRFManager() *CSRFManager {
	mgr := &CSRFManager{
		tokens: make(map[string]time.Time),
	}

	// Start cleanup goroutine
	go mgr.cleanup()

	return mgr
}

// GenerateToken generates a new CSRF token
func (m *CSRFManager) GenerateToken() (string, error) {
	bytes := make([]byte, CSRFTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	token := hex.EncodeToString(bytes)

	m.mu.Lock()
	m.tokens[token] = time.Now().Add(CSRFTokenLifetime)
	m.mu.Unlock()

	return token, nil
}

// ValidateToken validates a CSRF token
func (m *CSRFManager) ValidateToken(token string) bool {
	if token == "" {
		return false
	}

	m.mu.RLock()
	expiry, exists := m.tokens[token]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		// Token expired, delete it
		m.mu.Lock()
		delete(m.tokens, token)
		m.mu.Unlock()
		return false
	}

	return true
}

// DeleteToken removes a CSRF token (e.g., after use)
func (m *CSRFManager) DeleteToken(token string) {
	m.mu.Lock()
	delete(m.tokens, token)
	m.mu.Unlock()
}

// cleanup removes expired tokens periodically
func (m *CSRFManager) cleanup() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for token, expiry := range m.tokens {
			if now.After(expiry) {
				delete(m.tokens, token)
			}
		}
		m.mu.Unlock()
	}
}

// CSRFMiddleware validates CSRF tokens for state-changing requests
func CSRFMiddleware(csrfMgr *CSRFManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF for GET, HEAD, OPTIONS (safe methods)
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// Get token from header
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing",
			})
			c.Abort()
			return
		}

		// Validate token
		if !csrfMgr.ValidateToken(token) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "invalid or expired CSRF token",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetCSRFTokenHandler returns a handler that generates CSRF tokens
func GetCSRFTokenHandler(csrfMgr *CSRFManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := csrfMgr.GenerateToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to generate CSRF token",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"csrf_token": token,
			"expires_in": int(CSRFTokenLifetime.Seconds()),
		})
	}
}
