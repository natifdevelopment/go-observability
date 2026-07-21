// Package config provides configuration for the logging framework.
//
// Configuration sources (in priority order):
//  1. Programmatic options (builder/functional options)
//  2. Environment variables
//  3. Default values
//
// # Environment Variables
//
// Core:
//
//	LOG_LEVEL              Log level (TRACE/DEBUG/INFO/WARN/ERROR/FATAL/PANIC) [INFO]
//	LOG_FORMAT             Output format (json/console) [json]
//	LOG_OUTPUT             Output destination (console/file/both) [console]
//	LOG_FILE               Log file path (for file/both output)
//	LOG_AUDIT_FILE         Audit log file path
//
// Rotation:
//
//	LOG_MAX_SIZE           Max log file size in MB [100]
//	LOG_MAX_BACKUP         Max number of old log files [10]
//	LOG_MAX_AGE            Max days to retain old log files [30]
//	LOG_COMPRESS           Compress rotated files (true/false) [false]
//
// Service:
//
//	SERVICE_NAME           Service name [unknown]
//	SERVICE_VERSION        Service version [0.0.0]
//	ENVIRONMENT            Environment (development/staging/production) [development]
//
// Features:
//
//	ENABLE_CALLER          Include caller info (true/false) [true]
//	ENABLE_STACKTRACE      Include stacktrace for ERROR+ (true/false) [true]
//	ENABLE_COLOR           Color output in console mode (true/false) [false]
//	ENABLE_BODY_LOG        Log HTTP request/response bodies (true/false) [false]
//	ENABLE_ASYNC           Enable async logging (true/false) [false]
//	ASYNC_BUFFER_SIZE      Async queue buffer size [4096]
//	ASYNC_WORKERS          Async worker goroutine count [1]
//
// Masking:
//
//	LOG_MASK_ENABLED       Enable sensitive data masking (true/false) [true]
//
// Audit:
//
//	LOG_AUDIT_ENABLED      Enable audit logging (true/false) [false]
//
// # Security
//
// Config values are validated:
//   - File paths are checked for traversal attempts (../)
//   - Log level is validated against known values
//   - Numeric values are range-checked
//   - Boolean values accept: true/false/1/0/yes/no/on/off
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// GlobalConfig holds the logging configuration set by the application.
// When non-nil, FromEnv returns a copy of it instead of reading environment
// variables, so services can centralize config in configs.* globals.
var GlobalConfig *Config

// Config holds all logging configuration.
type Config struct {
	// Core
	Level     core.Level `json:"level" yaml:"level"`
	Format    string     `json:"format" yaml:"format"`
	Output    string     `json:"output" yaml:"output"`
	FilePath  string     `json:"file_path" yaml:"file_path"`
	AuditFile string     `json:"audit_file" yaml:"audit_file"`

	// Rotation
	MaxSizeMB  int  `json:"max_size_mb" yaml:"max_size_mb"`
	MaxBackups int  `json:"max_backups" yaml:"max_backups"`
	MaxAgeDays int  `json:"max_age_days" yaml:"max_age_days"`
	Compress   bool `json:"compress" yaml:"compress"`

	// Service
	ServiceName    string `json:"service_name" yaml:"service_name"`
	ServiceVersion string `json:"service_version" yaml:"service_version"`
	Environment    string `json:"environment" yaml:"environment"`

	// Features
	EnableCaller     bool `json:"enable_caller" yaml:"enable_caller"`
	EnableStacktrace bool `json:"enable_stacktrace" yaml:"enable_stacktrace"`
	EnableColor      bool `json:"enable_color" yaml:"enable_color"`
	EnableBodyLog    bool `json:"enable_body_log" yaml:"enable_body_log"`
	EnableAsync      bool `json:"enable_async" yaml:"enable_async"`
	AsyncBufferSize  int  `json:"async_buffer_size" yaml:"async_buffer_size"`
	AsyncWorkers     int  `json:"async_workers" yaml:"async_workers"`

	// Masking
	MaskEnabled bool `json:"mask_enabled" yaml:"mask_enabled"`

	// Audit
	AuditEnabled bool `json:"audit_enabled" yaml:"audit_enabled"`
}

// DefaultConfig returns a Config with sensible production defaults.
func DefaultConfig() *Config {
	return &Config{
		Level:      core.LevelInfo,
		Format:     "json",
		Output:     "console",
		MaxSizeMB:  100,
		MaxBackups: 10,
		MaxAgeDays: 30,
		Compress:   false,

		ServiceName:    "unknown",
		ServiceVersion: "0.0.0",
		Environment:    "development",

		EnableCaller:     true,
		EnableStacktrace: true,
		EnableColor:      false,
		EnableBodyLog:    false,
		EnableAsync:      false,
		AsyncBufferSize:  4096,
		AsyncWorkers:     1,

		MaskEnabled: true,

		AuditEnabled: false,
	}
}

// FromEnv returns the logging configuration.
// If the application has set GlobalConfig, a copy of it is returned.
// Otherwise configuration is loaded from environment variables with defaults.
func FromEnv() *Config {
	if GlobalConfig != nil {
		c := *GlobalConfig
		return &c
	}
	cfg := DefaultConfig()

	// Core
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		if level, ok := parseLevel(v); ok {
			cfg.Level = level
		}
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Format = v
	}
	if v := os.Getenv("LOG_OUTPUT"); v != "" {
		cfg.Output = v
	}
	if v := os.Getenv("LOG_FILE"); v != "" {
		cfg.FilePath = v
	}
	if v := os.Getenv("LOG_AUDIT_FILE"); v != "" {
		cfg.AuditFile = v
	}

	// Rotation
	cfg.MaxSizeMB = envInt("LOG_MAX_SIZE", cfg.MaxSizeMB)
	cfg.MaxBackups = envInt("LOG_MAX_BACKUP", cfg.MaxBackups)
	cfg.MaxAgeDays = envInt("LOG_MAX_AGE", cfg.MaxAgeDays)
	cfg.Compress = envBool("LOG_COMPRESS", cfg.Compress)

	// Service
	if v := os.Getenv("SERVICE_NAME"); v != "" {
		cfg.ServiceName = v
	}
	if v := os.Getenv("SERVICE_VERSION"); v != "" {
		cfg.ServiceVersion = v
	}
	if v := os.Getenv("ENVIRONMENT"); v != "" {
		cfg.Environment = v
	}

	// Features
	cfg.EnableCaller = envBool("ENABLE_CALLER", cfg.EnableCaller)
	cfg.EnableStacktrace = envBool("ENABLE_STACKTRACE", cfg.EnableStacktrace)
	cfg.EnableColor = envBool("ENABLE_COLOR", cfg.EnableColor)
	cfg.EnableBodyLog = envBool("ENABLE_BODY_LOG", cfg.EnableBodyLog)
	cfg.EnableAsync = envBool("ENABLE_ASYNC", cfg.EnableAsync)
	cfg.AsyncBufferSize = envInt("ASYNC_BUFFER_SIZE", cfg.AsyncBufferSize)
	cfg.AsyncWorkers = envInt("ASYNC_WORKERS", cfg.AsyncWorkers)

	// Masking
	cfg.MaskEnabled = envBool("LOG_MASK_ENABLED", cfg.MaskEnabled)

	// Audit
	cfg.AuditEnabled = envBool("LOG_AUDIT_ENABLED", cfg.AuditEnabled)

	return cfg
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	// Validate format.
	switch c.Format {
	case "json", "console":
		// OK
	default:
		return fmt.Errorf("config: invalid format %q (must be 'json' or 'console')", c.Format)
	}

	// Validate output.
	switch c.Output {
	case "console", "file", "both":
		// OK
	default:
		return fmt.Errorf("config: invalid output %q (must be 'console', 'file', or 'both')", c.Output)
	}

	// Validate file path if output requires it.
	if (c.Output == "file" || c.Output == "both") && c.FilePath == "" {
		return fmt.Errorf("config: LOG_FILE is required when output is 'file' or 'both'")
	}

	// Check for path traversal in file paths.
	if err := validatePath(c.FilePath); err != nil {
		return fmt.Errorf("config: invalid file path: %w", err)
	}
	if err := validatePath(c.AuditFile); err != nil {
		return fmt.Errorf("config: invalid audit file path: %w", err)
	}

	// Validate rotation params.
	if c.MaxSizeMB <= 0 {
		return fmt.Errorf("config: max_size_mb must be > 0, got %d", c.MaxSizeMB)
	}
	if c.MaxBackups < 0 {
		return fmt.Errorf("config: max_backups must be >= 0, got %d", c.MaxBackups)
	}
	if c.MaxAgeDays < 0 {
		return fmt.Errorf("config: max_age_days must be >= 0, got %d", c.MaxAgeDays)
	}

	// Validate async params.
	if c.AsyncBufferSize <= 0 {
		return fmt.Errorf("config: async_buffer_size must be > 0, got %d", c.AsyncBufferSize)
	}
	if c.AsyncWorkers <= 0 {
		return fmt.Errorf("config: async_workers must be > 0, got %d", c.AsyncWorkers)
	}

	// Validate environment.
	switch c.Environment {
	case "development", "staging", "production", "test":
		// OK
	default:
		return fmt.Errorf("config: invalid environment %q", c.Environment)
	}

	// Validate audit: if audit enabled, audit file must be set.
	if c.AuditEnabled && c.AuditFile == "" {
		return fmt.Errorf("config: LOG_AUDIT_FILE is required when audit is enabled")
	}

	return nil
}

// ValidateAndLog validates the config and returns the config or an error.
// Convenience method for chaining.
func (c *Config) ValidateAndLog() (*Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}

// --- Helpers ---

func parseLevel(s string) (core.Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRACE":
		return core.LevelTrace, true
	case "DEBUG":
		return core.LevelDebug, true
	case "INFO":
		return core.LevelInfo, true
	case "WARN", "WARNING":
		return core.LevelWarn, true
	case "ERROR":
		return core.LevelError, true
	case "FATAL":
		return core.LevelFatal, true
	case "PANIC":
		return core.LevelPanic, true
	default:
		return core.LevelInfo, false
	}
}

func envBool(key string, defaultVal bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return defaultVal
	}
	// Support standard bool values.
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	// Support extended values.
	switch v {
	case "yes", "on", "enable", "enabled":
		return true
	case "no", "off", "disable", "disabled":
		return false
	default:
		return defaultVal
	}
}

func envInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

// validatePath checks for path traversal attempts.
func validatePath(path string) error {
	if path == "" {
		return nil
	}
	// Check for path traversal.
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		return fmt.Errorf("path contains traversal: %q", path)
	}
	// Check for absolute paths outside allowed directories (optional).
	// For now, just warn about traversal.
	return nil
}

// String returns a human-readable representation of the config.
// Sensitive values (file paths) are included for debugging.
func (c *Config) String() string {
	return fmt.Sprintf("Config{Level:%s, Format:%s, Output:%s, Service:%s/%s, Env:%s, Mask:%v, Async:%v, Audit:%v}",
		core.LevelLabel(c.Level),
		c.Format,
		c.Output,
		c.ServiceName,
		c.ServiceVersion,
		c.Environment,
		c.MaskEnabled,
		c.EnableAsync,
		c.AuditEnabled,
	)
}
