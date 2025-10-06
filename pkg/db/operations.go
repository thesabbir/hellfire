package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// User Operations

// CreateUser creates a new user
func CreateUser(user *User) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Create(user).Error
}

// GetUserByID retrieves a user by ID
func GetUserByID(id uint) (*User, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var user User
	if err := DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func GetUserByUsername(username string) (*User, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var user User
	if err := DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers lists all users (excluding soft-deleted)
func ListUsers() ([]User, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var users []User
	if err := DB.Order("username ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUser updates a user
func UpdateUser(user *User) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Save(user).Error
}

// DeleteUser soft-deletes a user
func DeleteUser(id uint) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Delete(&User{}, id).Error
}

// UpdateUserLastLogin updates the user's last login time
func UpdateUserLastLogin(userID uint) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now()
	return DB.Model(&User{}).Where("id = ?", userID).Update("last_login_at", now).Error
}

// Session Operations

// CreateSession creates a new session
func CreateSession(session *Session) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Create(session).Error
}

// GetSessionByToken retrieves a session by token with user preloaded
func GetSessionByToken(token string) (*Session, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var session Session
	if err := DB.Preload("User").Where("token = ?", token).First(&session).Error; err != nil {
		return nil, err
	}

	// Check if expired
	if session.IsExpired() {
		return nil, fmt.Errorf("session expired")
	}

	return &session, nil
}

// DeleteSession deletes a session
func DeleteSession(token string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Where("token = ?", token).Delete(&Session{}).Error
}

// DeleteUserSessions deletes all sessions for a user
func DeleteUserSessions(userID uint) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Where("user_id = ?", userID).Delete(&Session{}).Error
}

// CleanupExpiredSessions removes expired sessions
func CleanupExpiredSessions() (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	result := DB.Where("expires_at < ?", time.Now()).Delete(&Session{})
	return result.RowsAffected, result.Error
}

// API Key Operations

// CreateAPIKey creates a new API key
func CreateAPIKey(key *APIKey) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Create(key).Error
}

// GetAPIKeyByKey retrieves an API key by key value with user preloaded (DEPRECATED: use GetAPIKeyByKeyHash)
func GetAPIKeyByKey(key string) (*APIKey, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var apiKey APIKey
	if err := DB.Preload("User").Where("key = ?", key).First(&apiKey).Error; err != nil {
		return nil, err
	}

	// Check if valid
	if !apiKey.IsValid() {
		return nil, fmt.Errorf("API key is disabled or expired")
	}

	return &apiKey, nil
}

// GetAPIKeyByKeyHash retrieves an API key by its SHA256 hash (fast O(1) lookup)
func GetAPIKeyByKeyHash(keyHash string) (*APIKey, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var apiKey APIKey
	if err := DB.Preload("User").Where("key_hash = ?", keyHash).First(&apiKey).Error; err != nil {
		return nil, err
	}

	// Check if valid
	if !apiKey.IsValid() {
		return nil, fmt.Errorf("API key is disabled or expired")
	}

	return &apiKey, nil
}

// GetAPIKeyByID retrieves an API key by its public KeyID
func GetAPIKeyByID(keyID string) (*APIKey, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var apiKey APIKey
	if err := DB.Preload("User").Where("key_id = ?", keyID).First(&apiKey).Error; err != nil {
		return nil, err
	}

	return &apiKey, nil
}

// ListAPIKeys lists all API keys for a user
func ListAPIKeys(userID uint) ([]APIKey, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var keys []APIKey
	if err := DB.Where("user_id = ?", userID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// UpdateAPIKey updates an API key
func UpdateAPIKey(key *APIKey) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Save(key).Error
}

// DeleteAPIKey soft-deletes an API key
func DeleteAPIKey(id uint) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Delete(&APIKey{}, id).Error
}

// UpdateAPIKeyLastUsed updates the API key's last used time
func UpdateAPIKeyLastUsed(keyID uint) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now()
	return DB.Model(&APIKey{}).Where("id = ?", keyID).Update("last_used_at", now).Error
}

// Audit Log Operations

// CreateAuditLog creates a new audit log entry
func CreateAuditLog(log *AuditLog) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Create(log).Error
}

// ListAuditLogs lists audit logs with optional filters
func ListAuditLogs(filters map[string]interface{}, limit, offset int) ([]AuditLog, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var logs []AuditLog
	var count int64

	query := DB.Model(&AuditLog{})

	// Apply filters
	if userID, ok := filters["user_id"]; ok {
		query = query.Where("user_id = ?", userID)
	}
	if action, ok := filters["action"]; ok {
		query = query.Where("action = ?", action)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if resource, ok := filters["resource"]; ok {
		query = query.Where("resource = ?", resource)
	}
	if from, ok := filters["from"]; ok {
		query = query.Where("created_at >= ?", from)
	}
	if to, ok := filters["to"]; ok {
		query = query.Where("created_at <= ?", to)
	}

	// Count total
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, count, nil
}

// GetAuditLogsByTransaction retrieves audit logs for a specific transaction
func GetAuditLogsByTransaction(txID string) ([]AuditLog, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var logs []AuditLog
	if err := DB.Where("transaction_id = ?", txID).Order("created_at ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// Transaction Operations

// CreateTransaction creates a new transaction record
func CreateTransaction(tx *Transaction) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Create(tx).Error
}

// GetTransactionByID retrieves a transaction by TxID
func GetTransactionByID(txID string) (*Transaction, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var tx Transaction
	if err := DB.Preload("User").Where("tx_id = ?", txID).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

// UpdateTransaction updates a transaction
func UpdateTransaction(tx *Transaction) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	return DB.Save(tx).Error
}

// ListTransactions lists transactions with optional filters
func ListTransactions(filters map[string]interface{}, limit, offset int) ([]Transaction, int64, error) {
	if DB == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var transactions []Transaction
	var count int64

	query := DB.Model(&Transaction{}).Preload("User")

	// Apply filters
	if userID, ok := filters["user_id"]; ok {
		query = query.Where("user_id = ?", userID)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if from, ok := filters["from"]; ok {
		query = query.Where("created_at >= ?", from)
	}
	if to, ok := filters["to"]; ok {
		query = query.Where("created_at <= ?", to)
	}

	// Count total
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Get paginated results
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		return nil, 0, err
	}

	return transactions, count, nil
}

// Utility Operations

// CountUsers counts total users
func CountUsers() (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var count int64
	if err := DB.Model(&User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountSessions counts active sessions
func CountSessions() (int64, error) {
	if DB == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var count int64
	if err := DB.Model(&Session{}).Where("expires_at > ?", time.Now()).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Transaction wrapper for atomic operations
func WithTransaction(fn func(*gorm.DB) error) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	return DB.Transaction(fn)
}
