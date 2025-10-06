package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/thesabbir/hellfire/pkg/appliers"
	"github.com/thesabbir/hellfire/pkg/auth"
	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/config"
	"github.com/thesabbir/hellfire/pkg/db"
	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/snapshot"
	"github.com/thesabbir/hellfire/pkg/transaction"
)

var (
	configDir       string
	stagingDir      string
	snapshotDir     string
	dbPath          string
	manager         *config.Manager
	snapshotMgr     *snapshot.Manager
	transactionMgr  *transaction.Manager
	applierRegistry *appliers.Registry
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "hf",
		Short: "Hellfire - Debian Router Configuration Tool",
		Long:  "A UCI-like configuration management tool for Debian routers",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize database (optional - some commands don't need it)
			if dbPath != "" {
				if err := db.Initialize(&db.Config{Path: dbPath}); err != nil {
					logger.Error("Failed to initialize database", "error", err)
					// Don't exit - some commands can work without DB
				} else {
					// Bootstrap: create default admin user if no users exist
					if err := bootstrapDefaultUser(); err != nil {
						logger.Warn("Failed to bootstrap default user", "error", err)
					}
				}
			}

			// Initialize managers
			manager = config.NewManager(configDir, stagingDir)
			snapshotMgr = snapshot.NewManager(snapshotDir, configDir)

			// Initialize applier registry
			applierRegistry = appliers.NewRegistry()
			applierRegistry.Register(appliers.NewNetworkApplier())
			applierRegistry.Register(appliers.NewFirewallApplier())
			applierRegistry.Register(appliers.NewDHCPApplier())

			// Initialize transaction manager
			transactionMgr = transaction.NewManager(manager, snapshotMgr, applierRegistry)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// Close database connection
			if db.DB != nil {
				_ = db.Close()
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&configDir, "config-dir", config.DefaultConfigDir, "Configuration directory")
	rootCmd.PersistentFlags().StringVar(&stagingDir, "staging-dir", config.StagingDir, "Staging directory")
	rootCmd.PersistentFlags().StringVar(&snapshotDir, "snapshot-dir", snapshot.DefaultSnapshotDir, "Snapshot directory")
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", db.DefaultDBPath, "Database file path")

	// Config management commands
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(setCmd)
	rootCmd.AddCommand(exportCmd)
	rootCmd.AddCommand(changesCmd)

	// Transaction commands
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(confirmCmd)
	rootCmd.AddCommand(rollbackCmd)

	// Snapshot commands
	rootCmd.AddCommand(snapshotCmd)

	// Apply commands (for systemd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(firewallCmd)
	rootCmd.AddCommand(dhcpCmd)

	// User management commands
	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(apikeyCmd)
	rootCmd.AddCommand(auditCmd)

	// API server
	rootCmd.AddCommand(serveCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var showCmd = &cobra.Command{
	Use:   "show <config>",
	Short: "Show configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		return manager.Export(configName, os.Stdout)
	},
}

var getCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Get configuration value (e.g., network.wan.ipaddr)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		value, err := manager.Get(path)
		if err != nil {
			return err
		}
		fmt.Println(value)
		return nil
	},
}

var setCmd = &cobra.Command{
	Use:   "set <path> <value>",
	Short: "Set configuration value (e.g., network.wan.ipaddr 192.168.1.1)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		value := args[1]

		if err := manager.Set(path, value); err != nil {
			return err
		}

		// Publish event
		bus.Publish(bus.Event{
			Type:       bus.EventConfigChanged,
			ConfigName: path,
			Data:       value,
		})

		fmt.Printf("Staged: %s = %s\n", path, value)
		fmt.Println("Run 'hf commit' to apply changes")
		return nil
	},
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit staged configuration changes",
	Long:  "Commit staged changes with automatic snapshot creation and optional confirm-or-revert",
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		confirmTimeout, _ := cmd.Flags().GetInt("confirm-timeout")

		if message == "" {
			message = "Configuration change"
		}

		confirmTimeoutDur := time.Duration(confirmTimeout) * time.Second

		// Call Commit with both confirmTimeout and overallTimeout (set overall to 0 = no timeout)
		if err := transactionMgr.Commit(message, confirmTimeoutDur, 0); err != nil {
			return err
		}

		if confirmTimeout > 0 {
			fmt.Printf("Changes applied successfully.\n")
			fmt.Printf("You have %d seconds to confirm or changes will be rolled back.\n", confirmTimeout)
			fmt.Printf("Run 'hf confirm' to confirm changes.\n")
		} else {
			fmt.Println("Changes committed successfully")
		}

		return nil
	},
}

func init() {
	commitCmd.Flags().StringP("message", "m", "", "Commit message")
	commitCmd.Flags().IntP("confirm-timeout", "t", 0, "Confirmation timeout in seconds (0 = no confirmation required)")
}

var confirmCmd = &cobra.Command{
	Use:   "confirm",
	Short: "Confirm pending configuration changes",
	Long:  "Confirm changes that are waiting for confirmation (prevents auto-rollback)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := transactionMgr.Confirm(); err != nil {
			return err
		}

		fmt.Println("Changes confirmed successfully")
		return nil
	},
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback to previous configuration",
	Long:  "Rollback to the most recent snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := transactionMgr.Rollback(); err != nil {
			return err
		}

		fmt.Println("Rolled back successfully")
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export <config>",
	Short: "Export configuration to stdout",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		return manager.Export(configName, os.Stdout)
	},
}

var changesCmd = &cobra.Command{
	Use:   "changes",
	Short: "Show staged changes",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !manager.HasChanges() {
			fmt.Println("No staged changes")
			return nil
		}

		changes := manager.GetChanges()
		fmt.Println("Staged changes:")
		for _, name := range changes {
			fmt.Printf("  - %s\n", name)
		}
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start web API server",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		return startAPIServer(port, manager)
	},
}

func init() {
	serveCmd.Flags().Int("port", 8888, "API server port")
}

// Snapshot commands
var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage configuration snapshots",
	Long:  "Create, list, and restore configuration snapshots",
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		snapshots, err := snapshotMgr.List()
		if err != nil {
			return err
		}

		if len(snapshots) == 0 {
			fmt.Println("No snapshots available")
			return nil
		}

		fmt.Println("Available snapshots:")
		for i, snap := range snapshots {
			fmt.Printf("%d. %s - %s\n", i+1, snap.ID, snap.Metadata.Message)
			fmt.Printf("   Time: %s\n", snap.Metadata.Timestamp.Format("2006-01-02 15:04:05"))
			fmt.Printf("   Configs: %v\n", snap.Metadata.Configs)
			fmt.Println()
		}

		return nil
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		// Load and display snapshot info
		snap, err := snapshotMgr.Load(id)
		if err != nil {
			return err
		}

		fmt.Printf("Restoring snapshot: %s\n", snap.Metadata.Message)
		fmt.Printf("Created: %s\n", snap.Metadata.Timestamp.Format("2006-01-02 15:04:05"))

		if err := snapshotMgr.Restore(id); err != nil {
			return err
		}

		fmt.Println("Snapshot restored successfully")
		fmt.Println("Note: Run 'hf commit' to apply the restored configuration")
		return nil
	},
}

var snapshotPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		keep, _ := cmd.Flags().GetInt("keep")

		deleted, err := snapshotMgr.Prune(keep)
		if err != nil {
			return err
		}

		if len(deleted) == 0 {
			fmt.Printf("No snapshots to prune (keeping last %d)\n", keep)
			return nil
		}

		fmt.Printf("Deleted %d snapshots:\n", len(deleted))
		for _, id := range deleted {
			fmt.Printf("  - %s\n", id)
		}

		return nil
	},
}

func init() {
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotPruneCmd)

	snapshotPruneCmd.Flags().Int("keep", 30, "Number of snapshots to keep")
}

// Network commands (for systemd)
var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Manage network configuration",
}

var networkApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply network configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		applier := appliers.NewNetworkApplier()

		cfg, err := manager.Load("network")
		if err != nil {
			return fmt.Errorf("failed to load network config: %w", err)
		}

		ctx := context.Background()
		if err := applier.Apply(ctx, cfg); err != nil {
			return fmt.Errorf("failed to apply network config: %w", err)
		}

		fmt.Println("Network configuration applied successfully")
		return nil
	},
}

var networkDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Bring down all managed interfaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Bringing down managed interfaces...")
		// Implementation would bring down interfaces
		return nil
	},
}

func init() {
	networkCmd.AddCommand(networkApplyCmd)
	networkCmd.AddCommand(networkDownCmd)
}

// Firewall commands (for systemd)
var firewallCmd = &cobra.Command{
	Use:   "firewall",
	Short: "Manage firewall configuration",
}

var firewallApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply firewall rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		applier := appliers.NewFirewallApplier()

		cfg, err := manager.Load("firewall")
		if err != nil {
			return fmt.Errorf("failed to load firewall config: %w", err)
		}

		ctx := context.Background()
		if err := applier.Apply(ctx, cfg); err != nil {
			return fmt.Errorf("failed to apply firewall rules: %w", err)
		}

		fmt.Println("Firewall rules applied successfully")
		return nil
	},
}

var firewallReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload firewall rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Reload is the same as apply
		return firewallApplyCmd.RunE(cmd, args)
	},
}

var firewallFlushCmd = &cobra.Command{
	Use:   "flush",
	Short: "Flush all firewall rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Flushing firewall rules...")
		// Run nft flush ruleset
		return nil
	},
}

func init() {
	firewallCmd.AddCommand(firewallApplyCmd)
	firewallCmd.AddCommand(firewallReloadCmd)
	firewallCmd.AddCommand(firewallFlushCmd)
}

// DHCP commands (for systemd)
var dhcpCmd = &cobra.Command{
	Use:   "dhcp",
	Short: "Manage DHCP/DNS configuration",
}

var dhcpApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply DHCP/DNS configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		applier := appliers.NewDHCPApplier()

		cfg, err := manager.Load("dhcp")
		if err != nil {
			return fmt.Errorf("failed to load dhcp config: %w", err)
		}

		ctx := context.Background()
		if err := applier.Apply(ctx, cfg); err != nil {
			return fmt.Errorf("failed to apply dhcp config: %w", err)
		}

		fmt.Println("DHCP/DNS configuration applied successfully")
		return nil
	},
}

func init() {
	dhcpCmd.AddCommand(dhcpApplyCmd)
}

// bootstrapDefaultUser creates a default admin user if no users exist
func bootstrapDefaultUser() error {
	// Check if any users exist
	count, err := db.CountUsers()
	if err != nil {
		return fmt.Errorf("failed to count users: %w", err)
	}

	// If users exist, don't create default user
	if count > 0 {
		return nil
	}

	logger.Info("No users found, creating default admin user")

	// Generate cryptographically secure random password
	randomPassword, err := generateSecurePassword(16)
	if err != nil {
		return fmt.Errorf("failed to generate secure password: %w", err)
	}

	// Hash the password
	passwordHash, err := auth.HashPassword(randomPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	defaultUser := &db.User{
		Username:     "admin",
		PasswordHash: passwordHash,
		Email:        "admin@localhost",
		Role:         db.RoleAdmin,
		Enabled:      true,
	}

	if err := db.CreateUser(defaultUser); err != nil {
		return fmt.Errorf("failed to create default user: %w", err)
	}

	logger.Warn("Default admin user created with random password",
		"username", "admin",
		"warning", "SAVE THE PASSWORD SHOWN BELOW - IT CANNOT BE RETRIEVED!")

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  DEFAULT ADMIN USER CREATED")
	fmt.Println("  Username: admin")
	fmt.Printf("  Password: %s\n", randomPassword)
	fmt.Println()
	fmt.Println("  ⚠️  SAVE THIS PASSWORD - IT WILL NOT BE SHOWN AGAIN!")
	fmt.Println("  To change password later: hf user password admin")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Also write to a secure log file (owner-only readable)
	logPath := "/var/lib/hellfire/initial-admin-password.txt"
	logContent := fmt.Sprintf("Initial admin password: %s\nGenerated: %s\n",
		randomPassword, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(logPath, []byte(logContent), 0600); err != nil {
		logger.Warn("Failed to write password to log file",
			"path", logPath,
			"error", err)
	} else {
		fmt.Printf("  Password also saved to: %s\n", logPath)
		fmt.Println()
	}

	return nil
}

// generateSecurePassword generates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"

	password := make([]byte, length)
	for i := range password {
		// Generate random index
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		password[i] = charset[num.Int64()]
	}

	return string(password), nil
}
