package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thesabbir/hellfire/pkg/db"
	"github.com/thesabbir/hellfire/pkg/logger"
)

// Action represents an audit action
type Action string

const (
	// User actions
	ActionUserLogin  Action = "user.login"
	ActionUserLogout Action = "user.logout"
	ActionUserCreate Action = "user.create"
	ActionUserUpdate Action = "user.update"
	ActionUserDelete Action = "user.delete"

	// Config actions
	ActionConfigRead   Action = "config.read"
	ActionConfigWrite  Action = "config.write"
	ActionConfigCommit Action = "config.commit"
	ActionConfigRevert Action = "config.revert"

	// Transaction actions
	ActionTxStart    Action = "transaction.start"
	ActionTxCommit   Action = "transaction.commit"
	ActionTxRollback Action = "transaction.rollback"
	ActionTxConfirm  Action = "transaction.confirm"

	// Snapshot actions
	ActionSnapshotCreate Action = "snapshot.create"
	ActionSnapshotDelete Action = "snapshot.delete"
	ActionSnapshotRestore Action = "snapshot.restore"

	// API key actions
	ActionAPIKeyCreate Action = "apikey.create"
	ActionAPIKeyDelete Action = "apikey.delete"
	ActionAPIKeyUpdate Action = "apikey.update"

	// System actions
	ActionSystemRestart Action = "system.restart"
)

// Status represents the status of an action
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailure Status = "failure"
)

// Context keys for audit logging
const (
	ContextKeyUserID   = "audit_user_id"
	ContextKeyUsername = "audit_username"
	ContextKeyIP       = "audit_ip"
	ContextKeyTxID     = "audit_tx_id"
)

// Log creates an audit log entry
func Log(action Action, status Status, userID *uint, username, resource, message string, details interface{}) error {
	return LogWithContext(context.Background(), action, status, userID, username, resource, message, details, nil)
}

// LogWithContext creates an audit log entry with context
func LogWithContext(ctx context.Context, action Action, status Status, userID *uint, username, resource, message string, details interface{}, err error) error {
	// Extract context values if available
	if ctxUserID := ctx.Value(ContextKeyUserID); ctxUserID != nil {
		if uid, ok := ctxUserID.(uint); ok {
			userID = &uid
		}
	}

	if ctxUsername := ctx.Value(ContextKeyUsername); ctxUsername != nil {
		if uname, ok := ctxUsername.(string); ok && uname != "" {
			username = uname
		}
	}

	var ipAddress string
	if ctxIP := ctx.Value(ContextKeyIP); ctxIP != nil {
		if ip, ok := ctxIP.(string); ok {
			ipAddress = ip
		}
	}

	var txID string
	if ctxTxID := ctx.Value(ContextKeyTxID); ctxTxID != nil {
		if tid, ok := ctxTxID.(string); ok {
			txID = tid
		}
	}

	// Marshal details to JSON if provided
	var detailsJSON string
	if details != nil {
		data, err := json.Marshal(details)
		if err != nil {
			logger.Warn("Failed to marshal audit details", "error", err)
		} else {
			detailsJSON = string(data)
		}
	}

	// Create error message if provided
	var errorMsg string
	if err != nil {
		errorMsg = err.Error()
	}

	// Create audit log entry
	entry := &db.AuditLog{
		UserID:    userID,
		Username:  username,
		Action:    string(action),
		Resource:  resource,
		Status:    string(status),
		Message:   message,
		Details:   detailsJSON,
		IPAddress: ipAddress,
		Error:     errorMsg,
		TxID:      txID,
	}

	// Save to database
	if err := db.CreateAuditLog(entry); err != nil {
		logger.Error("Failed to create audit log", "error", err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	// Also log to structured logger for immediate visibility
	logFields := []interface{}{
		"action", action,
		"status", status,
		"username", username,
		"resource", resource,
	}

	if message != "" {
		logFields = append(logFields, "message", message)
	}

	if ipAddress != "" {
		logFields = append(logFields, "ip", ipAddress)
	}

	if txID != "" {
		logFields = append(logFields, "tx_id", txID)
	}

	if status == StatusSuccess {
		logger.Info("Audit", logFields...)
	} else {
		if err != nil {
			logFields = append(logFields, "error", err)
		}
		logger.Warn("Audit", logFields...)
	}

	return nil
}

// LogSuccess logs a successful action
func LogSuccess(action Action, userID *uint, username, resource, message string) error {
	return Log(action, StatusSuccess, userID, username, resource, message, nil)
}

// LogFailure logs a failed action
func LogFailure(action Action, userID *uint, username, resource, message string, err error) error {
	return LogWithContext(context.Background(), action, StatusFailure, userID, username, resource, message, nil, err)
}

// LogUserAction logs a user action with automatic user info extraction
func LogUserAction(ctx context.Context, action Action, status Status, resource, message string, details interface{}) error {
	// Try to extract user from context
	var userID *uint
	var username string

	if ctxUserID := ctx.Value(ContextKeyUserID); ctxUserID != nil {
		if uid, ok := ctxUserID.(uint); ok {
			userID = &uid
		}
	}

	if ctxUsername := ctx.Value(ContextKeyUsername); ctxUsername != nil {
		if uname, ok := ctxUsername.(string); ok {
			username = uname
		}
	}

	return LogWithContext(ctx, action, status, userID, username, resource, message, details, nil)
}

// WithUser creates a context with user information for audit logging
func WithUser(ctx context.Context, userID uint, username string) context.Context {
	ctx = context.WithValue(ctx, ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, ContextKeyUsername, username)
	return ctx
}

// WithIP creates a context with IP address for audit logging
func WithIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ContextKeyIP, ip)
}

// WithTransaction creates a context with transaction ID for audit logging
func WithTransaction(ctx context.Context, txID string) context.Context {
	return context.WithValue(ctx, ContextKeyTxID, txID)
}

// MeasuredLog logs an action with duration measurement
func MeasuredLog(action Action, status Status, userID *uint, username, resource, message string, duration time.Duration, details interface{}) error {
	entry := &db.AuditLog{
		UserID:   userID,
		Username: username,
		Action:   string(action),
		Resource: resource,
		Status:   string(status),
		Message:  message,
		Duration: duration.Milliseconds(),
	}

	if details != nil {
		data, err := json.Marshal(details)
		if err != nil {
			logger.Warn("Failed to marshal audit details", "error", err)
		} else {
			entry.Details = string(data)
		}
	}

	if err := db.CreateAuditLog(entry); err != nil {
		logger.Error("Failed to create audit log", "error", err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	return nil
}

// CleanupOldLogs removes audit logs older than the specified duration
func CleanupOldLogs(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	result := db.DB.Where("created_at < ?", cutoff).Delete(&db.AuditLog{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup old logs: %w", result.Error)
	}

	logger.Info("Cleaned up old audit logs",
		"count", result.RowsAffected,
		"older_than", olderThan)

	return result.RowsAffected, nil
}

// StartCleanupScheduler starts a background goroutine that periodically cleans up old audit logs
func StartCleanupScheduler(retentionDays int, checkInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		logger.Info("Started audit log cleanup scheduler",
			"retention_days", retentionDays,
			"check_interval", checkInterval)

		// Run cleanup immediately on start
		retention := time.Duration(retentionDays) * 24 * time.Hour
		if _, err := CleanupOldLogs(retention); err != nil {
			logger.Error("Failed to cleanup old audit logs", "error", err)
		}

		// Then run on schedule
		for range ticker.C {
			if _, err := CleanupOldLogs(retention); err != nil {
				logger.Error("Failed to cleanup old audit logs", "error", err)
			}
		}
	}()
}
