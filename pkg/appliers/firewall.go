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

// FirewallApplier applies firewall configuration
type FirewallApplier struct {
	previousRules string // Store previous ruleset for rollback
}

// NewFirewallApplier creates a new firewall applier
func NewFirewallApplier() *FirewallApplier {
	return &FirewallApplier{}
}

// Name returns the applier name
func (a *FirewallApplier) Name() string {
	return "firewall"
}

// Apply applies firewall configuration
func (a *FirewallApplier) Apply(ctx context.Context, config *uci.Config) error {
	// Save current ruleset for rollback
	if err := a.saveCurrentRules(ctx); err != nil {
		logger.Warn("Failed to save current firewall rules", "error", err)
	}

	// Generate nftables configuration
	nftConfig, err := a.generateNftables(config)
	if err != nil {
		return fmt.Errorf("failed to generate nftables config: %w", err)
	}

	// Apply nftables rules
	if err := a.applyNftables(ctx, nftConfig); err != nil {
		return fmt.Errorf("failed to apply nftables rules: %w", err)
	}

	return nil
}

// Validate validates that firewall rules are loaded
func (a *FirewallApplier) Validate(ctx context.Context) error {
	// Check that nftables rules are loaded
	cmd := exec.CommandContext(ctx, "nft", "list", "ruleset")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to validate firewall: %w", err)
	}

	// Basic check: ensure we have some rules
	if len(output) == 0 {
		return fmt.Errorf("no firewall rules loaded")
	}

	return nil
}

// Rollback rolls back firewall changes
func (a *FirewallApplier) Rollback(ctx context.Context) error {
	if a.previousRules == "" {
		return fmt.Errorf("no previous rules to restore")
	}

	logger.Info("Rolling back firewall configuration")

	// Restore previous rules
	return a.applyNftables(ctx, a.previousRules)
}

// saveCurrentRules saves the current nftables ruleset
func (a *FirewallApplier) saveCurrentRules(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "nft", "list", "ruleset")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	a.previousRules = string(output)
	return nil
}

// validatePolicy validates a firewall policy
func validatePolicy(policy string) error {
	policy = strings.ToLower(policy)
	if policy != "accept" && policy != "drop" {
		return fmt.Errorf("invalid policy (must be accept or drop): %s", policy)
	}
	return nil
}

// generateNftables generates nftables configuration from UCI config
func (a *FirewallApplier) generateNftables(config *uci.Config) (string, error) {
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
			if err := validatePolicy(v); err != nil {
				return "", err
			}
			inputPolicy = strings.ToLower(v)
		}
		if v, ok := defaults.GetOption("output"); ok {
			if err := validatePolicy(v); err != nil {
				return "", err
			}
			outputPolicy = strings.ToLower(v)
		}
		if v, ok := defaults.GetOption("forward"); ok {
			if err := validatePolicy(v); err != nil {
				return "", err
			}
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
			// Sanitize rule name to prevent injection
			name = util.SanitizeString(name)
			buf.WriteString(fmt.Sprintf("\t\t# Rule: %s\n", name))
		}

		ruleStr := "\t\t"

		// Source interface
		if src, ok := rule.GetOption("src"); ok && src != "" {
			// Validate interface name
			if err := util.ValidateInterfaceName(src); err != nil {
				return "", fmt.Errorf("invalid source interface %s: %w", src, err)
			}
			ruleStr += fmt.Sprintf("iifname \"%s\" ", src)
		}

		// Destination interface
		if dest, ok := rule.GetOption("dest"); ok && dest != "" {
			// Validate interface name
			if err := util.ValidateInterfaceName(dest); err != nil {
				return "", fmt.Errorf("invalid destination interface %s: %w", dest, err)
			}
			ruleStr += fmt.Sprintf("oifname \"%s\" ", dest)
		}

		// Protocol
		if proto, ok := rule.GetOption("proto"); ok && proto != "" {
			// Validate protocol
			if err := util.ValidateProtocol(proto); err != nil {
				return "", fmt.Errorf("invalid protocol %s: %w", proto, err)
			}
			ruleStr += fmt.Sprintf("%s ", strings.ToLower(proto))
		}

		// Destination port
		if destPort, ok := rule.GetOption("dest_port"); ok && destPort != "" {
			// Validate port
			if err := util.ValidatePort(destPort); err != nil {
				return "", fmt.Errorf("invalid destination port %s: %w", destPort, err)
			}
			ruleStr += fmt.Sprintf("dport %s ", destPort)
		}

		// Source port
		if srcPort, ok := rule.GetOption("src_port"); ok && srcPort != "" {
			// Validate port
			if err := util.ValidatePort(srcPort); err != nil {
				return "", fmt.Errorf("invalid source port %s: %w", srcPort, err)
			}
			ruleStr += fmt.Sprintf("sport %s ", srcPort)
		}

		// Target - validate it's one of the allowed targets
		target := "accept"
		if t, ok := rule.GetOption("target"); ok {
			target = strings.ToLower(t)
			// Only allow safe targets
			validTargets := map[string]bool{
				"accept": true,
				"drop":   true,
				"reject": true,
			}
			if !validTargets[target] {
				return "", fmt.Errorf("invalid target: %s", target)
			}
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
				// Sanitize zone name
				name = util.SanitizeString(name)
				buf.WriteString(fmt.Sprintf("\t\t# Masquerade for zone: %s\n", name))
			}
			// Get network interfaces for this zone
			networks := zone.GetList("network")
			for _, network := range networks {
				// Validate interface name
				if err := util.ValidateInterfaceName(network); err != nil {
					return "", fmt.Errorf("invalid network interface %s: %w", network, err)
				}
				buf.WriteString(fmt.Sprintf("\t\toifname \"%s\" masquerade\n", network))
			}
		}
	}

	buf.WriteString("\t}\n")
	buf.WriteString("}\n")

	return buf.String(), nil
}

// applyNftables applies nftables configuration
func (a *FirewallApplier) applyNftables(ctx context.Context, nftConfig string) error {
	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(nftConfig)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Failed to apply nftables config", "error", stderr.String())
		return fmt.Errorf("nft failed: %s: %w", stderr.String(), err)
	}

	logger.Info("Firewall rules applied successfully")
	return nil
}
