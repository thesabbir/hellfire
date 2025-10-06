package hfconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/thesabbir/hellfire/pkg/logger"
	"github.com/thesabbir/hellfire/pkg/uci"
)

const (
	// DefaultConfigPath is the default path for Hellfire's own config
	DefaultConfigPath = "/etc/config/hellfire"

	// Default values
	DefaultAPIPort           = 8888
	DefaultEnableCORS        = true
	DefaultMinPasswordLength = 12
	DefaultSessionTimeout    = 86400  // 24 hours
	DefaultAbsoluteTimeout   = 604800 // 7 days
	DefaultMaxFailedLogins   = 5
	DefaultEnableSwagger     = false
	DefaultRetentionDays     = 90
	DefaultGlobalRateLimit   = 100
	DefaultAuthRateLimit     = 5
)

// Config represents Hellfire's configuration
type Config struct {
	API       APIConfig
	Security  SecurityConfig
	Audit     AuditConfig
	RateLimit RateLimitConfig
}

// APIConfig contains API server configuration
type APIConfig struct {
	Port           int
	EnableCORS     bool
	AllowedOrigins []string
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	MinPasswordLength    int
	SessionTimeout       int // seconds
	AbsoluteTimeout      int // seconds
	MaxFailedLogins      int
	EnableSwagger        bool
}

// AuditConfig contains audit log settings
type AuditConfig struct {
	Enabled       bool
	RetentionDays int
	ArchivePath   string
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	GlobalRequestsPerMinute int
	GlobalBurst             int
	AuthRequestsPerMinute   int
	AuthBurst               int
}

// Load loads Hellfire configuration from UCI file
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	// Try to load the config
	file, err := os.Open(path)
	if err != nil {
		logger.Warn("Failed to load Hellfire config, using defaults", "path", path, "error", err)
		return DefaultConfig(), nil
	}
	defer file.Close()

	cfg, err := uci.Parse(file)
	if err != nil {
		logger.Warn("Failed to parse Hellfire config, using defaults", "path", path, "error", err)
		return DefaultConfig(), nil
	}

	config := &Config{}

	// Load API config
	if apiSection := cfg.GetSection("server", "api"); apiSection != nil {
		config.API = loadAPIConfig(apiSection)
	} else {
		config.API = defaultAPIConfig()
	}

	// Load security config
	if secSection := cfg.GetSection("settings", "security"); secSection != nil {
		config.Security = loadSecurityConfig(secSection)
	} else {
		config.Security = defaultSecurityConfig()
	}

	// Load audit config
	if auditSection := cfg.GetSection("retention", "audit"); auditSection != nil {
		config.Audit = loadAuditConfig(auditSection)
	} else {
		config.Audit = defaultAuditConfig()
	}

	// Load rate limit config
	config.RateLimit = loadRateLimitConfig(cfg)

	return config, nil
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		API:       defaultAPIConfig(),
		Security:  defaultSecurityConfig(),
		Audit:     defaultAuditConfig(),
		RateLimit: defaultRateLimitConfig(),
	}
}

func loadAPIConfig(section *uci.Section) APIConfig {
	cfg := defaultAPIConfig()

	if port, ok := section.GetOption("port"); ok {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if enableCORS, ok := section.GetOption("enable_cors"); ok {
		cfg.EnableCORS = enableCORS == "1" || strings.ToLower(enableCORS) == "true"
	}

	if origins := section.GetList("allowed_origins"); len(origins) > 0 {
		cfg.AllowedOrigins = origins
	}

	return cfg
}

func loadSecurityConfig(section *uci.Section) SecurityConfig {
	cfg := defaultSecurityConfig()

	if minLen, ok := section.GetOption("min_password_length"); ok {
		if l, err := strconv.Atoi(minLen); err == nil {
			cfg.MinPasswordLength = l
		}
	}

	if timeout, ok := section.GetOption("session_timeout"); ok {
		if t, err := strconv.Atoi(timeout); err == nil {
			cfg.SessionTimeout = t
		}
	}

	if absTimeout, ok := section.GetOption("absolute_session_timeout"); ok {
		if t, err := strconv.Atoi(absTimeout); err == nil {
			cfg.AbsoluteTimeout = t
		}
	}

	if maxFailed, ok := section.GetOption("max_failed_logins"); ok {
		if m, err := strconv.Atoi(maxFailed); err == nil {
			cfg.MaxFailedLogins = m
		}
	}

	if swagger, ok := section.GetOption("enable_swagger"); ok {
		cfg.EnableSwagger = swagger == "1" || strings.ToLower(swagger) == "true"
	}

	return cfg
}

func loadAuditConfig(section *uci.Section) AuditConfig {
	cfg := defaultAuditConfig()

	if enabled, ok := section.GetOption("enabled"); ok {
		cfg.Enabled = enabled == "1" || strings.ToLower(enabled) == "true"
	}

	if days, ok := section.GetOption("retention_days"); ok {
		if d, err := strconv.Atoi(days); err == nil {
			cfg.RetentionDays = d
		}
	}

	if path, ok := section.GetOption("archive_path"); ok {
		cfg.ArchivePath = path
	}

	return cfg
}

func loadRateLimitConfig(cfg *uci.Config) RateLimitConfig {
	rlCfg := defaultRateLimitConfig()

	// Load global rate limit
	if globalSection := cfg.GetSection("global", "ratelimit"); globalSection != nil {
		if rpm, ok := globalSection.GetOption("requests_per_minute"); ok {
			if r, err := strconv.Atoi(rpm); err == nil {
				rlCfg.GlobalRequestsPerMinute = r
				rlCfg.GlobalBurst = r // Default burst = requests per minute
			}
		}

		if burst, ok := globalSection.GetOption("burst"); ok {
			if b, err := strconv.Atoi(burst); err == nil {
				rlCfg.GlobalBurst = b
			}
		}
	}

	// Load auth rate limit
	if authSection := cfg.GetSection("auth", "ratelimit"); authSection != nil {
		if rpm, ok := authSection.GetOption("requests_per_minute"); ok {
			if r, err := strconv.Atoi(rpm); err == nil {
				rlCfg.AuthRequestsPerMinute = r
				rlCfg.AuthBurst = r // Default burst = requests per minute
			}
		}

		if burst, ok := authSection.GetOption("burst"); ok {
			if b, err := strconv.Atoi(burst); err == nil {
				rlCfg.AuthBurst = b
			}
		}
	}

	return rlCfg
}

func defaultAPIConfig() APIConfig {
	return APIConfig{
		Port:       DefaultAPIPort,
		EnableCORS: DefaultEnableCORS,
		AllowedOrigins: []string{
			"http://localhost:5173",  // Default Vite dev server
			"https://router.local",   // Default production
		},
	}
}

func defaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		MinPasswordLength: DefaultMinPasswordLength,
		SessionTimeout:    DefaultSessionTimeout,
		AbsoluteTimeout:   DefaultAbsoluteTimeout,
		MaxFailedLogins:   DefaultMaxFailedLogins,
		EnableSwagger:     DefaultEnableSwagger,
	}
}

func defaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:       true,
		RetentionDays: DefaultRetentionDays,
		ArchivePath:   "/var/lib/hellfire/audit-archive",
	}
}

func defaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		GlobalRequestsPerMinute: DefaultGlobalRateLimit,
		GlobalBurst:             DefaultGlobalRateLimit,
		AuthRequestsPerMinute:   DefaultAuthRateLimit,
		AuthBurst:               DefaultAuthRateLimit,
	}
}

// CreateDefaultConfig creates a default Hellfire config file
func CreateDefaultConfig(path string) error {
	if path == "" {
		path = DefaultConfigPath
	}

	content := `# Hellfire Configuration
# This file configures Hellfire itself using UCI format

config api 'server'
	option port '8888'
	option enable_cors '1'
	list allowed_origins 'http://localhost:5173'
	list allowed_origins 'https://router.local'

config security 'settings'
	option min_password_length '12'
	option session_timeout '86400'
	option absolute_session_timeout '604800'
	option max_failed_logins '5'
	option enable_swagger '0'

config audit 'retention'
	option enabled '1'
	option retention_days '90'
	option archive_path '/var/lib/hellfire/audit-archive'

config ratelimit 'global'
	option requests_per_minute '100'
	option burst '100'

config ratelimit 'auth'
	option requests_per_minute '5'
	option burst '5'
`

	return os.WriteFile(path, []byte(content), 0644)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.API.Port < 1 || c.API.Port > 65535 {
		return fmt.Errorf("invalid API port: %d", c.API.Port)
	}

	if c.Security.MinPasswordLength < 8 {
		return fmt.Errorf("minimum password length must be at least 8")
	}

	if c.Security.SessionTimeout < 300 {
		return fmt.Errorf("session timeout must be at least 300 seconds (5 minutes)")
	}

	if c.Security.AbsoluteTimeout < c.Security.SessionTimeout {
		return fmt.Errorf("absolute timeout must be >= session timeout")
	}

	if c.Audit.RetentionDays < 1 {
		return fmt.Errorf("audit retention must be at least 1 day")
	}

	if c.RateLimit.GlobalRequestsPerMinute < 1 {
		return fmt.Errorf("global rate limit must be at least 1 request per minute")
	}

	if c.RateLimit.AuthRequestsPerMinute < 1 {
		return fmt.Errorf("auth rate limit must be at least 1 request per minute")
	}

	return nil
}
