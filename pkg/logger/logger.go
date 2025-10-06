// Package logger provides structured logging for Hellfire
package logger

import (
	"log/slog"
	"os"
)

var log *slog.Logger

func init() {
	// Default to JSON handler for production
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// SetLogger allows setting a custom logger
func SetLogger(l *slog.Logger) {
	log = l
}

// SetLevel sets the log level
func SetLevel(level slog.Level) {
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

// SetTextOutput sets the logger to use text output (for development)
func SetTextOutput() {
	log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	log.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	log.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	log.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	log.Error(msg, args...)
}

// With returns a new logger with the given attributes
func With(args ...any) *slog.Logger {
	return log.With(args...)
}
