package db

import (
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	hflogger "github.com/thesabbir/hellfire/pkg/logger"
)

const (
	DefaultDBPath = "/var/lib/hellfire/hellfire.db"
)

var (
	// Global DB instance
	DB *gorm.DB
)

// Config holds database configuration
type Config struct {
	Path string
}

// Initialize initializes the database connection
func Initialize(cfg *Config) error {
	if cfg == nil {
		cfg = &Config{Path: DefaultDBPath}
	}

	if cfg.Path == "" {
		cfg.Path = DefaultDBPath
	}

	// Ensure directory exists with restricted permissions (owner only)
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Configure GORM logger to use our structured logger
	gormLogger := logger.New(
		&gormLogAdapter{},
		logger.Config{
			SlowThreshold:             200, // Log slow queries (>200ms)
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open database connection
	db, err := gorm.Open(sqlite.Open(cfg.Path), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get underlying SQL DB for connection pooling
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying db: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(1) // SQLite only supports 1 writer
	sqlDB.SetMaxIdleConns(1)

	// Auto-migrate schemas
	if err := db.AutoMigrate(
		&User{},
		&Session{},
		&APIKey{},
		&AuditLog{},
		&Transaction{},
	); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Set database file permissions to owner-only (security)
	if err := os.Chmod(cfg.Path, 0600); err != nil {
		hflogger.Warn("Failed to set database file permissions", "error", err)
	}

	DB = db
	hflogger.Info("Database initialized", "path", cfg.Path)

	return nil
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// gormLogAdapter adapts GORM logger to our structured logger
type gormLogAdapter struct{}

func (l *gormLogAdapter) Printf(format string, args ...interface{}) {
	hflogger.Debug(fmt.Sprintf(format, args...))
}
