package util

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// CopyFileAtomic copies a file atomically, preserving permissions
func CopyFileAtomic(src, dst string) error {
	// Get source file info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Create temp file in destination directory
	tmpFile, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup temp file on error
	success := false
	defer func() {
		if !success {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Copy contents
	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	// Close temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions to match source
	if err := os.Chmod(tmpPath, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("failed to rename: %w", err)
	}

	success = true
	return nil
}

// GenerateUniqueID generates a unique ID for snapshots
// Format: YYYYMMDD-HHMMSS-mmm-RRRR
// Where mmm = milliseconds, RRRR = random hex suffix
func GenerateUniqueID() string {
	timestamp := time.Now().Format("20060102-150405")

	// Add milliseconds
	ms := time.Now().UnixMilli() % 1000

	// Add random suffix for extra safety
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		// Fallback to timestamp-only if crypto/rand fails
		// This is extremely rare but we handle it gracefully
		return fmt.Sprintf("%s-%03d-fallback", timestamp, ms)
	}
	randSuffix := fmt.Sprintf("%x", randBytes[:2])

	return fmt.Sprintf("%s-%03d-%s", timestamp, ms, randSuffix)
}

// CheckDiskSpace checks if sufficient disk space is available
// Returns error if less than requiredGB is available
func CheckDiskSpace(path string, requiredGB uint64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	// Calculate available space in GB
	availableGB := stat.Bavail * uint64(stat.Bsize) / (1024 * 1024 * 1024)

	if availableGB < requiredGB {
		return fmt.Errorf("insufficient disk space: %d GB available, %d GB required", availableGB, requiredGB)
	}

	return nil
}

// GetDiskUsageGB returns the disk usage of a directory in GB
func GetDiskUsageGB(path string) (uint64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return uint64(size) / (1024 * 1024 * 1024), nil
}
