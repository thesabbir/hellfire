package db

import (
	"time"

	"gorm.io/gorm"
)

// Role represents user roles
type Role string

const (
	RoleAdmin    Role = "admin"    // Full access
	RoleOperator Role = "operator" // Read + write (no user management)
	RoleViewer   Role = "viewer"   // Read-only
)

// User represents a system user
type User struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"` // Soft delete
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"not null" json:"-"` // Never expose in JSON
	Email        string         `gorm:"index" json:"email"`
	Role         Role           `gorm:"not null;default:'viewer'" json:"role"`
	Enabled      bool           `gorm:"not null;default:true" json:"enabled"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
}

// TableName overrides the table name
func (User) TableName() string {
	return "users"
}

// Session represents an active user session
type Session struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Token          string    `gorm:"uniqueIndex;not null" json:"token"` // Session token
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	User           User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ExpiresAt      time.Time `gorm:"not null;index" json:"expires_at"`          // Idle timeout
	AbsoluteExpiry time.Time `gorm:"not null;index" json:"absolute_expiry"`     // Max session lifetime
	IPAddress      string    `gorm:"index" json:"ip_address"`
	UserAgent      string    `json:"user_agent"`
	Fingerprint    string    `gorm:"not null;index" json:"fingerprint"` // SHA256(IP + UA)
}

// TableName overrides the table name
func (Session) TableName() string {
	return "sessions"
}

// IsExpired checks if the session has expired (either idle or absolute)
func (s *Session) IsExpired() bool {
	now := time.Now()
	return now.After(s.ExpiresAt) || now.After(s.AbsoluteExpiry)
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Key         string     `gorm:"not null" json:"-"`                         // bcrypt hash (secure storage)
	KeyHash     string     `gorm:"uniqueIndex;not null" json:"-"`             // SHA256 hash (fast lookup)
	KeyID       string     `gorm:"uniqueIndex;not null" json:"key_id"`        // Public key identifier (key_xxxxx)
	Name        string     `gorm:"not null" json:"name"`                      // Descriptive name
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	User        User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ExpiresAt   *time.Time `gorm:"index" json:"expires_at,omitempty"`         // Optional expiry
	Enabled     bool       `gorm:"not null;default:true" json:"enabled"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	Permissions []string   `gorm:"serializer:json" json:"permissions"`        // Optional fine-grained permissions
}

// TableName overrides the table name
func (APIKey) TableName() string {
	return "api_keys"
}

// IsExpired checks if the API key has expired
func (k *APIKey) IsExpired() bool {
	if k.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*k.ExpiresAt)
}

// IsValid checks if the API key is valid (enabled and not expired)
func (k *APIKey) IsValid() bool {
	return k.Enabled && !k.IsExpired()
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	UserID    *uint  `gorm:"index" json:"user_id,omitempty"`               // Null for system actions
	User      *User  `gorm:"foreignKey:UserID" json:"user,omitempty"`      // Nullable
	Username  string `gorm:"index;not null" json:"username"`               // Denormalized for deleted users
	Action    string `gorm:"index;not null" json:"action"`                 // e.g., "config.commit", "user.create"
	Resource  string `gorm:"index" json:"resource,omitempty"`              // e.g., "network", "user:5"
	Status    string `gorm:"index;not null" json:"status"`                 // "success", "failure"
	Message   string `json:"message,omitempty"`                            // Human-readable message
	Details   string `gorm:"type:text" json:"details,omitempty"`           // JSON details
	IPAddress string `gorm:"index" json:"ip_address,omitempty"`            // Source IP
	Error     string `gorm:"type:text" json:"error,omitempty"`             // Error message if failed
	Duration  int64  `json:"duration_ms,omitempty"`                        // Duration in milliseconds
	TxID      string `gorm:"index" json:"transaction_id,omitempty"`        // Transaction ID if applicable
}

// TableName overrides the table name
func (AuditLog) TableName() string {
	return "audit_logs"
}

// Transaction represents a configuration transaction
type Transaction struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	TxID         string     `gorm:"uniqueIndex;not null" json:"transaction_id"` // Unique transaction ID
	UserID       *uint      `gorm:"index" json:"user_id,omitempty"`
	User         *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Username     string     `gorm:"index;not null" json:"username"` // Denormalized
	Message      string     `gorm:"not null" json:"message"`
	Status       string     `gorm:"index;not null" json:"status"` // "pending", "committed", "failed", "rolledback"
	SnapshotID   string     `gorm:"index" json:"snapshot_id,omitempty"`
	Configs      string     `gorm:"type:text" json:"configs"` // JSON array of changed configs
	ConfirmedAt  *time.Time `json:"confirmed_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	RolledBackAt *time.Time `json:"rolled_back_at,omitempty"`
	Error        string     `gorm:"type:text" json:"error,omitempty"`
}

// TableName overrides the table name
func (Transaction) TableName() string {
	return "transactions"
}
