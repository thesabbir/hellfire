package handlers

import (
	"fmt"
	"os"
	"strings"

	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/config"
	"github.com/thesabbir/hellfire/pkg/uci"
)

// DHCPHandler handles DHCP/DNS configuration changes
type DHCPHandler struct {
	configName string
	manager    *config.Manager
}

// NewDHCPHandler creates a new DHCP handler
func NewDHCPHandler(manager *config.Manager) *DHCPHandler {
	h := &DHCPHandler{
		configName: "dhcp",
		manager:    manager,
	}

	// Subscribe to config events
	bus.Subscribe(bus.EventConfigCommitted, h.handleCommit)

	return h
}

// handleCommit handles committed configuration changes
func (h *DHCPHandler) handleCommit(event bus.Event) {
	// Check if this is a dhcp config change
	changes, ok := event.Data.([]string)
	if !ok {
		return
	}

	dhcpChanged := false
	for _, name := range changes {
		if name == h.configName {
			dhcpChanged = true
			break
		}
	}

	if !dhcpChanged {
		return
	}

	fmt.Println("DHCP configuration changed, applying dnsmasq configuration...")

	if err := h.applyConfig(); err != nil {
		fmt.Printf("Error applying DHCP config: %v\n", err)
	}
}

// applyConfig applies the DHCP/DNS configuration
func (h *DHCPHandler) applyConfig() error {
	cfg, err := h.manager.Load(h.configName)
	if err != nil {
		return fmt.Errorf("failed to load dhcp config: %w", err)
	}

	// Generate dnsmasq configuration
	if err := h.generateDnsmasqConfig(cfg); err != nil {
		return fmt.Errorf("failed to generate dnsmasq config: %w", err)
	}

	// Restart dnsmasq service
	if err := runCommand("killall", "dnsmasq"); err != nil {
		// Ignore error if dnsmasq is not running
	}

	if err := runCommand("dnsmasq", "-C", "/tmp/dnsmasq.conf"); err != nil {
		return fmt.Errorf("failed to start dnsmasq: %w", err)
	}

	fmt.Println("dnsmasq restarted successfully")
	return nil
}

// generateDnsmasqConfig generates dnsmasq configuration file
func (h *DHCPHandler) generateDnsmasqConfig(cfg *uci.Config) error {
	var sb strings.Builder

	// Process dnsmasq section
	for _, section := range cfg.Sections {
		if section.Type == "dnsmasq" {
			h.writeDnsmasqSection(&sb, section)
		}
	}

	// Process dhcp sections
	for _, section := range cfg.Sections {
		if section.Type == "dhcp" {
			h.writeDHCPSection(&sb, section)
		}
	}

	// Write to file
	if err := os.WriteFile("/tmp/dnsmasq.conf", []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write dnsmasq config: %w", err)
	}

	return nil
}

// writeDnsmasqSection writes dnsmasq global options
func (h *DHCPHandler) writeDnsmasqSection(sb *strings.Builder, section *uci.Section) {
	// Domain needed
	if val, ok := section.GetOption("domainneeded"); ok && val == "1" {
		sb.WriteString("domain-needed\n")
	}

	// Bogus private
	if val, ok := section.GetOption("boguspriv"); ok && val == "1" {
		sb.WriteString("bogus-priv\n")
	}

	// Localise queries
	if val, ok := section.GetOption("localise_queries"); ok && val == "1" {
		sb.WriteString("localise-queries\n")
	}

	// Local domain
	if val, ok := section.GetOption("local"); ok {
		sb.WriteString(fmt.Sprintf("local=%s\n", val))
	}

	// Domain
	if val, ok := section.GetOption("domain"); ok {
		sb.WriteString(fmt.Sprintf("domain=%s\n", val))
	}

	// Expand hosts
	if val, ok := section.GetOption("expandhosts"); ok && val == "1" {
		sb.WriteString("expand-hosts\n")
	}

	// Authoritative
	if val, ok := section.GetOption("authoritative"); ok && val == "1" {
		sb.WriteString("authoritative\n")
	}

	// Read ethers
	if val, ok := section.GetOption("readethers"); ok && val == "1" {
		sb.WriteString("read-ethers\n")
	}

	// Lease file
	if val, ok := section.GetOption("leasefile"); ok {
		sb.WriteString(fmt.Sprintf("dhcp-leasefile=%s\n", val))
	}

	// Resolv file
	if val, ok := section.GetOption("resolvfile"); ok {
		sb.WriteString(fmt.Sprintf("resolv-file=%s\n", val))
	}

	// No negative cache
	if val, ok := section.GetOption("nonegcache"); ok && val == "1" {
		sb.WriteString("no-negcache\n")
	}

	// Local service
	if val, ok := section.GetOption("localservice"); ok && val == "1" {
		sb.WriteString("local-service\n")
	}
}

// writeDHCPSection writes DHCP pool configuration
func (h *DHCPHandler) writeDHCPSection(sb *strings.Builder, section *uci.Section) {
	iface, ok := section.GetOption("interface")
	if !ok {
		return
	}

	// Check if DHCP is ignored for this interface
	if ignore, ok := section.GetOption("ignore"); ok && ignore == "1" {
		sb.WriteString(fmt.Sprintf("no-dhcp-interface=%s\n", iface))
		return
	}

	// Get DHCP range
	start, hasStart := section.GetOption("start")
	limit, hasLimit := section.GetOption("limit")
	leasetime, hasLease := section.GetOption("leasetime")

	if hasStart && hasLimit {
		// Get network address from network config
		// For simplicity, assuming lan interface with 10.0.0.x
		networkBase := "10.0.0"

		rangeStr := fmt.Sprintf("dhcp-range=%s,%s.%s,%s.%d",
			iface, networkBase, start, networkBase, mustAtoi(start)+mustAtoi(limit)-1)

		if hasLease {
			rangeStr += fmt.Sprintf(",%s", leasetime)
		}

		sb.WriteString(rangeStr + "\n")
	}

	// DHCP options
	if options := section.GetList("dhcp_option"); len(options) > 0 {
		for _, opt := range options {
			sb.WriteString(fmt.Sprintf("dhcp-option=%s,%s\n", iface, opt))
		}
	}
}

// mustAtoi converts string to int, returns 0 on error
func mustAtoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
