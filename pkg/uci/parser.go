package uci

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Parse parses a UCI configuration from a reader
func Parse(r io.Reader) (*Config, error) {
	config := NewConfig()
	scanner := bufio.NewScanner(r)

	var currentSection *Section
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse config line
		if strings.HasPrefix(line, "config ") {
			// Save previous section if exists
			if currentSection != nil {
				config.AddSection(currentSection)
			}

			// Parse: config <type> ['name']
			parts := parseQuotedLine(line[7:]) // Skip "config "
			if len(parts) < 1 {
				return nil, fmt.Errorf("line %d: invalid config line", lineNum)
			}

			sectionType := parts[0]
			sectionName := ""
			if len(parts) > 1 {
				sectionName = parts[1]
			}

			currentSection = NewSection(sectionType, sectionName)
			continue
		}

		// Parse option line
		if strings.HasPrefix(line, "option ") {
			if currentSection == nil {
				return nil, fmt.Errorf("line %d: option outside of section", lineNum)
			}

			// Parse: option 'key' 'value'
			parts := parseQuotedLine(line[7:]) // Skip "option "
			if len(parts) != 2 {
				return nil, fmt.Errorf("line %d: invalid option line", lineNum)
			}

			currentSection.SetOption(parts[0], parts[1])
			continue
		}

		// Parse list line
		if strings.HasPrefix(line, "list ") {
			if currentSection == nil {
				return nil, fmt.Errorf("line %d: list outside of section", lineNum)
			}

			// Parse: list 'key' 'value'
			parts := parseQuotedLine(line[5:]) // Skip "list "
			if len(parts) != 2 {
				return nil, fmt.Errorf("line %d: invalid list line", lineNum)
			}

			currentSection.AddListValue(parts[0], parts[1])
			continue
		}

		return nil, fmt.Errorf("line %d: unknown syntax: %s", lineNum, line)
	}

	// Add last section
	if currentSection != nil {
		config.AddSection(currentSection)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}

	return config, nil
}

// parseQuotedLine splits a line into quoted or unquoted tokens
// Example: "interface 'wan'" -> ["interface", "wan"]
func parseQuotedLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := rune(0)

	for _, r := range line {
		switch {
		case !inQuotes && (r == '\'' || r == '"'):
			// Start quoted section
			inQuotes = true
			quoteChar = r
		case inQuotes && r == quoteChar:
			// End quoted section
			inQuotes = false
			parts = append(parts, current.String())
			current.Reset()
			quoteChar = 0
		case inQuotes:
			// Inside quotes
			current.WriteRune(r)
		case r == ' ' || r == '\t':
			// Whitespace outside quotes
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			// Regular character
			current.WriteRune(r)
		}
	}

	// Add final token if exists
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// Write writes a UCI configuration to a writer
func Write(w io.Writer, config *Config) error {
	for i, section := range config.Sections {
		// Add blank line between sections (except before first)
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		// Write section header
		if section.Name != "" {
			if _, err := fmt.Fprintf(w, "config %s '%s'\n", section.Type, section.Name); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintf(w, "config %s\n", section.Type); err != nil {
				return err
			}
		}

		// Write options
		for key, value := range section.Options {
			if _, err := fmt.Fprintf(w, "\toption '%s' '%s'\n", key, escapeQuotes(value)); err != nil {
				return err
			}
		}

		// Write lists
		for key, values := range section.Lists {
			for _, value := range values {
				if _, err := fmt.Fprintf(w, "\tlist '%s' '%s'\n", key, escapeQuotes(value)); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// escapeQuotes escapes single quotes in a string
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}
