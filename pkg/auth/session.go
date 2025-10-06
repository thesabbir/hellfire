package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/thesabbir/hellfire/pkg/db"
	"github.com/thesabbir/hellfire/pkg/logger"
)

const (
	// DefaultSessionDuration is the default session lifetime (idle timeout)
	DefaultSessionDuration = 24 * time.Hour

	// AbsoluteSessionDuration is the maximum session lifetime
	AbsoluteSessionDuration = 7 * 24 * time.Hour

	// SessionTokenLength is the length of session tokens in bytes
	SessionTokenLength = 32
)

// CreateSession creates a new session for a user
func CreateSession(userID uint, ipAddress, userAgent string, duration time.Duration) (*db.Session, error) {
	if duration == 0 {
		duration = DefaultSessionDuration
	}

	// Generate secure random token
	token, err := generateSecureToken(SessionTokenLength)
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	// Generate session fingerprint
	fingerprint := generateFingerprint(ipAddress, userAgent)

	now := time.Now()

	session := &db.Session{
		Token:          token,
		UserID:         userID,
		ExpiresAt:      now.Add(duration),
		AbsoluteExpiry: now.Add(AbsoluteSessionDuration),
		IPAddress:      ipAddress,
		UserAgent:      userAgent,
		Fingerprint:    fingerprint,
	}

	if err := db.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// generateFingerprint creates a SHA256 hash of IP + User-Agent
func generateFingerprint(ipAddress, userAgent string) string {
	data := fmt.Sprintf("%s|%s", ipAddress, userAgent)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// ValidateSession validates a session token and returns the session with user
func ValidateSession(token string) (*db.Session, error) {
	if token == "" {
		return nil, fmt.Errorf("session token is required")
	}

	session, err := db.GetSessionByToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	// Check if user is enabled
	if !session.User.Enabled {
		return nil, fmt.Errorf("user account is disabled")
	}

	return session, nil
}

// ValidateSessionWithFingerprint validates a session token with fingerprint verification
func ValidateSessionWithFingerprint(token, ipAddress, userAgent string) (*db.Session, error) {
	if token == "" {
		return nil, fmt.Errorf("session token is required")
	}

	session, err := db.GetSessionByToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}

	// Validate fingerprint
	expectedFingerprint := generateFingerprint(ipAddress, userAgent)
	if session.Fingerprint != expectedFingerprint {
		// Log potential session hijacking
		logger.Warn("Session fingerprint mismatch - possible hijacking attempt",
			"user_id", session.UserID,
			"session_ip", session.IPAddress,
			"request_ip", ipAddress,
			"session_ua_hash", session.Fingerprint[:16],
			"request_ua_hash", expectedFingerprint[:16])

		// Delete suspicious session
		_ = db.DeleteSession(token)

		return nil, fmt.Errorf("session validation failed")
	}

	// Check if user is enabled
	if !session.User.Enabled {
		return nil, fmt.Errorf("user account is disabled")
	}

	return session, nil
}

// DeleteSession deletes a session (logout)
func DeleteSession(token string) error {
	return db.DeleteSession(token)
}

// DeleteAllUserSessions deletes all sessions for a user
func DeleteAllUserSessions(userID uint) error {
	return db.DeleteUserSessions(userID)
}

// CleanupExpiredSessions removes all expired sessions from the database
func CleanupExpiredSessions() (int64, error) {
	return db.CleanupExpiredSessions()
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Login authenticates a user and creates a session
func Login(username, password, ipAddress, userAgent string) (*db.Session, error) {
	// Get user by username
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is enabled
	if !user.Enabled {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login time
	if err := db.UpdateUserLastLogin(user.ID); err != nil {
		// Log error but don't fail login
		// TODO: Add proper logging
	}

	// Create session
	session, err := CreateSession(user.ID, ipAddress, userAgent, DefaultSessionDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Load user into session
	session.User = *user

	return session, nil
}

// StartSessionCleanupScheduler starts a background goroutine that periodically cleans up expired sessions
func StartSessionCleanupScheduler(checkInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		logger.Info("Started session cleanup scheduler", "check_interval", checkInterval)

		// Run cleanup immediately on start
		if count, err := CleanupExpiredSessions(); err != nil {
			logger.Error("Failed to cleanup expired sessions", "error", err)
		} else if count > 0 {
			logger.Info("Cleaned up expired sessions", "count", count)
		}

		// Then run on schedule
		for range ticker.C {
			if count, err := CleanupExpiredSessions(); err != nil {
				logger.Error("Failed to cleanup expired sessions", "error", err)
			} else if count > 0 {
				logger.Info("Cleaned up expired sessions", "count", count)
			}
		}
	}()
}
