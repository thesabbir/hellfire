package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/thesabbir/hellfire/pkg/audit"
	"github.com/thesabbir/hellfire/pkg/auth"
	"github.com/thesabbir/hellfire/pkg/db"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage users",
	Long:  "Create, update, delete, and list users",
}

var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	RunE:  runUserList,
}

var userCreateCmd = &cobra.Command{
	Use:   "create <username>",
	Short: "Create a new user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserCreate,
}

var userUpdateCmd = &cobra.Command{
	Use:   "update <username>",
	Short: "Update a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserUpdate,
}

var userDeleteCmd = &cobra.Command{
	Use:   "delete <username>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserDelete,
}

var userPasswordCmd = &cobra.Command{
	Use:   "password <username>",
	Short: "Change user password",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserPassword,
}

var userShowCmd = &cobra.Command{
	Use:   "show <username>",
	Short: "Show user details",
	Args:  cobra.ExactArgs(1),
	RunE:  runUserShow,
}

func init() {
	// User create flags
	userCreateCmd.Flags().String("email", "", "User email address")
	userCreateCmd.Flags().String("role", "viewer", "User role (admin, operator, viewer)")
	userCreateCmd.Flags().String("password", "", "User password (prompted if not provided)")

	// User update flags
	userUpdateCmd.Flags().String("email", "", "User email address")
	userUpdateCmd.Flags().String("role", "", "User role (admin, operator, viewer)")
	userUpdateCmd.Flags().Bool("enable", false, "Enable user")
	userUpdateCmd.Flags().Bool("disable", false, "Disable user")

	// Add subcommands
	userCmd.AddCommand(
		userListCmd,
		userCreateCmd,
		userUpdateCmd,
		userDeleteCmd,
		userPasswordCmd,
		userShowCmd,
	)
}

func runUserList(cmd *cobra.Command, args []string) error {
	users, err := db.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	if len(users) == 0 {
		fmt.Println("No users found")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tUSERNAME\tROLE\tENABLED\tEMAIL\tLAST LOGIN")
	fmt.Fprintln(w, "--\t--------\t----\t-------\t-----\t----------")

	for _, user := range users {
		lastLogin := "never"
		if user.LastLoginAt != nil {
			lastLogin = user.LastLoginAt.Format("2006-01-02 15:04:05")
		}

		enabled := "yes"
		if !user.Enabled {
			enabled = "no"
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			user.ID,
			user.Username,
			user.Role,
			enabled,
			user.Email,
			lastLogin,
		)
	}

	w.Flush()
	return nil
}

func runUserCreate(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Get flags
	email, _ := cmd.Flags().GetString("email")
	roleStr, _ := cmd.Flags().GetString("role")
	password, _ := cmd.Flags().GetString("password")

	// Validate role
	role := db.Role(roleStr)
	if role != db.RoleAdmin && role != db.RoleOperator && role != db.RoleViewer {
		return fmt.Errorf("invalid role: %s (must be admin, operator, or viewer)", roleStr)
	}

	// Prompt for password if not provided
	if password == "" {
		fmt.Print("Enter password: ")
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		password = string(passwordBytes)

		fmt.Print("Confirm password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}

		if password != string(confirmBytes) {
			return fmt.Errorf("passwords do not match")
		}
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &db.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        email,
		Role:         role,
		Enabled:      true,
	}

	if err := db.CreateUser(user); err != nil {
		// Audit log
		audit.LogFailure(audit.ActionUserCreate, nil, "system", fmt.Sprintf("user:%s", username), "Failed to create user", err)
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Audit log
	audit.LogSuccess(audit.ActionUserCreate, nil, "system", fmt.Sprintf("user:%d", user.ID), fmt.Sprintf("User '%s' created", username))

	fmt.Printf("User '%s' created successfully (ID: %d)\n", username, user.ID)
	return nil
}

func runUserUpdate(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Get flags
	email, _ := cmd.Flags().GetString("email")
	roleStr, _ := cmd.Flags().GetString("role")
	enable, _ := cmd.Flags().GetBool("enable")
	disable, _ := cmd.Flags().GetBool("disable")

	changes := []string{}

	// Update email
	if email != "" {
		user.Email = email
		changes = append(changes, "email")
	}

	// Update role
	if roleStr != "" {
		role := db.Role(roleStr)
		if role != db.RoleAdmin && role != db.RoleOperator && role != db.RoleViewer {
			return fmt.Errorf("invalid role: %s", roleStr)
		}
		user.Role = role
		changes = append(changes, "role")
	}

	// Update enabled status
	if enable && disable {
		return fmt.Errorf("cannot use both --enable and --disable")
	}

	if enable {
		user.Enabled = true
		changes = append(changes, "enabled")
	}

	if disable {
		user.Enabled = false
		changes = append(changes, "disabled")
	}

	if len(changes) == 0 {
		return fmt.Errorf("no changes specified")
	}

	// Save changes
	if err := db.UpdateUser(user); err != nil {
		audit.LogFailure(audit.ActionUserUpdate, nil, "system", fmt.Sprintf("user:%d", user.ID), "Failed to update user", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Audit log
	audit.LogSuccess(audit.ActionUserUpdate, nil, "system", fmt.Sprintf("user:%d", user.ID),
		fmt.Sprintf("User '%s' updated (%s)", username, strings.Join(changes, ", ")))

	fmt.Printf("User '%s' updated successfully (%s)\n", username, strings.Join(changes, ", "))
	return nil
}

func runUserDelete(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Confirm deletion
	fmt.Printf("Are you sure you want to delete user '%s'? (yes/no): ", username)
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Deletion cancelled")
		return nil
	}

	// Delete user
	if err := db.DeleteUser(user.ID); err != nil {
		audit.LogFailure(audit.ActionUserDelete, nil, "system", fmt.Sprintf("user:%d", user.ID), "Failed to delete user", err)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Also delete all sessions for this user
	_ = db.DeleteUserSessions(user.ID)

	// Audit log
	audit.LogSuccess(audit.ActionUserDelete, nil, "system", fmt.Sprintf("user:%d", user.ID), fmt.Sprintf("User '%s' deleted", username))

	fmt.Printf("User '%s' deleted successfully\n", username)
	return nil
}

func runUserPassword(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Prompt for new password
	fmt.Print("Enter new password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)

	fmt.Print("Confirm new password: ")
	confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password != string(confirmBytes) {
		return fmt.Errorf("passwords do not match")
	}

	// Validate password strength
	if err := auth.ValidatePasswordStrength(password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	// Hash password
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.PasswordHash = passwordHash
	if err := db.UpdateUser(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Invalidate all existing sessions
	_ = db.DeleteUserSessions(user.ID)

	// Audit log
	audit.LogSuccess(audit.ActionUserUpdate, nil, "system", fmt.Sprintf("user:%d", user.ID), fmt.Sprintf("Password changed for user '%s'", username))

	fmt.Printf("Password changed successfully for user '%s'\n", username)
	fmt.Println("All existing sessions have been invalidated")
	return nil
}

func runUserShow(cmd *cobra.Command, args []string) error {
	username := args[0]

	// Get user
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	fmt.Printf("User Details:\n")
	fmt.Printf("  ID:         %d\n", user.ID)
	fmt.Printf("  Username:   %s\n", user.Username)
	fmt.Printf("  Email:      %s\n", user.Email)
	fmt.Printf("  Role:       %s\n", user.Role)
	fmt.Printf("  Enabled:    %t\n", user.Enabled)
	fmt.Printf("  Created:    %s\n", user.CreatedAt.Format(time.RFC3339))
	fmt.Printf("  Updated:    %s\n", user.UpdatedAt.Format(time.RFC3339))

	if user.LastLoginAt != nil {
		fmt.Printf("  Last Login: %s\n", user.LastLoginAt.Format(time.RFC3339))
	} else {
		fmt.Printf("  Last Login: never\n")
	}

	// Show permissions
	perms := auth.GetUserPermissions(user)
	fmt.Printf("\nPermissions:\n")
	for _, perm := range perms {
		fmt.Printf("  - %s\n", perm)
	}

	return nil
}
