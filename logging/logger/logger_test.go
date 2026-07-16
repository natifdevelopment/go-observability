package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/builder"
	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
)

func newTestLogger(t *testing.T, cfg *config.Config) (*Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "test.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	cleanup := func() { log.Close() }
	readFn := func() string {
		data, _ := os.ReadFile(cfg.FilePath)
		return string(data)
	}
	return log, cleanup, readFn
}

func TestLogger_Info(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Info(context.Background(), "test info message", String("key", "value"))

	output := read()
	if !strings.Contains(output, "test info message") {
		t.Errorf("output should contain message: %q", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("output should contain key: %q", output)
	}
	if !strings.Contains(output, "value") {
		t.Errorf("output should contain value: %q", output)
	}
}

func TestLogger_AllLevels(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	ctx := context.Background()
	log.Trace(ctx, "trace msg")
	log.Debug(ctx, "debug msg")
	log.Info(ctx, "info msg")
	log.Warn(ctx, "warn msg")
	log.Error(ctx, "error msg")

	output := read()
	// INFO and above should be logged (default level is INFO).
	if strings.Contains(output, "trace msg") {
		t.Error("TRACE should be filtered at INFO level")
	}
	if strings.Contains(output, "debug msg") {
		t.Error("DEBUG should be filtered at INFO level")
	}
	if !strings.Contains(output, "info msg") {
		t.Error("INFO should be logged")
	}
	if !strings.Contains(output, "warn msg") {
		t.Error("WARN should be logged")
	}
	if !strings.Contains(output, "error msg") {
		t.Error("ERROR should be logged")
	}
}

func TestLogger_TraceEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Level = core.LevelTrace
	log, cleanup, read := newTestLogger(t, cfg)
	defer cleanup()

	log.Trace(context.Background(), "trace msg")
	output := read()
	if !strings.Contains(output, "trace msg") {
		t.Error("TRACE should be logged at TRACE level")
	}
}

func TestLogger_WithCarrier(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	ctx := core.WithTraceID(context.Background(), "trace-abc-123")
	ctx = core.WithUser(ctx, "user-1", "john", "admin")

	log.Info(ctx, "with carrier")

	output := read()
	if !strings.Contains(output, "trace-abc-123") {
		t.Error("output should contain trace_id")
	}
	if !strings.Contains(output, "user-1") {
		t.Error("output should contain user_id")
	}
}

func TestLogger_WithMasking(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Info(context.Background(), "test", String("password", "secret123"))

	output := read()
	if strings.Contains(output, "secret123") {
		t.Error("password should be masked")
	}
}

func TestLogger_ErrorWithErr(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	err := fmt.Errorf("database connection failed")
	log.ErrorWithErr(context.Background(), "db error", err)

	output := read()
	if !strings.Contains(output, "database connection failed") {
		t.Error("output should contain error message")
	}
	if !strings.Contains(output, "error") {
		t.Error("output should contain error field")
	}
}

func TestLogger_ErrorWithErr_NilErr(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.ErrorWithErr(context.Background(), "no error", nil)

	output := read()
	if !strings.Contains(output, "no error") {
		t.Error("output should contain message")
	}
}

func TestLogger_With(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	reqLog := log.With(String("request_id", "req-123"))
	reqLog.Info(context.Background(), "processing")

	output := read()
	if !strings.Contains(output, "req-123") {
		t.Error("output should contain request_id from With()")
	}
}

func TestLogger_WithGroup(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	groupedLog := log.WithGroup("module")
	groupedLog.Info(context.Background(), "test", String("name", "handler"))

	output := read()
	if !strings.Contains(output, "module.name") {
		t.Errorf("output should contain grouped key: %q", output)
	}
}

func TestLogger_SetLevel(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	// Default is INFO; DEBUG should be filtered.
	log.Debug(context.Background(), "debug before")
	if read() != "" {
		t.Error("DEBUG should be filtered at INFO level")
	}

	// Change to DEBUG.
	log.SetLevel(core.LevelDebug)
	log.Debug(context.Background(), "debug after")
	output := read()
	if !strings.Contains(output, "debug after") {
		t.Error("DEBUG should pass after level change")
	}
}

func TestLogger_Close(t *testing.T) {
	log, _, _ := newTestLogger(t, nil)
	if err := log.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	// Second close should be idempotent.
	if err := log.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestLogger_LogAfterClose(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	log.Close()
	defer cleanup()

	log.Info(context.Background(), "after close")
	// Should not panic and should not write.
}

func TestLogger_Flush(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	defer cleanup()

	if err := log.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestLogger_SLogger(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	defer cleanup()

	if log.SLogger() == nil {
		t.Error("SLogger should not be nil")
	}
}

func TestLogger_FromEnv(t *testing.T) {
	cfg := FromEnv()
	if cfg == nil {
		t.Error("FromEnv should return config")
	}
}

func TestLogger_Audit(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "test.log")
	cfg.AuditEnabled = true
	cfg.AuditFile = filepath.Join(tmpDir, "audit.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer log.Close()

	audit := log.Audit()
	if audit == nil {
		t.Fatal("Audit should not be nil when audit is enabled")
	}

	err = audit.Log(context.Background(), core.AuditPayload{
		User:      "user-1",
		Action:    "test.action",
		IP:        "10.0.0.1",
		Timestamp: "2026-01-01",
	})
	if err != nil {
		t.Errorf("Audit Log failed: %v", err)
	}

	// Verify audit chain.
	idx, err := audit.Verify(context.Background())
	if err != nil {
		t.Errorf("Audit Verify failed: %v", err)
	}
	if idx != -1 {
		t.Errorf("audit chain should be valid, tampered at %d", idx)
	}
}

func TestLogger_Audit_NotEnabled(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	defer cleanup()

	if log.Audit() != nil {
		t.Error("Audit should be nil when audit is not enabled")
	}
}

func TestLogger_Security_LoginFailed(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Security().LoginFailed(context.Background(), "john", "10.0.0.1")

	output := read()
	if !strings.Contains(output, "login failed") {
		t.Error("output should contain 'login failed'")
	}
	if !strings.Contains(output, "john") {
		t.Error("output should contain username")
	}
}

func TestLogger_Security_LoginSuccess(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Security().LoginSuccess(context.Background(), "john", "10.0.0.1")

	output := read()
	if !strings.Contains(output, "login success") {
		t.Error("output should contain 'login success'")
	}
}

func TestLogger_Security_Logout(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Security().Logout(context.Background(), "john", "10.0.0.1")

	output := read()
	if !strings.Contains(output, "logout") {
		t.Error("output should contain 'logout'")
	}
}

func TestLogger_Security_BruteForce(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Security().BruteForce(context.Background(), "10.0.0.1", 100)

	output := read()
	if !strings.Contains(output, "brute force") {
		t.Error("output should contain 'brute force'")
	}
}

func TestLogger_Security_SQLInjection(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Security().SQLInjection(context.Background(), "DROP TABLE users", "10.0.0.1")

	output := read()
	if !strings.Contains(output, "SQL injection") {
		t.Error("output should contain 'SQL injection'")
	}
}

func TestLogger_Request_LogRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Request().LogRequest(context.Background(), "GET", "/api/users", 200, 50*time.Millisecond)

	output := read()
	if !strings.Contains(output, "request completed") {
		t.Error("output should contain 'request completed'")
	}
	if !strings.Contains(output, "GET") {
		t.Error("output should contain method")
	}
	if !strings.Contains(output, "/api/users") {
		t.Error("output should contain path")
	}
}

// --- Attribute constructors ---

func TestAttrConstructors(t *testing.T) {
	if String("key", "val").Value.String() != "val" {
		t.Error("String attr failed")
	}
	if Int("n", 42).Value.Int64() != 42 {
		t.Error("Int attr failed")
	}
	if Int64("n", 64).Value.Int64() != 64 {
		t.Error("Int64 attr failed")
	}
	if Float64("f", 0.5).Value.Float64() != 0.5 {
		t.Error("Float64 attr failed")
	}
	if !Bool("b", true).Value.Bool() {
		t.Error("Bool attr failed")
	}
	if ErrorAttr(fmt.Errorf("test")).Value.String() != "test" {
		t.Error("Error attr failed")
	}
	if ErrorAttr(nil).Value.String() != "" {
		t.Error("Error(nil) should return empty string")
	}
	if ErrCode("E001").Value.String() != "E001" {
		t.Error("ErrCode attr failed")
	}
}

func TestLogger_JSONOutput(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Info(context.Background(), "json test", String("key", "value"))

	output := read()
	// Should be valid JSON.
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Errorf("output should be valid JSON: %v\n%s", err, output)
	}
	if result["msg"] != "json test" {
		t.Errorf("msg = %v", result["msg"])
	}
}

func TestLogger_NewWithBuilder(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "builder.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	b := builder.New(builder.WithConfig(cfg))
	log, err := NewWithBuilder(b)
	if err != nil {
		t.Fatalf("NewWithBuilder failed: %v", err)
	}
	defer log.Close()

	log.Info(context.Background(), "builder test")

	data, _ := os.ReadFile(cfg.FilePath)
	if !strings.Contains(string(data), "builder test") {
		t.Error("output should contain message")
	}
}

// Ensure slog import is used.
var _ = slog.LevelInfo
