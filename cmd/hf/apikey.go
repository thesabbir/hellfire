package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/thesabbir/hellfire/pkg/audit"
	"github.com/thesabbir/hellfire/pkg/auth"
	"github.com/thesabbir/hellfire/pkg/db"
)

var apikeyCmd = &cobra.Command{
	Use:   "apikey",
	Short: "Manage API keys",
	Long:  "Create, delete, and list API keys",
}

var apikeyListCmd = &cobra.Command{
	Use:   "list [username]",
	Short: "List API keys (all or for specific user)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAPIKeyList,
}

var apikeyCreateCmd = &cobra.Command{
	Use:   "create <username> <name>",
	Short: "Create a new API key for a user",
	Args:  cobra.ExactArgs(2),
	RunE:  runAPIKeyCreate,
}

var apikeyDeleteCmd = &cobra.Command{
	Use:   "delete <key-id>",
	Short: "Delete an API key by Key ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runAPIKeyDelete,
}

var apikeyShowCmd = &cobra.Command{
	Use:   "show <key-id>",
	Short: "Show API key details by Key ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runAPIKeyShow,
}

func init() {
	// API key create flags
	apikeyCreateCmd.Flags().Int("expires-days", 0, "Expiration in days (0 = no expiration)")

	// Add subcommands
	apikeyCmd.AddCommand(
		apikeyListCmd,
		apikeyCreateCmd,
		apikeyDeleteCmd,
		apikeyShowCmd,
	)
}

func runAPIKeyList(cmd *cobra.Command, args []string) error {
	var keys []db.APIKey
	var err error

	if len(args) > 0 {
		// List keys for specific user
		username := args[0]
		user, userErr := db.GetUserByUsername(username)
		if userErr != nil {
			return fmt.Errorf("user not found: %w", userErr)
		}

		keys, err = db.ListAPIKeys(user.ID)
		if err != nil {
			return fmt.Errorf("failed to list API keys: %w", err)
		}
	} else {
		// List all API keys (admin only)
		// For now, list all - in production should check permissions
		users, err := db.ListUsers()
		if err != nil {
			return fmt.Errorf("failed to list users: %w", err)
		}

		for _, user := range users {
			userKeys, err := db.ListAPIKeys(user.ID)
			if err != nil {
				continue
			}
			keys = append(keys, userKeys...)
		}
	}

	if len(keys) == 0 {
		fmt.Println("No API keys found")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tUSER\tENABLED\tEXPIRES\tLAST USED\tKEY (first 16 chars)")
	fmt.Fprintln(w, "--\t----\t----\t-------\t-------\t---------\t-------------------")

	for _, key := range keys {
		// Load user for each key
		user, err := db.GetUserByID(key.UserID)
		if err != nil {
			continue
		}

		expires := "never"
		if key.ExpiresAt != nil {
			expires = key.ExpiresAt.Format("2006-01-02")
		}

		lastUsed := "never"
		if key.LastUsedAt != nil {
			lastUsed = key.LastUsedAt.Format("2006-01-02 15:04")
		}

		enabled := "yes"
		if !key.Enabled {
			enabled = "no"
		}

		// Show only first 16 chars of key for security
		keyPreview := key.Key
		if len(keyPreview) > 16 {
			keyPreview = keyPreview[:16] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			key.ID,
			key.Name,
			user.Username,
			enabled,
			expires,
			lastUsed,
			keyPreview,
		)
	}

	w.Flush()
	return nil
}

func runAPIKeyCreate(cmd *cobra.Command, args []string) error {
	username := args[0]
	name := args[1]

	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Get expiration
	expiresDays, _ := cmd.Flags().GetInt("expires-days")
	var expiresAt *time.Time
	if expiresDays > 0 {
		expiry := time.Now().AddDate(0, 0, expiresDays)
		expiresAt = &expiry
	}

	// Generate secure API key (32 bytes = 64 hex chars)
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}
	apiKeyValue := "hf_" + hex.EncodeToString(keyBytes)

	// Generate KeyID (public identifier)
	keyID := generateKeyID()

	// Create SHA256 hash for fast lookup
	keyHashBytes := sha256.Sum256([]byte(apiKeyValue))
	keyHash := hex.EncodeToString(keyHashBytes[:])

	// Create bcrypt hash for secure storage
	keyBcryptHash, err := auth.HashPassword(apiKeyValue)
	if err != nil {
		return fmt.Errorf("failed to hash API key: %w", err)
	}

	// Create API key
	apiKey := &db.APIKey{
		Key:       keyBcryptHash,
		KeyHash:   keyHash,
		KeyID:     keyID,
		Name:      name,
		UserID:    user.ID,
		ExpiresAt: expiresAt,
		Enabled:   true,
	}

	if err := db.CreateAPIKey(apiKey); err != nil {
		audit.LogFailure(audit.ActionAPIKeyCreate, nil, "system", fmt.Sprintf("apikey:%s", name), "Failed to create API key", err)
		return fmt.Errorf("failed to create API key: %w", err)
	}

	// Audit log
	audit.LogSuccess(audit.ActionAPIKeyCreate, nil, "system", fmt.Sprintf("apikey:%d", apiKey.ID),
		fmt.Sprintf("API key '%s' created for user '%s'", name, username))

	// IMPORTANT: Display the key ONCE - it cannot be retrieved later
	fmt.Printf("API Key created successfully!\n\n")
	fmt.Printf("IMPORTANT: Save this API key - it cannot be retrieved again!\n\n")
	fmt.Printf("API Key: %s\n", apiKeyValue)
	fmt.Printf("Key ID:  %s\n\n", keyID)
	fmt.Printf("Details:\n")
	fmt.Printf("  ID:      %d\n", apiKey.ID)
	fmt.Printf("  Name:    %s\n", name)
	fmt.Printf("  User:    %s\n", username)

	if expiresAt != nil {
		fmt.Printf("  Expires: %s\n", expiresAt.Format("2006-01-02"))
	} else {
		fmt.Printf("  Expires: never\n")
	}

	fmt.Printf("\nUse Key ID '%s' for management commands (show, delete)\n", keyID)

	return nil
}

// generateKeyID generates a unique key identifier
func generateKeyID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "key_" + hex.EncodeToString(bytes)
}

func runAPIKeyDelete(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	// Get API key by KeyID (fast O(1) lookup)
	apiKey, err := db.GetAPIKeyByID(keyID)
	if err != nil {
		return fmt.Errorf("API key not found: %w", err)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete API key '%s'? (yes/no): ", apiKey.Name)
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Deletion cancelled")
		return nil
	}

	// Delete API key
	if err := db.DeleteAPIKey(apiKey.ID); err != nil {
		audit.LogFailure(audit.ActionAPIKeyDelete, nil, "system", fmt.Sprintf("apikey:%d", apiKey.ID), "Failed to delete API key", err)
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	// Audit log
	audit.LogSuccess(audit.ActionAPIKeyDelete, nil, "system", fmt.Sprintf("apikey:%d", apiKey.ID),
		fmt.Sprintf("API key '%s' deleted", apiKey.Name))

	fmt.Printf("API key '%s' deleted successfully\n", apiKey.Name)
	return nil
}

func runAPIKeyShow(cmd *cobra.Command, args []string) error {
	keyID := args[0]

	// Get API key by KeyID (fast O(1) lookup)
	apiKey, err := db.GetAPIKeyByID(keyID)
	if err != nil {
		return fmt.Errorf("API key not found: %w", err)
	}

	fmt.Printf("API Key Details:\n")
	fmt.Printf("  ID:       %d\n", apiKey.ID)
	fmt.Printf("  Key ID:   %s\n", apiKey.KeyID)
	fmt.Printf("  Name:     %s\n", apiKey.Name)
	fmt.Printf("  User:     %s (%s)\n", apiKey.User.Username, apiKey.User.Role)
	fmt.Printf("  Enabled:  %t\n", apiKey.Enabled)
	fmt.Printf("  Created:  %s\n", apiKey.CreatedAt.Format(time.RFC3339))

	if apiKey.ExpiresAt != nil {
		fmt.Printf("  Expires:  %s", apiKey.ExpiresAt.Format(time.RFC3339))
		if apiKey.IsExpired() {
			fmt.Printf(" (EXPIRED)")
		}
		fmt.Println()
	} else {
		fmt.Printf("  Expires:  never\n")
	}

	if apiKey.LastUsedAt != nil {
		fmt.Printf("  Last Used: %s\n", apiKey.LastUsedAt.Format(time.RFC3339))
	} else {
		fmt.Printf("  Last Used: never\n")
	}

	if len(apiKey.Permissions) > 0 {
		fmt.Printf("\nPermissions:\n")
		for _, perm := range apiKey.Permissions {
			fmt.Printf("  - %s\n", perm)
		}
	}

	return nil
}
