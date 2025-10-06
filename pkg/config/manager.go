package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/thesabbir/hellfire/pkg/uci"
)

const (
	DefaultConfigDir = "/etc/config"
	StagingDir       = "/tmp/uci-staging"
)

// Manager manages UCI configuration files with staging support
type Manager struct {
	configDir  string
	stagingDir string
	mu         sync.RWMutex
	staged     map[string]*uci.Config // staged configs (not yet committed)
}

// NewManager creates a new config manager
func NewManager(configDir, stagingDir string) *Manager {
	if configDir == "" {
		configDir = DefaultConfigDir
	}
	if stagingDir == "" {
		stagingDir = StagingDir
	}

	return &Manager{
		configDir:  configDir,
		stagingDir: stagingDir,
		staged:     make(map[string]*uci.Config),
	}
}

// Load loads a configuration file
func (m *Manager) Load(name string) (*uci.Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if there's a staged version
	if staged, ok := m.staged[name]; ok {
		return staged, nil
	}

	// Load from disk
	path := filepath.Join(m.configDir, name)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return uci.NewConfig(), nil
		}
		return nil, fmt.Errorf("failed to open config %s: %w", name, err)
	}
	defer f.Close()

	config, err := uci.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", name, err)
	}

	return config, nil
}

// Stage stages a configuration for commit
func (m *Manager) Stage(name string, config *uci.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.staged[name] = config
	return nil
}

// Commit commits all staged configurations
func (m *Manager) Commit() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.staged) == 0 {
		return fmt.Errorf("no staged changes to commit")
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write all staged configs
	for name, config := range m.staged {
		path := filepath.Join(m.configDir, name)

		// Create temporary file
		tmpPath := path + ".tmp"
		f, err := os.Create(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to create temp file for %s: %w", name, err)
		}

		// Write config
		if err := uci.Write(f, config); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write config %s: %w", name, err)
		}
		f.Close()

		// Atomic rename
		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to commit config %s: %w", name, err)
		}
	}

	// Clear staged changes
	m.staged = make(map[string]*uci.Config)

	return nil
}

// Revert reverts all staged configurations
func (m *Manager) Revert() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.staged) == 0 {
		return fmt.Errorf("no staged changes to revert")
	}

	m.staged = make(map[string]*uci.Config)
	return nil
}

// HasChanges returns true if there are staged changes
func (m *Manager) HasChanges() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.staged) > 0
}

// GetChanges returns a list of config names with staged changes
func (m *Manager) GetChanges() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	changes := make([]string, 0, len(m.staged))
	for name := range m.staged {
		changes = append(changes, name)
	}
	return changes
}

// Get gets a value from a config using dot notation (e.g., "network.wan.ipaddr")
func (m *Manager) Get(path string) (string, error) {
	configName, sectionName, optionName, err := parsePath(path)
	if err != nil {
		return "", err
	}

	config, err := m.Load(configName)
	if err != nil {
		return "", err
	}

	// Find section by name (for named sections) or by type (for unnamed)
	var section *uci.Section
	for _, s := range config.Sections {
		if s.Name == sectionName || (s.Name == "" && s.Type == sectionName) {
			section = s
			break
		}
	}

	if section == nil {
		return "", fmt.Errorf("section not found: %s", sectionName)
	}

	if optionName == "" {
		return "", fmt.Errorf("option name required")
	}

	value, ok := section.GetOption(optionName)
	if !ok {
		return "", fmt.Errorf("option not found: %s", optionName)
	}

	return value, nil
}

// Set sets a value in a config using dot notation
func (m *Manager) Set(path, value string) error {
	configName, sectionName, optionName, err := parsePath(path)
	if err != nil {
		return err
	}

	config, err := m.Load(configName)
	if err != nil {
		return err
	}

	// Find or create section
	var section *uci.Section
	for _, s := range config.Sections {
		if s.Name == sectionName || (s.Name == "" && s.Type == sectionName) {
			section = s
			break
		}
	}

	if section == nil {
		// Create new section
		section = uci.NewSection(sectionName, sectionName)
		config.AddSection(section)
	}

	section.SetOption(optionName, value)

	// Stage the modified config
	return m.Stage(configName, config)
}

// Export exports a configuration to a writer
func (m *Manager) Export(name string, w io.Writer) error {
	config, err := m.Load(name)
	if err != nil {
		return err
	}

	return uci.Write(w, config)
}

// parsePath parses a dot-notation path like "network.wan.ipaddr"
// Returns: configName, sectionName, optionName, error
func parsePath(path string) (string, string, string, error) {
	parts := splitPath(path)

	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid path: must be config.section[.option]")
	}

	configName := parts[0]
	sectionName := parts[1]
	optionName := ""

	if len(parts) > 2 {
		optionName = parts[2]
	}

	return configName, sectionName, optionName, nil
}

// splitPath splits a dot-notation path
func splitPath(path string) []string {
	parts := []string{}
	current := ""

	for _, r := range path {
		if r == '.' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	return parts
}
