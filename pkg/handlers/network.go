package handlers

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/uci"
)

// NetworkHandler handles network configuration changes
type NetworkHandler struct {
	configName string
}

// NewNetworkHandler creates a new network handler
func NewNetworkHandler() *NetworkHandler {
	h := &NetworkHandler{
		configName: "network",
	}

	// Subscribe to config events
	bus.Subscribe(bus.EventConfigCommitted, h.handleCommit)

	return h
}

// handleCommit handles committed configuration changes
func (h *NetworkHandler) handleCommit(event bus.Event) {
	// Check if this is a network config change
	changes, ok := event.Data.([]string)
	if !ok {
		return
	}

	networkChanged := false
	for _, name := range changes {
		if name == h.configName {
			networkChanged = true
			break
		}
	}

	if !networkChanged {
		return
	}

	fmt.Println("Network configuration changed, would apply interface changes here")
}

// ApplyInterface applies network interface configuration
func ApplyInterface(iface string, section *uci.Section) error {
	proto, _ := section.GetOption("proto")

	switch proto {
	case "static":
		return applyStaticInterface(iface, section)
	case "dhcp":
		return applyDHCPInterface(iface, section)
	default:
		return fmt.Errorf("unsupported protocol: %s", proto)
	}
}

// applyStaticInterface configures a static IP interface
func applyStaticInterface(iface string, section *uci.Section) error {
	ipaddr, hasIP := section.GetOption("ipaddr")
	netmask, hasMask := section.GetOption("netmask")

	if !hasIP || !hasMask {
		return fmt.Errorf("static interface requires ipaddr and netmask")
	}

	// Flush existing addresses
	if err := runCommand("ip", "addr", "flush", "dev", iface); err != nil {
		return fmt.Errorf("failed to flush interface: %w", err)
	}

	// Add IP address
	cidr := convertNetmaskToCIDR(netmask)
	addr := fmt.Sprintf("%s/%d", ipaddr, cidr)
	if err := runCommand("ip", "addr", "add", addr, "dev", iface); err != nil {
		return fmt.Errorf("failed to add address: %w", err)
	}

	// Bring interface up
	if err := runCommand("ip", "link", "set", iface, "up"); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Add gateway if specified
	if gateway, ok := section.GetOption("gateway"); ok {
		if err := runCommand("ip", "route", "add", "default", "via", gateway, "dev", iface); err != nil {
			// Ignore error if default route already exists
			if !strings.Contains(err.Error(), "File exists") {
				return fmt.Errorf("failed to add gateway: %w", err)
			}
		}
	}

	return nil
}

// applyDHCPInterface configures a DHCP interface
func applyDHCPInterface(iface string, section *uci.Section) error {
	// Bring interface up
	if err := runCommand("ip", "link", "set", iface, "up"); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	// Start DHCP client (using dhclient)
	if err := runCommand("dhclient", iface); err != nil {
		return fmt.Errorf("failed to start dhcp client: %w", err)
	}

	return nil
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

	return 24 // default
}

// runCommand runs a shell command
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", string(output), err)
	}
	return nil
}
