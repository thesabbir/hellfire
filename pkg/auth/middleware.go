package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thesabbir/hellfire/pkg/db"
)

const (
	// ContextKeyUser is the context key for the authenticated user
	ContextKeyUser = "user"

	// ContextKeySession is the context key for the session
	ContextKeySession = "session"
)

// AuthMiddleware is a Gin middleware that validates session tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get token from Authorization header
		token := extractToken(c)

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Get client IP and user agent for fingerprinting
		ipAddress := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Validate session with fingerprint check
		session, err := ValidateSessionWithFingerprint(token, ipAddress, userAgent)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired session",
			})
			c.Abort()
			return
		}

		// Store user and session in context
		c.Set(ContextKeyUser, &session.User)
		c.Set(ContextKeySession, session)

		c.Next()
	}
}

// RequireRole is a middleware that checks if the user has the required role
func RequireRole(roles ...db.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetUser(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRole := false
		for _, role := range roles {
			if user.Role == role {
				hasRole = true
				break
			}
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUser retrieves the authenticated user from the context
func GetUser(c *gin.Context) *db.User {
	if user, exists := c.Get(ContextKeyUser); exists {
		if u, ok := user.(*db.User); ok {
			return u
		}
	}
	return nil
}

// GetSession retrieves the session from the context
func GetSession(c *gin.Context) *db.Session {
	if session, exists := c.Get(ContextKeySession); exists {
		if s, ok := session.(*db.Session); ok {
			return s
		}
	}
	return nil
}

// extractToken extracts the token from the Authorization header or cookie
func extractToken(c *gin.Context) string {
	// Try Authorization header first (Bearer token)
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try cookie
	token, err := c.Cookie("session_token")
	if err == nil && token != "" {
		return token
	}

	return ""
}

// APIKeyMiddleware is a middleware that validates API keys
func APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API key from X-API-Key header
		apiKeyValue := c.GetHeader("X-API-Key")

		if apiKeyValue == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key required",
			})
			c.Abort()
			return
		}

		// Create SHA256 hash for fast lookup
		keyHashBytes := sha256.Sum256([]byte(apiKeyValue))
		keyHash := hex.EncodeToString(keyHashBytes[:])

		// Fast O(1) lookup by hash
		key, err := db.GetAPIKeyByKeyHash(keyHash)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			c.Abort()
			return
		}

		// Verify with bcrypt (prevents timing attacks on the actual key)
		if err := VerifyPassword(apiKeyValue, key.Key); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			c.Abort()
			return
		}

		// Update last used time (async, don't block)
		go func() {
			_ = db.UpdateAPIKeyLastUsed(key.ID)
		}()

		// Check if user is enabled
		if !key.User.Enabled {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "user account is disabled",
			})
			c.Abort()
			return
		}

		// Store user in context
		c.Set(ContextKeyUser, &key.User)

		c.Next()
	}
}

// OptionalAuthMiddleware tries to authenticate but doesn't require it
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)

		if token != "" {
			// Try to validate session
			session, err := ValidateSession(token)
			if err == nil {
				c.Set(ContextKeyUser, &session.User)
				c.Set(ContextKeySession, session)
			}
		}

		c.Next()
	}
}
