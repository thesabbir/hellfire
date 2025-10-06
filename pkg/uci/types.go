package uci

// Config represents a UCI configuration file
type Config struct {
	Sections []*Section
}

// Section represents a config section (named or unnamed)
type Section struct {
	Type    string              // e.g., "interface", "rule"
	Name    string              // optional name, e.g., "wan", "lan"
	Options map[string]string   // single-value options
	Lists   map[string][]string // multi-value lists
}

// NewConfig creates a new empty config
func NewConfig() *Config {
	return &Config{
		Sections: make([]*Section, 0),
	}
}

// NewSection creates a new section
func NewSection(sectionType, name string) *Section {
	return &Section{
		Type:    sectionType,
		Name:    name,
		Options: make(map[string]string),
		Lists:   make(map[string][]string),
	}
}

// AddSection adds a section to the config
func (c *Config) AddSection(s *Section) {
	c.Sections = append(c.Sections, s)
}

// GetSection finds a section by type and name
func (c *Config) GetSection(sectionType, name string) *Section {
	for _, s := range c.Sections {
		if s.Type == sectionType && s.Name == name {
			return s
		}
	}
	return nil
}

// GetSectionsByType returns all sections of a given type
func (c *Config) GetSectionsByType(sectionType string) []*Section {
	sections := make([]*Section, 0)
	for _, s := range c.Sections {
		if s.Type == sectionType {
			sections = append(sections, s)
		}
	}
	return sections
}

// SetOption sets a single-value option in a section
func (s *Section) SetOption(key, value string) {
	s.Options[key] = value
}

// GetOption gets a single-value option from a section
func (s *Section) GetOption(key string) (string, bool) {
	val, ok := s.Options[key]
	return val, ok
}

// AddListValue adds a value to a list option
func (s *Section) AddListValue(key, value string) {
	if s.Lists[key] == nil {
		s.Lists[key] = make([]string, 0)
	}
	s.Lists[key] = append(s.Lists[key], value)
}

// GetList gets a list option from a section
func (s *Section) GetList(key string) []string {
	return s.Lists[key]
}
