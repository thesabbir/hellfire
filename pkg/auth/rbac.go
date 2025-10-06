package auth

import (
	"fmt"

	"github.com/thesabbir/hellfire/pkg/db"
)

// Permission represents a system permission
type Permission string

const (
	// Config permissions
	PermConfigRead   Permission = "config.read"
	PermConfigWrite  Permission = "config.write"
	PermConfigCommit Permission = "config.commit"

	// User permissions
	PermUserRead   Permission = "user.read"
	PermUserWrite  Permission = "user.write"
	PermUserDelete Permission = "user.delete"

	// Snapshot permissions
	PermSnapshotRead   Permission = "snapshot.read"
	PermSnapshotCreate Permission = "snapshot.create"
	PermSnapshotDelete Permission = "snapshot.delete"

	// Audit permissions
	PermAuditRead Permission = "audit.read"

	// System permissions
	PermSystemRestart Permission = "system.restart"
	PermSystemShell   Permission = "system.shell"
)

// RolePermissions maps roles to their permissions
var RolePermissions = map[db.Role][]Permission{
	db.RoleAdmin: {
		// Full access to everything
		PermConfigRead,
		PermConfigWrite,
		PermConfigCommit,
		PermUserRead,
		PermUserWrite,
		PermUserDelete,
		PermSnapshotRead,
		PermSnapshotCreate,
		PermSnapshotDelete,
		PermAuditRead,
		PermSystemRestart,
		PermSystemShell,
	},
	db.RoleOperator: {
		// Read + write configs, read users, manage snapshots
		PermConfigRead,
		PermConfigWrite,
		PermConfigCommit,
		PermUserRead,
		PermSnapshotRead,
		PermSnapshotCreate,
		PermAuditRead,
	},
	db.RoleViewer: {
		// Read-only access
		PermConfigRead,
		PermUserRead,
		PermSnapshotRead,
		PermAuditRead,
	},
}

// HasPermission checks if a user has a specific permission
func HasPermission(user *db.User, perm Permission) bool {
	if user == nil {
		return false
	}

	perms, ok := RolePermissions[user.Role]
	if !ok {
		return false
	}

	for _, p := range perms {
		if p == perm {
			return true
		}
	}

	return false
}

// HasAnyPermission checks if a user has any of the specified permissions
func HasAnyPermission(user *db.User, perms ...Permission) bool {
	for _, perm := range perms {
		if HasPermission(user, perm) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if a user has all of the specified permissions
func HasAllPermissions(user *db.User, perms ...Permission) bool {
	for _, perm := range perms {
		if !HasPermission(user, perm) {
			return false
		}
	}
	return true
}

// RequirePermission returns an error if the user lacks the permission
func RequirePermission(user *db.User, perm Permission) error {
	if !HasPermission(user, perm) {
		return fmt.Errorf("permission denied: requires %s", perm)
	}
	return nil
}

// GetUserPermissions returns all permissions for a user's role
func GetUserPermissions(user *db.User) []Permission {
	if user == nil {
		return []Permission{}
	}

	perms, ok := RolePermissions[user.Role]
	if !ok {
		return []Permission{}
	}

	return perms
}

// IsAdmin checks if a user is an admin
func IsAdmin(user *db.User) bool {
	return user != nil && user.Role == db.RoleAdmin
}

// CanManageUsers checks if a user can manage other users
func CanManageUsers(user *db.User) bool {
	return HasPermission(user, PermUserWrite)
}

// CanCommitConfig checks if a user can commit config changes
func CanCommitConfig(user *db.User) bool {
	return HasPermission(user, PermConfigCommit)
}
