package snapshot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/uci"
	"github.com/thesabbir/hellfire/pkg/util"
	"github.com/thesabbir/hellfire/pkg/version"
)

const (
	DefaultSnapshotDir = "/var/lib/hellfire/snapshots"
	MetadataFile       = "metadata.json"
)

// Metadata contains information about a snapshot
type Metadata struct {
	Timestamp time.Time         `json:"timestamp"`
	Message   string            `json:"message"`
	Configs   []string          `json:"configs"`   // List of config files included
	ID        string            `json:"id"`        // Snapshot ID (timestamp-based)
	Version   string            `json:"version"`   // Hellfire version that created this snapshot
	Checksums map[string]string `json:"checksums"` // Config file name -> SHA256 checksum
}

// Snapshot represents a configuration snapshot
type Snapshot struct {
	ID       string
	Metadata Metadata
	Path     string
}

// Manager manages configuration snapshots
type Manager struct {
	snapshotDir string
	configDir   string
}

// NewManager creates a new snapshot manager
func NewManager(snapshotDir, configDir string) *Manager {
	if snapshotDir == "" {
		snapshotDir = DefaultSnapshotDir
	}
	return &Manager{
		snapshotDir: snapshotDir,
		configDir:   configDir,
	}
}

// Create creates a new snapshot of the current configuration
func (m *Manager) Create(message string, configs []string) (*Snapshot, error) {
	// Ensure snapshot directory exists before checking disk space
	if err := os.MkdirAll(m.snapshotDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Check disk space (require at least 1GB available)
	if err := util.CheckDiskSpace(m.snapshotDir, 1); err != nil {
		return nil, fmt.Errorf("insufficient disk space: %w", err)
	}

	// Generate unique snapshot ID (includes milliseconds + random suffix)
	id := util.GenerateUniqueID()
	snapshotPath := filepath.Join(m.snapshotDir, id)

	// Create specific snapshot directory with restricted permissions (owner only)
	if err := os.MkdirAll(snapshotPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Cleanup on error - remove partial snapshot
	success := false
	defer func() {
		if !success {
			os.RemoveAll(snapshotPath)
		}
	}()

	// Copy config files atomically
	copiedConfigs := []string{}
	for _, configName := range configs {
		srcPath := filepath.Join(m.configDir, configName)
		dstPath := filepath.Join(snapshotPath, configName)

		// Check if source file exists
		if _, err := os.Stat(srcPath); err != nil {
			if os.IsNotExist(err) {
				// Skip non-existent files
				continue
			}
			return nil, fmt.Errorf("failed to stat config %s: %w", configName, err)
		}

		// Copy file atomically with permission preservation
		if err := util.CopyFileAtomic(srcPath, dstPath); err != nil {
			return nil, fmt.Errorf("failed to copy config %s: %w", configName, err)
		}

		copiedConfigs = append(copiedConfigs, configName)
	}

	// Calculate checksums for all copied files
	checksums := make(map[string]string)
	for _, configName := range copiedConfigs {
		filePath := filepath.Join(snapshotPath, configName)
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config for checksum: %w", err)
		}
		hash := sha256.Sum256(data)
		checksums[configName] = fmt.Sprintf("%x", hash)
	}

	// Create metadata
	metadata := Metadata{
		Timestamp: time.Now(),
		Message:   message,
		Configs:   copiedConfigs,
		ID:        id,
		Version:   version.GetVersion(),
		Checksums: checksums,
	}

	// Write metadata atomically
	metadataPath := filepath.Join(snapshotPath, MetadataFile)
	tmpFile, err := os.CreateTemp(snapshotPath, ".metadata-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp metadata file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Cleanup temp file on error
	metaSuccess := false
	defer func() {
		if !metaSuccess {
			os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to sync metadata: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close metadata file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, metadataPath); err != nil {
		return nil, fmt.Errorf("failed to rename metadata file: %w", err)
	}

	metaSuccess = true
	success = true

	// Auto-prune old snapshots if we have too many
	snapshots, err := m.List()
	if err != nil {
		logger.Warn("Failed to list snapshots for auto-prune", "error", err)
	} else if len(snapshots) > 100 {
		deleted, err := m.Prune(100) // Keep last 100 snapshots
		if err != nil {
			logger.Warn("Failed to prune old snapshots", "error", err)
		} else {
			logger.Info("Auto-pruned old snapshots", "count", len(deleted))
		}
	}

	logger.Info("Snapshot created",
		"id", id,
		"configs", len(copiedConfigs),
		"version", metadata.Version)

	return &Snapshot{
		ID:       id,
		Metadata: metadata,
		Path:     snapshotPath,
	}, nil
}

// List returns all snapshots, sorted by timestamp (newest first)
func (m *Manager) List() ([]*Snapshot, error) {
	// Ensure snapshot directory exists with restricted permissions
	if err := os.MkdirAll(m.snapshotDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	entries, err := os.ReadDir(m.snapshotDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot directory: %w", err)
	}

	snapshots := []*Snapshot{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip if it's not a valid snapshot directory
		metadataPath := filepath.Join(m.snapshotDir, entry.Name(), MetadataFile)
		if _, err := os.Stat(metadataPath); err != nil {
			continue
		}

		// Load metadata
		snapshot, err := m.Load(entry.Name())
		if err != nil {
			// Skip invalid snapshots
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	// Sort by timestamp (newest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Metadata.Timestamp.After(snapshots[j].Metadata.Timestamp)
	})

	return snapshots, nil
}

// Load loads a snapshot by ID
func (m *Manager) Load(id string) (*Snapshot, error) {
	snapshotPath := filepath.Join(m.snapshotDir, id)

	// Check if snapshot exists
	if _, err := os.Stat(snapshotPath); err != nil {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}

	// Read metadata
	metadataPath := filepath.Join(snapshotPath, MetadataFile)
	f, err := os.Open(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata: %w", err)
	}
	defer f.Close()

	var metadata Metadata
	if err := json.NewDecoder(f).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return &Snapshot{
		ID:       id,
		Metadata: metadata,
		Path:     snapshotPath,
	}, nil
}

// Restore restores a snapshot to the config directory
func (m *Manager) Restore(id string) error {
	snapshot, err := m.Load(id)
	if err != nil {
		return err
	}

	// Validate snapshot integrity first
	if err := m.ValidateSnapshot(snapshot); err != nil {
		return fmt.Errorf("snapshot validation failed: %w", err)
	}

	// Copy each config file back atomically
	for _, configName := range snapshot.Metadata.Configs {
		srcPath := filepath.Join(snapshot.Path, configName)
		dstPath := filepath.Join(m.configDir, configName)

		// Use atomic copy to prevent partial writes
		if err := util.CopyFileAtomic(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to restore config %s: %w", configName, err)
		}
	}

	return nil
}

// ValidateSnapshot validates that a snapshot contains valid UCI config files
func (m *Manager) ValidateSnapshot(snapshot *Snapshot) error {
	for _, configName := range snapshot.Metadata.Configs {
		srcPath := filepath.Join(snapshot.Path, configName)

		// Check that file exists
		if _, err := os.Stat(srcPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("snapshot corrupted: %s missing", configName)
			}
			return fmt.Errorf("failed to stat %s: %w", configName, err)
		}

		// Verify checksum if available
		if len(snapshot.Metadata.Checksums) > 0 {
			expectedChecksum, ok := snapshot.Metadata.Checksums[configName]
			if ok {
				data, err := os.ReadFile(srcPath)
				if err != nil {
					return fmt.Errorf("failed to read %s for checksum: %w", configName, err)
				}
				hash := sha256.Sum256(data)
				actualChecksum := fmt.Sprintf("%x", hash)
				if actualChecksum != expectedChecksum {
					return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
						configName, expectedChecksum, actualChecksum)
				}
			}
		}

		// Validate that it's a valid UCI config
		f, err := os.Open(srcPath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", configName, err)
		}

		_, err = uci.Parse(f)
		f.Close()

		if err != nil {
			return fmt.Errorf("snapshot corrupted: invalid UCI in %s: %w", configName, err)
		}
	}

	return nil
}

// Delete deletes a snapshot
func (m *Manager) Delete(id string) error {
	snapshotPath := filepath.Join(m.snapshotDir, id)

	if err := os.RemoveAll(snapshotPath); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	return nil
}

// Prune removes old snapshots, keeping only the specified number
func (m *Manager) Prune(keep int) ([]string, error) {
	snapshots, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(snapshots) <= keep {
		return []string{}, nil
	}

	// Delete old snapshots
	deleted := []string{}
	for i := keep; i < len(snapshots); i++ {
		if err := m.Delete(snapshots[i].ID); err != nil {
			return deleted, fmt.Errorf("failed to delete snapshot %s: %w", snapshots[i].ID, err)
		}
		deleted = append(deleted, snapshots[i].ID)
	}

	return deleted, nil
}

// GetLatest returns the most recent snapshot
func (m *Manager) GetLatest() (*Snapshot, error) {
	snapshots, err := m.List()
	if err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots available")
	}

	return snapshots[0], nil
}
