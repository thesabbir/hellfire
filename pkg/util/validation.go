package util

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// ValidateInterfaceName validates a network interface name
// Interface names can only contain alphanumeric, dash, underscore, dot
func ValidateInterfaceName(name string) error {
	if name == "" {
		return fmt.Errorf("interface name cannot be empty")
	}

	if len(name) > 15 {
		return fmt.Errorf("interface name too long (max 15 chars): %s", name)
	}

	matched, err := regexp.MatchString(`^[a-zA-Z0-9\-_.]+$`, name)
	if err != nil {
		return fmt.Errorf("failed to validate interface name: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid interface name (only alphanumeric, dash, underscore, dot allowed): %s", name)
	}

	return nil
}

// ValidateIPAddress validates an IPv4 or IPv6 address
func ValidateIPAddress(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	return nil
}

// ValidatePort validates a port number (1-65535)
func ValidatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Handle port ranges (e.g., "80-443")
	if strings.Contains(port, "-") {
		parts := strings.Split(port, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port range: %s", port)
		}
		if err := ValidatePort(parts[0]); err != nil {
			return err
		}
		return ValidatePort(parts[1])
	}

	// Handle port lists (e.g., "80,443")
	if strings.Contains(port, ",") {
		parts := strings.Split(port, ",")
		for _, p := range parts {
			if err := ValidatePort(strings.TrimSpace(p)); err != nil {
				return err
			}
		}
		return nil
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %s", port)
	}

	if p < 1 || p > 65535 {
		return fmt.Errorf("port out of range (1-65535): %d", p)
	}

	return nil
}

// ValidateProtocol validates a network protocol
func ValidateProtocol(proto string) error {
	if proto == "" {
		return nil // Empty protocol is allowed (means any)
	}

	validProtos := map[string]bool{
		"tcp":    true,
		"udp":    true,
		"icmp":   true,
		"icmpv6": true,
		"esp":    true,
		"ah":     true,
		"sctp":   true,
		"all":    true,
	}

	proto = strings.ToLower(proto)
	if !validProtos[proto] {
		return fmt.Errorf("invalid protocol: %s", proto)
	}

	return nil
}

// ValidateNetmask validates a netmask
func ValidateNetmask(netmask string) error {
	if netmask == "" {
		return fmt.Errorf("netmask cannot be empty")
	}

	// Check if it's a valid IP address
	ip := net.ParseIP(netmask)
	if ip == nil {
		return fmt.Errorf("invalid netmask: %s", netmask)
	}

	// Validate it's a proper netmask (contiguous 1s followed by 0s)
	ip = ip.To4()
	if ip == nil {
		return fmt.Errorf("invalid IPv4 netmask: %s", netmask)
	}

	return nil
}

// ValidateMAC validates a MAC address
func ValidateMAC(mac string) error {
	if mac == "" {
		return fmt.Errorf("MAC address cannot be empty")
	}

	_, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %s", mac)
	}

	return nil
}

// ValidateHostname validates a hostname or domain name
func ValidateHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if len(hostname) > 253 {
		return fmt.Errorf("hostname too long (max 253 chars)")
	}

	// Hostname regex: alphanumeric and hyphens, dots for FQDN
	matched, err := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`, hostname)
	if err != nil {
		return fmt.Errorf("failed to validate hostname: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid hostname: %s", hostname)
	}

	return nil
}

// SanitizeString removes potentially dangerous characters from a string
func SanitizeString(s string) string {
	// Remove shell metacharacters and other dangerous characters
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\n", "\r", "\\"}
	result := s
	for _, char := range dangerous {
		result = strings.ReplaceAll(result, char, "")
	}
	return result
}
