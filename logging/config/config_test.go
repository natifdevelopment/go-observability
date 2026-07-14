package config

import (
	"os"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/core"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Level != core.LevelInfo {
		t.Errorf("Level = %s, want INFO", core.LevelLabel(cfg.Level))
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want 'json'", cfg.Format)
	}
	if cfg.Output != "console" {
		t.Errorf("Output = %q, want 'console'", cfg.Output)
	}
	if !cfg.MaskEnabled {
		t.Error("MaskEnabled should be true by default")
	}
	if !cfg.EnableCaller {
		t.Error("EnableCaller should be true by default")
	}
}

func TestFromEnv_Defaults(t *testing.T) {
	// Clear all relevant env vars.
	envVars := []string{
		"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUT", "LOG_FILE", "LOG_AUDIT_FILE",
		"LOG_MAX_SIZE", "LOG_MAX_BACKUP", "LOG_MAX_AGE", "LOG_COMPRESS",
		"SERVICE_NAME", "SERVICE_VERSION", "ENVIRONMENT",
		"ENABLE_CALLER", "ENABLE_STACKTRACE", "ENABLE_COLOR", "ENABLE_BODY_LOG",
		"ENABLE_ASYNC", "ASYNC_BUFFER_SIZE", "ASYNC_WORKERS",
		"LOG_MASK_ENABLED", "LOG_AUDIT_ENABLED",
	}
	for _, v := range envVars {
		old := os.Getenv(v)
		os.Unsetenv(v)
		defer os.Setenv(v, old)
	}

	cfg := FromEnv()
	if cfg.Level != core.LevelInfo {
		t.Errorf("Level = %s, want INFO", core.LevelLabel(cfg.Level))
	}
	if cfg.ServiceName != "unknown" {
		t.Errorf("ServiceName = %q, want 'unknown'", cfg.ServiceName)
	}
}

func TestFromEnv_Custom(t *testing.T) {
	// Save and restore all env vars.
	envVars := map[string]string{
		"LOG_LEVEL":         "DEBUG",
		"LOG_FORMAT":        "console",
		"LOG_OUTPUT":        "file",
		"LOG_FILE":          "/var/log/app.log",
		"SERVICE_NAME":      "my-api",
		"SERVICE_VERSION":   "2.0.0",
		"ENVIRONMENT":       "production",
		"ENABLE_COLOR":      "true",
		"ENABLE_ASYNC":      "true",
		"ASYNC_BUFFER_SIZE": "8192",
		"ASYNC_WORKERS":     "4",
		"LOG_MASK_ENABLED":  "false",
		"LOG_AUDIT_ENABLED": "true",
		"LOG_AUDIT_FILE":    "/var/log/audit.log",
	}
	saved := make(map[string]string)
	for k, v := range envVars {
		saved[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	defer func() {
		for k, v := range saved {
			os.Setenv(k, v)
		}
	}()

	cfg := FromEnv()

	if cfg.Level != core.LevelDebug {
		t.Errorf("Level = %s, want DEBUG", core.LevelLabel(cfg.Level))
	}
	if cfg.Format != "console" {
		t.Errorf("Format = %q, want 'console'", cfg.Format)
	}
	if cfg.Output != "file" {
		t.Errorf("Output = %q, want 'file'", cfg.Output)
	}
	if cfg.FilePath != "/var/log/app.log" {
		t.Errorf("FilePath = %q", cfg.FilePath)
	}
	if cfg.ServiceName != "my-api" {
		t.Errorf("ServiceName = %q", cfg.ServiceName)
	}
	if cfg.ServiceVersion != "2.0.0" {
		t.Errorf("ServiceVersion = %q", cfg.ServiceVersion)
	}
	if cfg.Environment != "production" {
		t.Errorf("Environment = %q", cfg.Environment)
	}
	if !cfg.EnableColor {
		t.Error("EnableColor should be true")
	}
	if !cfg.EnableAsync {
		t.Error("EnableAsync should be true")
	}
	if cfg.AsyncBufferSize != 8192 {
		t.Errorf("AsyncBufferSize = %d, want 8192", cfg.AsyncBufferSize)
	}
	if cfg.AsyncWorkers != 4 {
		t.Errorf("AsyncWorkers = %d, want 4", cfg.AsyncWorkers)
	}
	if cfg.MaskEnabled {
		t.Error("MaskEnabled should be false")
	}
	if !cfg.AuditEnabled {
		t.Error("AuditEnabled should be true")
	}
	if cfg.AuditFile != "/var/log/audit.log" {
		t.Errorf("AuditFile = %q", cfg.AuditFile)
	}
}

func TestFromEnv_LevelVariations(t *testing.T) {
	levels := map[string]core.Level{
		"trace":   core.LevelTrace,
		"TRACE":   core.LevelTrace,
		"debug":   core.LevelDebug,
		"info":    core.LevelInfo,
		"warn":    core.LevelWarn,
		"warning": core.LevelWarn,
		"error":   core.LevelError,
		"fatal":   core.LevelFatal,
		"panic":   core.LevelPanic,
	}
	for input, expected := range levels {
		old := os.Getenv("LOG_LEVEL")
		os.Setenv("LOG_LEVEL", input)
		cfg := FromEnv()
		os.Setenv("LOG_LEVEL", old)
		if cfg.Level != expected {
			t.Errorf("LOG_LEVEL=%q -> Level = %s, want %s", input, core.LevelLabel(cfg.Level), core.LevelLabel(expected))
		}
	}
}

func TestFromEnv_InvalidLevel(t *testing.T) {
	old := os.Getenv("LOG_LEVEL")
	os.Setenv("LOG_LEVEL", "INVALID")
	defer os.Setenv("LOG_LEVEL", old)

	cfg := FromEnv()
	// Should fall back to default (INFO).
	if cfg.Level != core.LevelInfo {
		t.Errorf("invalid level should default to INFO, got %s", core.LevelLabel(cfg.Level))
	}
}

func TestFromEnv_BoolVariations(t *testing.T) {
	boolVars := map[string]bool{
		"true":  true,
		"True":  true,
		"TRUE":  true,
		"1":     true,
		"yes":   true,
		"on":    true,
		"false": false,
		"0":     false,
		"no":    false,
		"off":   false,
	}
	for input, expected := range boolVars {
		old := os.Getenv("ENABLE_COLOR")
		os.Setenv("ENABLE_COLOR", input)
		cfg := FromEnv()
		os.Setenv("ENABLE_COLOR", old)
		if cfg.EnableColor != expected {
			t.Errorf("ENABLE_COLOR=%q -> EnableColor = %v, want %v", input, cfg.EnableColor, expected)
		}
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config should not error: %v", err)
	}
}

func TestValidate_ValidWithFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = "/var/log/app.log"
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid file config should not error: %v", err)
	}
}

func TestValidate_InvalidFormat(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Format = "xml"
	if err := cfg.Validate(); err == nil {
		t.Error("invalid format should error")
	}
}

func TestValidate_InvalidOutput(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "kafka"
	if err := cfg.Validate(); err == nil {
		t.Error("invalid output should error")
	}
}

func TestValidate_FileOutputNoPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = ""
	if err := cfg.Validate(); err == nil {
		t.Error("file output without path should error")
	}
}

func TestValidate_PathTraversal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = "../../../etc/passwd"
	if err := cfg.Validate(); err == nil {
		t.Error("path traversal should error")
	}
}

func TestValidate_AuditTraversal(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditFile = "../../audit.log"
	if err := cfg.Validate(); err == nil {
		t.Error("audit path traversal should error")
	}
}

func TestValidate_InvalidMaxSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxSizeMB = 0
	if err := cfg.Validate(); err == nil {
		t.Error("max_size_mb=0 should error")
	}
}

func TestValidate_NegativeMaxBackups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxBackups = -1
	if err := cfg.Validate(); err == nil {
		t.Error("negative max_backups should error")
	}
}

func TestValidate_NegativeMaxAge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxAgeDays = -1
	if err := cfg.Validate(); err == nil {
		t.Error("negative max_age_days should error")
	}
}

func TestValidate_InvalidAsyncBufferSize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AsyncBufferSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("async_buffer_size=0 should error")
	}
}

func TestValidate_InvalidAsyncWorkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AsyncWorkers = 0
	if err := cfg.Validate(); err == nil {
		t.Error("async_workers=0 should error")
	}
}

func TestValidate_InvalidEnvironment(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Environment = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("invalid environment should error")
	}
}

func TestValidate_AuditEnabledNoFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditFile = ""
	if err := cfg.Validate(); err == nil {
		t.Error("audit enabled without file should error")
	}
}

func TestValidate_AuditEnabledWithFile(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditFile = "/var/log/audit.log"
	if err := cfg.Validate(); err != nil {
		t.Errorf("audit enabled with file should not error: %v", err)
	}
}

func TestValidateAndLog(t *testing.T) {
	cfg := DefaultConfig()
	result, err := cfg.ValidateAndLog()
	if err != nil {
		t.Errorf("ValidateAndLog failed: %v", err)
	}
	if result != cfg {
		t.Error("ValidateAndLog should return the same config")
	}
}

func TestValidateAndLog_Invalid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Format = "invalid"
	_, err := cfg.ValidateAndLog()
	if err == nil {
		t.Error("invalid config should error")
	}
}

func TestConfig_String(t *testing.T) {
	cfg := DefaultConfig()
	s := cfg.String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}

func TestValidate_ValidEnvironments(t *testing.T) {
	for _, env := range []string{"development", "staging", "production", "test"} {
		cfg := DefaultConfig()
		cfg.Environment = env
		if err := cfg.Validate(); err != nil {
			t.Errorf("environment %q should be valid: %v", env, err)
		}
	}
}

func TestValidate_BothOutputNoPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "both"
	cfg.FilePath = ""
	if err := cfg.Validate(); err == nil {
		t.Error("both output without path should error")
	}
}

func TestValidate_BothOutputWithPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Output = "both"
	cfg.FilePath = "/var/log/app.log"
	if err := cfg.Validate(); err != nil {
		t.Errorf("both output with path should not error: %v", err)
	}
}
