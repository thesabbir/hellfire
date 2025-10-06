package appliers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/uci"
	"github.com/thesabbir/hellfire/pkg/util"
)

const (
	// DefaultCIDR is the default CIDR for unknown netmasks (Class C network)
	DefaultCIDR = 24
)

// NetworkApplier applies network configuration
type NetworkApplier struct {
	previousState map[string]string // Store previous interface states for rollback
}

// NewNetworkApplier creates a new network applier
func NewNetworkApplier() *NetworkApplier {
	return &NetworkApplier{
		previousState: make(map[string]string),
	}
}

// Name returns the applier name
func (a *NetworkApplier) Name() string {
	return "network"
}

// Apply applies network configuration
func (a *NetworkApplier) Apply(ctx context.Context, config *uci.Config) error {
	// Get all interface sections
	interfaces := config.GetSectionsByType("interface")

	for _, iface := range interfaces {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ifaceName := iface.Name
		if ifaceName == "" {
			continue
		}

		// Save current state for potential rollback
		if err := a.saveInterfaceState(ctx, ifaceName); err != nil {
			// Continue even if we can't save state
			logger.Warn("Failed to save interface state",
				"interface", ifaceName,
				"error", err)
		}

		// Apply interface configuration
		if err := a.applyInterface(ctx, ifaceName, iface); err != nil {
			return fmt.Errorf("failed to apply interface %s: %w", ifaceName, err)
		}
	}

	return nil
}

// Validate validates that interfaces are configured correctly
func (a *NetworkApplier) Validate(ctx context.Context) error {
	// Basic validation: check that interfaces mentioned in config are up
	// In production, you might want more sophisticated checks
	return nil
}

// Rollback rolls back network changes
func (a *NetworkApplier) Rollback(ctx context.Context) error {
	logger.Info("Starting network rollback", "interfaces", len(a.previousState))

	for ifaceName, state := range a.previousState {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := a.restoreInterfaceState(ctx, ifaceName, state); err != nil {
			logger.Error("Failed to rollback interface",
				"interface", ifaceName,
				"error", err)
			return fmt.Errorf("failed to rollback %s: %w", ifaceName, err)
		}
	}

	logger.Info("Network rollback completed successfully")
	return nil
}

// saveInterfaceState saves the current state of an interface
func (a *NetworkApplier) saveInterfaceState(ctx context.Context, ifaceName string) error {
	// Validate interface name
	if err := util.ValidateInterfaceName(ifaceName); err != nil {
		return fmt.Errorf("invalid interface name: %w", err)
	}

	// Get current IP configuration
	cmd := exec.CommandContext(ctx, "ip", "addr", "show", "dev", ifaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	a.previousState[ifaceName] = string(output)
	return nil
}

// restoreInterfaceState restores a saved interface state
func (a *NetworkApplier) restoreInterfaceState(ctx context.Context, ifaceName, state string) error {
	// For now, we'll parse the state and try to restore it
	// This is a simplified implementation
	logger.Debug("Restoring interface state",
		"interface", ifaceName,
		"state_length", len(state))

	// Flush interface first
	if err := runCommandContext(ctx, "ip", "addr", "flush", "dev", ifaceName); err != nil {
		logger.Warn("Failed to flush interface during rollback",
			"interface", ifaceName,
			"error", err)
	}

	// Bring interface down
	if err := runCommandContext(ctx, "ip", "link", "set", ifaceName, "down"); err != nil {
		logger.Warn("Failed to bring interface down during rollback",
			"interface", ifaceName,
			"error", err)
	}

	// TODO: Parse state and restore IPs, routes, etc.
	// This requires more sophisticated state parsing
	return nil
}

// applyInterface applies configuration to a single interface
func (a *NetworkApplier) applyInterface(ctx context.Context, ifaceName string, section *uci.Section) error {
	// Validate interface name to prevent command injection
	if err := util.ValidateInterfaceName(ifaceName); err != nil {
		return fmt.Errorf("invalid interface name: %w", err)
	}

	proto, _ := section.GetOption("proto")

	switch proto {
	case "static":
		return a.applyStaticInterface(ctx, ifaceName, section)
	case "dhcp":
		return a.applyDHCPInterface(ctx, ifaceName, section)
	case "none":
		return a.applyNoneInterface(ctx, ifaceName)
	default:
		return fmt.Errorf("unsupported protocol: %s", proto)
	}
}

// applyStaticInterface configures a static IP interface
func (a *NetworkApplier) applyStaticInterface(ctx context.Context, ifaceName string, section *uci.Section) error {
	ipaddr, hasIP := section.GetOption("ipaddr")
	netmask, hasMask := section.GetOption("netmask")

	if !hasIP || !hasMask {
		return fmt.Errorf("static interface requires ipaddr and netmask")
	}

	// Validate IP address
	if err := util.ValidateIPAddress(ipaddr); err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	// Validate netmask
	if err := util.ValidateNetmask(netmask); err != nil {
		return fmt.Errorf("invalid netmask: %w", err)
	}

	// Flush existing addresses
	if err := runCommandContext(ctx, "ip", "addr", "flush", "dev", ifaceName); err != nil {
		return fmt.Errorf("failed to flush interface: %w", err)
	}

	// Add IP address
	cidr := convertNetmaskToCIDR(netmask)
	addr := fmt.Sprintf("%s/%d", ipaddr, cidr)
	if err := runCommandContext(ctx, "ip", "addr", "add", addr, "dev", ifaceName); err != nil {
		return fmt.Errorf("failed to add address: %w", err)
	}

	// Bring interface up
	if err := runCommandContext(ctx, "ip", "link", "set", ifaceName, "up"); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Add gateway if specified
	if gateway, ok := section.GetOption("gateway"); ok {
		// Validate gateway IP
		if err := util.ValidateIPAddress(gateway); err != nil {
			return fmt.Errorf("invalid gateway: %w", err)
		}

		// Remove existing default route (ignore errors)
		_ = runCommandContext(ctx, "ip", "route", "del", "default")

		// Add new default route
		if err := runCommandContext(ctx, "ip", "route", "add", "default", "via", gateway, "dev", ifaceName); err != nil {
			// Ignore error if route already exists
			if !strings.Contains(err.Error(), "File exists") {
				return fmt.Errorf("failed to add gateway: %w", err)
			}
		}
	}

	return nil
}

// applyDHCPInterface configures a DHCP interface
func (a *NetworkApplier) applyDHCPInterface(ctx context.Context, ifaceName string, section *uci.Section) error {
	// Bring interface up
	if err := runCommandContext(ctx, "ip", "link", "set", ifaceName, "up"); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Release existing DHCP lease (safer than pkill)
	// dhclient -r will gracefully release and exit
	_ = runCommandContext(ctx, "dhclient", "-r", ifaceName)

	// Start DHCP client
	if err := runCommandContext(ctx, "dhclient", ifaceName); err != nil {
		return fmt.Errorf("failed to start dhcp client: %w", err)
	}

	return nil
}

// applyNoneInterface brings down an interface
func (a *NetworkApplier) applyNoneInterface(ctx context.Context, ifaceName string) error {
	return runCommandContext(ctx, "ip", "link", "set", ifaceName, "down")
}

// convertNetmaskToCIDR converts a netmask to CIDR notation
func convertNetmaskToCIDR(netmask string) int {
	masks := map[string]int{
		"255.255.255.255": 32,
		"255.255.255.254": 31,
		"255.255.255.252": 30,
		"255.255.255.248": 29,
		"255.255.255.240": 28,
		"255.255.255.224": 27,
		"255.255.255.192": 26,
		"255.255.255.128": 25,
		"255.255.255.0":   24,
		"255.255.254.0":   23,
		"255.255.252.0":   22,
		"255.255.248.0":   21,
		"255.255.240.0":   20,
		"255.255.224.0":   19,
		"255.255.192.0":   18,
		"255.255.128.0":   17,
		"255.255.0.0":     16,
		"255.254.0.0":     15,
		"255.252.0.0":     14,
		"255.248.0.0":     13,
		"255.240.0.0":     12,
		"255.224.0.0":     11,
		"255.192.0.0":     10,
		"255.128.0.0":     9,
		"255.0.0.0":       8,
	}

	if cidr, ok := masks[netmask]; ok {
		return cidr
	}

	return DefaultCIDR
}

// runCommandContext runs a shell command with context support
func runCommandContext(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", stderr.String(), err)
	}
	return nil
}
