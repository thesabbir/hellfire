package main

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/thesabbir/hellfire/pkg/db"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	Long:  "View and filter audit logs",
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit logs",
	RunE:  runAuditList,
}

var auditShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show detailed audit log entry",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuditShow,
}

var auditCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old audit logs",
	RunE:  runAuditCleanup,
}

func init() {
	// Audit list flags
	auditListCmd.Flags().String("user", "", "Filter by username")
	auditListCmd.Flags().String("action", "", "Filter by action")
	auditListCmd.Flags().String("status", "", "Filter by status (success/failure)")
	auditListCmd.Flags().String("resource", "", "Filter by resource")
	auditListCmd.Flags().String("from", "", "Filter from date (YYYY-MM-DD)")
	auditListCmd.Flags().String("to", "", "Filter to date (YYYY-MM-DD)")
	auditListCmd.Flags().Int("limit", 50, "Maximum number of logs to show")
	auditListCmd.Flags().Int("offset", 0, "Offset for pagination")

	// Audit cleanup flags
	auditCleanupCmd.Flags().Int("days", 90, "Delete logs older than N days")

	// Add subcommands
	auditCmd.AddCommand(
		auditListCmd,
		auditShowCmd,
		auditCleanupCmd,
	)
}

func runAuditList(cmd *cobra.Command, args []string) error {
	// Get filters from flags
	filters := make(map[string]interface{})

	if username, _ := cmd.Flags().GetString("user"); username != "" {
		// Look up user by username
		user, err := db.GetUserByUsername(username)
		if err != nil {
			return fmt.Errorf("user not found: %w", err)
		}
		filters["user_id"] = user.ID
	}

	if action, _ := cmd.Flags().GetString("action"); action != "" {
		filters["action"] = action
	}

	if status, _ := cmd.Flags().GetString("status"); status != "" {
		filters["status"] = status
	}

	if resource, _ := cmd.Flags().GetString("resource"); resource != "" {
		filters["resource"] = resource
	}

	if fromStr, _ := cmd.Flags().GetString("from"); fromStr != "" {
		from, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return fmt.Errorf("invalid from date: %w", err)
		}
		filters["from"] = from
	}

	if toStr, _ := cmd.Flags().GetString("to"); toStr != "" {
		to, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return fmt.Errorf("invalid to date: %w", err)
		}
		// Set to end of day
		to = to.Add(24*time.Hour - time.Second)
		filters["to"] = to
	}

	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")

	// Get audit logs
	logs, total, err := db.ListAuditLogs(filters, limit, offset)
	if err != nil {
		return fmt.Errorf("failed to list audit logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Println("No audit logs found")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTIME\tUSER\tACTION\tRESOURCE\tSTATUS\tMESSAGE")
	fmt.Fprintln(w, "--\t----\t----\t------\t--------\t------\t-------")

	for _, log := range logs {
		timestamp := log.CreatedAt.Format("2006-01-02 15:04:05")
		message := log.Message
		if len(message) > 40 {
			message = message[:37] + "..."
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			log.ID,
			timestamp,
			log.Username,
			log.Action,
			log.Resource,
			log.Status,
			message,
		)
	}

	w.Flush()

	fmt.Printf("\nShowing %d-%d of %d total logs\n", offset+1, offset+len(logs), total)

	if offset+len(logs) < int(total) {
		fmt.Printf("Use --offset=%d to see more\n", offset+len(logs))
	}

	return nil
}

func runAuditShow(cmd *cobra.Command, args []string) error {
	idStr := args[0]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid log ID: %w", err)
	}

	// Get single audit log
	var log db.AuditLog
	if err := db.DB.First(&log, id).Error; err != nil {
		return fmt.Errorf("audit log not found: %w", err)
	}

	// Print detailed information
	fmt.Printf("Audit Log #%d\n", log.ID)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	fmt.Printf("Timestamp:  %s\n", log.CreatedAt.Format(time.RFC3339))
	fmt.Printf("User:       %s", log.Username)
	if log.UserID != nil {
		fmt.Printf(" (ID: %d)", *log.UserID)
	}
	fmt.Println()

	fmt.Printf("Action:     %s\n", log.Action)
	fmt.Printf("Status:     %s\n", log.Status)

	if log.Resource != "" {
		fmt.Printf("Resource:   %s\n", log.Resource)
	}

	if log.Message != "" {
		fmt.Printf("Message:    %s\n", log.Message)
	}

	if log.IPAddress != "" {
		fmt.Printf("IP Address: %s\n", log.IPAddress)
	}

	if log.TxID != "" {
		fmt.Printf("Transaction: %s\n", log.TxID)
	}

	if log.Duration > 0 {
		fmt.Printf("Duration:   %dms\n", log.Duration)
	}

	if log.Error != "" {
		fmt.Printf("\nError:\n%s\n", log.Error)
	}

	if log.Details != "" {
		fmt.Printf("\nDetails:\n%s\n", log.Details)
	}

	return nil
}

func runAuditCleanup(cmd *cobra.Command, args []string) error {
	days, _ := cmd.Flags().GetInt("days")

	if days < 1 {
		return fmt.Errorf("days must be at least 1")
	}

	// Confirm cleanup
	fmt.Printf("This will delete all audit logs older than %d days.\n", days)
	fmt.Printf("Are you sure? (yes/no): ")
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "yes" {
		fmt.Println("Cleanup cancelled")
		return nil
	}

	// Calculate cutoff time
	cutoff := time.Now().AddDate(0, 0, -days)

	// Delete old logs
	result := db.DB.Where("created_at < ?", cutoff).Delete(&db.AuditLog{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup audit logs: %w", result.Error)
	}

	fmt.Printf("Deleted %d audit log(s) older than %s\n", result.RowsAffected, cutoff.Format("2006-01-02"))

	return nil
}
