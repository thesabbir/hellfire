package uci

import (
	"bytes"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `
# Network configuration
config interface 'wan'
	option proto 'static'
	option ipaddr '192.168.1.1'
	option netmask '255.255.255.0'
	list dns '8.8.8.8'
	list dns '1.1.1.1'

config interface 'lan'
	option proto 'static'
	option ipaddr '10.0.0.1'

config rule
	option name 'ssh'
	option target 'ACCEPT'
	option proto 'tcp'
	option dest_port '22'
`

	config, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Check number of sections
	if len(config.Sections) != 3 {
		t.Errorf("Expected 3 sections, got %d", len(config.Sections))
	}

	// Check WAN interface
	wan := config.GetSection("interface", "wan")
	if wan == nil {
		t.Fatal("WAN section not found")
	}

	if proto, _ := wan.GetOption("proto"); proto != "static" {
		t.Errorf("Expected proto='static', got '%s'", proto)
	}

	if ipaddr, _ := wan.GetOption("ipaddr"); ipaddr != "192.168.1.1" {
		t.Errorf("Expected ipaddr='192.168.1.1', got '%s'", ipaddr)
	}

	dns := wan.GetList("dns")
	if len(dns) != 2 {
		t.Errorf("Expected 2 DNS entries, got %d", len(dns))
	}
	if dns[0] != "8.8.8.8" || dns[1] != "1.1.1.1" {
		t.Errorf("Unexpected DNS values: %v", dns)
	}

	// Check unnamed section
	rules := config.GetSectionsByType("rule")
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule section, got %d", len(rules))
	}
}

func TestWrite(t *testing.T) {
	config := NewConfig()

	wan := NewSection("interface", "wan")
	wan.SetOption("proto", "static")
	wan.SetOption("ipaddr", "192.168.1.1")
	wan.AddListValue("dns", "8.8.8.8")
	wan.AddListValue("dns", "1.1.1.1")
	config.AddSection(wan)

	rule := NewSection("rule", "")
	rule.SetOption("name", "ssh")
	rule.SetOption("target", "ACCEPT")
	config.AddSection(rule)

	var buf bytes.Buffer
	if err := Write(&buf, config); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	output := buf.String()

	// Verify output contains expected content
	if !strings.Contains(output, "config interface 'wan'") {
		t.Error("Missing interface section")
	}
	if !strings.Contains(output, "option 'proto' 'static'") {
		t.Error("Missing proto option")
	}
	if !strings.Contains(output, "list 'dns' '8.8.8.8'") {
		t.Error("Missing dns list")
	}
}

func TestRoundTrip(t *testing.T) {
	input := `config interface 'wan'
	option proto 'static'
	option ipaddr '192.168.1.1'
	list dns '8.8.8.8'

config rule
	option name 'ssh'
`

	// Parse
	config, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Write
	var buf bytes.Buffer
	if err := Write(&buf, config); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Parse again
	config2, err := Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("Second parse error: %v", err)
	}

	// Compare
	if len(config.Sections) != len(config2.Sections) {
		t.Errorf("Section count mismatch: %d vs %d", len(config.Sections), len(config2.Sections))
	}
}
