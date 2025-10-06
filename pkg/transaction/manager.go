package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thesabbir/hellfire/pkg/appliers"
	"github.com/thesabbir/hellfire/pkg/audit"
	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/config"
	"github.com/thesabbir/hellfire/pkg/db"
	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/snapshot"
	"github.com/thesabbir/hellfire/pkg/util"
)

// State represents the current transaction state
type State string

const (
	StateIdle       State = "idle"
	StateInProgress State = "in_progress"
	StatePending    State = "pending" // Waiting for confirmation
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
)

// Manager manages configuration transactions
type Manager struct {
	configManager   *config.Manager
	snapshotManager *snapshot.Manager
	applierRegistry *appliers.Registry

	mu              sync.RWMutex
	state           State
	currentSnapshot *snapshot.Snapshot
	currentTxRecord *db.Transaction // Database transaction record
	pendingConfirm  *pendingConfirmation
	confirmCancelCh chan struct{}
	timerWg         sync.WaitGroup // Track confirmation timer goroutines
	applyOrder      []string       // Configurable order for applying configs
	userID          *uint          // User ID for audit logging
	username        string         // Username for audit logging
}

// pendingConfirmation holds information about a pending confirmation
type pendingConfirmation struct {
	Snapshot  *snapshot.Snapshot
	Timeout   time.Duration
	StartTime time.Time
}

// NewManager creates a new transaction manager
func NewManager(configManager *config.Manager, snapshotManager *snapshot.Manager, registry *appliers.Registry) *Manager {
	return &Manager{
		configManager:   configManager,
		snapshotManager: snapshotManager,
		applierRegistry: registry,
		state:           StateIdle,
		applyOrder:      []string{"network", "firewall", "dhcp"}, // Default order
	}
}

// SetApplyOrder sets the order in which configurations are applied
func (m *Manager) SetApplyOrder(order []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.applyOrder = order
}

// SetUser sets the user context for audit logging
func (m *Manager) SetUser(userID uint, username string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userID = &userID
	m.username = username
}

// Commit commits staged configuration changes
// overallTimeout is the maximum time for the entire transaction (0 = no timeout)
// confirmTimeout is how long to wait for user confirmation (0 = no confirmation needed)
func (m *Manager) Commit(message string, confirmTimeout, overallTimeout time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateIdle {
		return fmt.Errorf("transaction already in progress (state: %s)", m.state)
	}

	// Check if there are changes to commit
	if !m.configManager.HasChanges() {
		return fmt.Errorf("no changes to commit")
	}

	// Create context with timeout if specified
	ctx := context.Background()
	if overallTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, overallTimeout)
		defer cancel()
	}

	m.state = StateInProgress
	logger.Info("Starting transaction", "message", message)

	// Generate transaction ID
	txID := util.GenerateUniqueID()

	// Create database transaction record
	configsJSON, _ := json.Marshal([]string{}) // Will be updated later with actual configs
	m.currentTxRecord = &db.Transaction{
		TxID:     txID,
		UserID:   m.userID,
		Username: m.username,
		Message:  message,
		Status:   string(StatePending),
		Configs:  string(configsJSON),
	}

	// Save transaction to database (if DB is available)
	if db.DB != nil {
		if err := db.CreateTransaction(m.currentTxRecord); err != nil {
			logger.Warn("Failed to create transaction record", "error", err)
		}

		// Audit log: transaction started
		audit.Log(audit.ActionTxStart, audit.StatusSuccess, m.userID, m.username, txID, message, nil)
	}

	// Publish event
	bus.Publish(bus.Event{
		Type: bus.EventTransactionStarted,
		Data: message,
	})

	// Get list of changed configs
	changedConfigs := m.configManager.GetChanges()

	// Update transaction record with changed configs
	if db.DB != nil {
		configsJSON, _ := json.Marshal(changedConfigs)
		m.currentTxRecord.Configs = string(configsJSON)
		_ = db.UpdateTransaction(m.currentTxRecord)
	}

	// Create snapshot before applying changes
	snapshot, err := m.snapshotManager.Create(message, changedConfigs)
	if err != nil {
		m.state = StateFailed
		if db.DB != nil {
			m.currentTxRecord.Status = string(StateFailed)
			m.currentTxRecord.Error = err.Error()
			_ = db.UpdateTransaction(m.currentTxRecord)
			audit.LogFailure(audit.ActionSnapshotCreate, m.userID, m.username, txID, "Failed to create snapshot", err)
		}
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	m.currentSnapshot = snapshot

	// Update transaction with snapshot ID
	if db.DB != nil {
		m.currentTxRecord.SnapshotID = snapshot.ID
		_ = db.UpdateTransaction(m.currentTxRecord)
		audit.LogSuccess(audit.ActionSnapshotCreate, m.userID, m.username, snapshot.ID, "Snapshot created")
	}

	// Publish snapshot created event
	bus.Publish(bus.Event{
		Type: bus.EventSnapshotCreated,
		Data: snapshot,
	})

	// Commit config changes (write to disk)
	if err := m.configManager.Commit(); err != nil {
		m.state = StateFailed
		return fmt.Errorf("failed to commit config: %w", err)
	}

	// Apply configurations in configured order
	for _, applierName := range m.applyOrder {
		// Check context cancellation
		select {
		case <-ctx.Done():
			m.rollbackInternal(ctx)
			m.state = StateFailed
			return ctx.Err()
		default:
		}

		// Skip if this config wasn't changed
		configChanged := false
		for _, changed := range changedConfigs {
			if changed == applierName {
				configChanged = true
				break
			}
		}

		if !configChanged {
			continue
		}

		// Get applier
		applier, ok := m.applierRegistry.Get(applierName)
		if !ok {
			// Applier not registered, skip
			continue
		}

		// Load config
		cfg, err := m.configManager.Load(applierName)
		if err != nil {
			// Rollback on error
			m.rollbackInternal(ctx)
			m.state = StateFailed
			return fmt.Errorf("failed to load config %s: %w", applierName, err)
		}

		// Apply configuration
		logger.Info("Applying configuration", "applier", applierName)
		if err := applier.Apply(ctx, cfg); err != nil {
			// Rollback on error
			logger.Error("Failed to apply configuration", "applier", applierName, "error", err)
			m.rollbackInternal(ctx)
			m.state = StateFailed
			return fmt.Errorf("failed to apply %s config: %w", applierName, err)
		}

		// Validate
		logger.Info("Validating configuration", "applier", applierName)
		if err := applier.Validate(ctx); err != nil {
			// Rollback on validation failure
			logger.Error("Validation failed", "applier", applierName, "error", err)
			m.rollbackInternal(ctx)
			m.state = StateFailed
			return fmt.Errorf("validation failed for %s: %w", applierName, err)
		}
	}

	// If confirm timeout is set, start confirmation timer
	if confirmTimeout > 0 {
		m.state = StatePending
		m.pendingConfirm = &pendingConfirmation{
			Snapshot:  snapshot,
			Timeout:   confirmTimeout,
			StartTime: time.Now(),
		}

		// Start confirmation timer in background with proper tracking
		m.confirmCancelCh = make(chan struct{})
		m.timerWg.Add(1)
		go func() {
			defer m.timerWg.Done()
			m.confirmationTimer(confirmTimeout)
		}()

		return nil
	}

	// No confirmation needed, mark as completed
	m.state = StateCompleted

	// Update database transaction record
	if db.DB != nil {
		now := time.Now()
		m.currentTxRecord.Status = string(StateCompleted)
		m.currentTxRecord.CompletedAt = &now
		_ = db.UpdateTransaction(m.currentTxRecord)

		// Audit log: transaction completed
		audit.LogSuccess(audit.ActionTxCommit, m.userID, m.username, txID, "Transaction completed successfully")
	}

	bus.Publish(bus.Event{
		Type: bus.EventTransactionCompleted,
		Data: changedConfigs,
	})

	logger.Info("Transaction completed successfully", "tx_id", txID)

	return nil
}

// Confirm confirms pending changes
func (m *Manager) Confirm() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StatePending {
		return fmt.Errorf("no pending confirmation (state: %s)", m.state)
	}

	// Cancel the confirmation timer safely (prevents race condition)
	if m.confirmCancelCh != nil {
		util.SafeClose(m.confirmCancelCh)
		m.confirmCancelCh = nil
	}

	// Mark as completed
	m.state = StateCompleted
	m.pendingConfirm = nil

	// Update database transaction record
	if db.DB != nil && m.currentTxRecord != nil {
		now := time.Now()
		m.currentTxRecord.Status = string(StateCompleted)
		m.currentTxRecord.ConfirmedAt = &now
		m.currentTxRecord.CompletedAt = &now
		_ = db.UpdateTransaction(m.currentTxRecord)

		// Audit log: transaction confirmed
		audit.LogSuccess(audit.ActionTxConfirm, m.userID, m.username, m.currentTxRecord.TxID, "Transaction confirmed")
	}

	bus.Publish(bus.Event{
		Type: bus.EventTransactionCompleted,
		Data: "confirmed",
	})

	logger.Info("Transaction confirmed")

	return nil
}

// Rollback rolls back to the previous snapshot
func (m *Manager) Rollback() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	return m.rollbackInternal(ctx)
}

// rollbackInternal performs the actual rollback (must be called with lock held)
func (m *Manager) rollbackInternal(ctx context.Context) error {
	if m.currentSnapshot == nil {
		// Try to get the latest snapshot
		latest, err := m.snapshotManager.GetLatest()
		if err != nil {
			return fmt.Errorf("no snapshot to rollback to: %w", err)
		}
		m.currentSnapshot = latest
	}

	bus.Publish(bus.Event{
		Type: bus.EventRollbackStarted,
		Data: m.currentSnapshot.ID,
	})

	// Restore snapshot
	if err := m.snapshotManager.Restore(m.currentSnapshot.ID); err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	// Re-apply all configurations, collecting errors
	var rollbackErrors []string
	for _, configName := range m.currentSnapshot.Metadata.Configs {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		applier, ok := m.applierRegistry.Get(configName)
		if !ok {
			continue
		}

		// Load config
		cfg, err := m.configManager.Load(configName)
		if err != nil {
			rollbackErrors = append(rollbackErrors,
				fmt.Sprintf("%s: failed to load: %v", configName, err))
			continue
		}

		// Apply
		if err := applier.Apply(ctx, cfg); err != nil {
			rollbackErrors = append(rollbackErrors,
				fmt.Sprintf("%s: failed to apply: %v", configName, err))
			continue
		}
	}

	// Check if there were any errors
	if len(rollbackErrors) > 0 {
		m.state = StateFailed
		return fmt.Errorf("rollback partially failed: %s", strings.Join(rollbackErrors, "; "))
	}

	m.state = StateIdle
	m.currentSnapshot = nil
	m.pendingConfirm = nil

	// Update database transaction record
	if db.DB != nil && m.currentTxRecord != nil {
		now := time.Now()
		m.currentTxRecord.Status = "rolledback"
		m.currentTxRecord.RolledBackAt = &now
		_ = db.UpdateTransaction(m.currentTxRecord)

		// Audit log: rollback completed
		audit.LogSuccess(audit.ActionTxRollback, m.userID, m.username, m.currentTxRecord.TxID, "Rollback completed successfully")
	}

	bus.Publish(bus.Event{
		Type: bus.EventConfigReverted,
		Data: "rollback completed",
	})

	logger.Info("Rollback completed successfully")

	return nil
}

// confirmationTimer waits for timeout and auto-rollback if not confirmed
func (m *Manager) confirmationTimer(timeout time.Duration) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		// Timeout reached, rollback
		m.mu.Lock()
		if m.state == StatePending {
			logger.Warn("Confirmation timeout reached, rolling back changes...")
			ctx := context.Background()
			_ = m.rollbackInternal(ctx)
		}
		m.mu.Unlock()

	case <-m.confirmCancelCh:
		// Confirmation received, do nothing
		return
	}
}

// GetState returns the current transaction state
func (m *Manager) GetState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// GetPendingConfirmation returns pending confirmation info if any
func (m *Manager) GetPendingConfirmation() *pendingConfirmation {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pendingConfirm
}

// RemainingConfirmTime returns the remaining time to confirm
func (m *Manager) RemainingConfirmTime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.pendingConfirm == nil {
		return 0
	}

	elapsed := time.Since(m.pendingConfirm.StartTime)
	remaining := m.pendingConfirm.Timeout - elapsed

	if remaining < 0 {
		return 0
	}

	return remaining
}

// Close waits for all background goroutines to finish
func (m *Manager) Close() {
	m.timerWg.Wait()
}
