package handlers

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/thesabbir/hellfire/pkg/bus"
	"github.com/thesabbir/hellfire/pkg/uci"
)

// FirewallHandler handles firewall configuration changes
type FirewallHandler struct {
	configName string
}

// NewFirewallHandler creates a new firewall handler
func NewFirewallHandler() *FirewallHandler {
	h := &FirewallHandler{
		configName: "firewall",
	}

	// Subscribe to config events
	bus.Subscribe(bus.EventConfigCommitted, h.handleCommit)

	return h
}

// handleCommit handles committed configuration changes
func (h *FirewallHandler) handleCommit(event bus.Event) {
	// Check if this is a firewall config change
	changes, ok := event.Data.([]string)
	if !ok {
		return
	}

	firewallChanged := false
	for _, name := range changes {
		if name == h.configName {
			firewallChanged = true
			break
		}
	}

	if !firewallChanged {
		return
	}

	// Apply firewall rules (this would load the config and apply it)
	// For now, we'll just log it
	fmt.Println("Firewall configuration changed, would reload nftables rules here")
}

// GenerateNftables generates nftables configuration from UCI config
func GenerateNftables(config *uci.Config) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("#!/usr/sbin/nft -f\n\n")
	buf.WriteString("flush ruleset\n\n")
	buf.WriteString("table inet router {\n")

	// Get defaults
	defaults := config.GetSection("defaults", "")
	inputPolicy := "accept"
	outputPolicy := "accept"
	forwardPolicy := "drop"

	if defaults != nil {
		if v, ok := defaults.GetOption("input"); ok {
			inputPolicy = strings.ToLower(v)
		}
		if v, ok := defaults.GetOption("output"); ok {
			outputPolicy = strings.ToLower(v)
		}
		if v, ok := defaults.GetOption("forward"); ok {
			forwardPolicy = strings.ToLower(v)
		}
	}

	// Input chain
	buf.WriteString("\tchain input {\n")
	buf.WriteString(fmt.Sprintf("\t\ttype filter hook input priority filter; policy %s;\n\n", inputPolicy))
	buf.WriteString("\t\t# Allow loopback\n")
	buf.WriteString("\t\tiif lo accept\n\n")
	buf.WriteString("\t\t# Allow established/related\n")
	buf.WriteString("\t\tct state established,related accept\n\n")
	buf.WriteString("\t\t# Allow ICMP\n")
	buf.WriteString("\t\tip protocol icmp accept\n")
	buf.WriteString("\t\tip6 nexthdr icmpv6 accept\n")
	buf.WriteString("\t}\n\n")

	// Forward chain with rules
	buf.WriteString("\tchain forward {\n")
	buf.WriteString(fmt.Sprintf("\t\ttype filter hook forward priority filter; policy %s;\n\n", forwardPolicy))
	buf.WriteString("\t\t# Allow established/related\n")
	buf.WriteString("\t\tct state established,related accept\n\n")

	// Add forwarding rules
	rules := config.GetSectionsByType("rule")
	for _, rule := range rules {
		if name, ok := rule.GetOption("name"); ok {
			buf.WriteString(fmt.Sprintf("\t\t# Rule: %s\n", name))
		}

		ruleStr := "\t\t"

		// Source interface
		if src, ok := rule.GetOption("src"); ok && src != "" {
			ruleStr += fmt.Sprintf("iifname \"%s\" ", src)
		}

		// Destination interface
		if dest, ok := rule.GetOption("dest"); ok && dest != "" {
			ruleStr += fmt.Sprintf("oifname \"%s\" ", dest)
		}

		// Protocol
		if proto, ok := rule.GetOption("proto"); ok && proto != "" {
			ruleStr += fmt.Sprintf("%s ", proto)
		}

		// Destination port
		if destPort, ok := rule.GetOption("dest_port"); ok && destPort != "" {
			ruleStr += fmt.Sprintf("dport %s ", destPort)
		}

		// Source port
		if srcPort, ok := rule.GetOption("src_port"); ok && srcPort != "" {
			ruleStr += fmt.Sprintf("sport %s ", srcPort)
		}

		// Target
		target := "accept"
		if t, ok := rule.GetOption("target"); ok {
			target = strings.ToLower(t)
		}
		ruleStr += target

		buf.WriteString(ruleStr + "\n")
	}

	buf.WriteString("\n\t\t# Drop invalid\n")
	buf.WriteString("\t\tct state invalid drop\n")
	buf.WriteString("\t}\n\n")

	// Output chain
	buf.WriteString("\tchain output {\n")
	buf.WriteString(fmt.Sprintf("\t\ttype filter hook output priority filter; policy %s;\n", outputPolicy))
	buf.WriteString("\t}\n\n")

	// NAT chains
	buf.WriteString("\tchain prerouting {\n")
	buf.WriteString("\t\ttype nat hook prerouting priority dstnat; policy accept;\n")
	buf.WriteString("\t}\n\n")

	buf.WriteString("\tchain postrouting {\n")
	buf.WriteString("\t\ttype nat hook postrouting priority srcnat; policy accept;\n\n")

	// Add masquerade rules
	zones := config.GetSectionsByType("zone")
	for _, zone := range zones {
		if masq, ok := zone.GetOption("masq"); ok && masq == "1" {
			if name, ok := zone.GetOption("name"); ok {
				buf.WriteString(fmt.Sprintf("\t\t# Masquerade for zone: %s\n", name))
			}
			// Get network interfaces for this zone
			networks := zone.GetList("network")
			for _, network := range networks {
				buf.WriteString(fmt.Sprintf("\t\toifname \"%s\" masquerade\n", network))
			}
		}
	}

	buf.WriteString("\t}\n")
	buf.WriteString("}\n")

	return buf.String(), nil
}

// ApplyNftables applies nftables configuration
func ApplyNftables(nftConfig string) error {
	cmd := exec.Command("nft", "-f", "-")
	cmd.Stdin = strings.NewReader(nftConfig)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft failed: %s: %w", stderr.String(), err)
	}

	return nil
}
