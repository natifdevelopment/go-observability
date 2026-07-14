package builder

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/handler"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

func TestBuilder_Default(t *testing.T) {
	b := New()
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	if log.SLogger() == nil {
		t.Error("SLogger should not be nil")
	}
	if log.Handler() == nil {
		t.Error("Handler should not be nil")
	}
}

func TestBuilder_WithConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ServiceName = "test-service"
	cfg.ServiceVersion = "1.0.0"

	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()
}

func TestBuilder_WithConsoleSink(t *testing.T) {
	b := New(WithConsoleSink(), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("test message")
}

func TestBuilder_WithFileSink(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.log")

	b := New(WithFileSink(path), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("file sink test")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "file sink test") {
		t.Errorf("file should contain message: %q", string(data))
	}
}

func TestBuilder_WithRotateSink(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "rotate.log")

	b := New(WithRotateSink(sink.RotateConfig{
		Path:       path,
		MaxSizeMB:  1,
		MaxBackups: 3,
	}), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("rotate sink test")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "rotate sink test") {
		t.Errorf("file should contain message: %q", string(data))
	}
}

func TestBuilder_WithFormatter(t *testing.T) {
	b := New(
		WithFormatter(handler.NewConsoleFormatter()),
		WithConsoleSink(),
		SkipDefaultSinks(),
	)
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("console format test")
}

func TestBuilder_WithMaskEngine(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mask.log")

	engine := core.NewDefaultMaskEngine()
	b := New(
		WithMaskEngine(engine),
		WithFileSink(path),
		SkipDefaultSinks(),
	)
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("test", slog.String("password", "secret123"))

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "secret123") {
		t.Error("password should be masked")
	}
}

func TestBuilder_WithDynamicLevel(t *testing.T) {
	b := New(WithDynamicLevel(), WithConsoleSink(), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	// Change level.
	log.SetLevel(core.LevelDebug)
}

func TestBuilder_WithHooks(t *testing.T) {
	hook := &testHook{}
	b := New(WithHooks(hook), WithConsoleSink(), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("hook test")

	if hook.beforeCount != 1 {
		t.Errorf("BeforeWrite should be called once, got %d", hook.beforeCount)
	}
}

func TestBuilder_WithFilters(t *testing.T) {
	b := New(
		WithFilters(&blockFilter{blocked: "filtered"}),
		WithConsoleSink(),
		SkipDefaultSinks(),
	)
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("normal message")
	log.SLogger().Info("filtered message")
}

func TestBuilder_WithAsync(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "async.log")

	cfg := config.DefaultConfig()
	cfg.EnableAsync = true
	cfg.AsyncBufferSize = 100
	cfg.AsyncWorkers = 2

	b := New(WithConfig(cfg), WithFileSink(path), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	for i := 0; i < 10; i++ {
		log.SLogger().Info("async test")
	}
}

func TestBuilder_OutputConsole(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Output = "console"
	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()
}

func TestBuilder_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "output.log")
	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("file output test")

	data, _ := os.ReadFile(cfg.FilePath)
	if !strings.Contains(string(data), "file output test") {
		t.Errorf("file should contain message: %q", string(data))
	}
}

func TestBuilder_OutputBoth(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "both"
	cfg.FilePath = filepath.Join(tmpDir, "both.log")
	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("both output test")

	data, _ := os.ReadFile(cfg.FilePath)
	if !strings.Contains(string(data), "both output test") {
		t.Errorf("file should contain message: %q", string(data))
	}
}

func TestBuilder_InvalidConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Format = "invalid"
	b := New(WithConfig(cfg))
	_, err := b.Build()
	if err == nil {
		t.Error("invalid config should cause Build to fail")
	}
}

func TestBuilder_FileSinkFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = "/nonexistent/path/log.txt"
	b := New(WithConfig(cfg))
	_, err := b.Build()
	if err == nil {
		t.Error("nonexistent file path should cause Build to fail")
	}
}

func TestBuilder_NoSinksFallback(t *testing.T) {
	// With SkipDefaultSinks and no custom sinks, should fallback to console.
	b := New(SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	// Should not panic when logging.
	log.SLogger().Info("fallback test")
}

func TestBuilder_WithAuditSink(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditFile = filepath.Join(tmpDir, "audit.log")
	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	if log.AuditSink() == nil {
		t.Error("AuditSink should not be nil when audit is enabled")
	}

	// Write an audit record.
	err = log.AuditSink().WriteAudit(context.Background(), core.AuditPayload{
		User:      "user-1",
		Action:    "test.action",
		IP:        "10.0.0.1",
		Timestamp: "2026-01-01",
	})
	if err != nil {
		t.Errorf("WriteAudit failed: %v", err)
	}
}

func TestBuilder_Close_Idempotent(t *testing.T) {
	b := New()
	log, _ := b.Build()
	log.Close()
	if err := log.Close(); err != nil {
		t.Errorf("second Close should be idempotent: %v", err)
	}
}

func TestBuilder_Flush(t *testing.T) {
	b := New(WithConsoleSink(), SkipDefaultSinks())
	log, _ := b.Build()
	defer log.Close()

	if err := log.Flush(); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestBuilder_ColorConsole(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Format = "console"
	cfg.EnableColor = true
	b := New(WithConfig(cfg), WithConsoleSink(), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("color test")
}

func TestBuilder_WithMultipleSinks(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi.log")

	b := New(
		WithConsoleSink(),
		WithFileSink(path),
		SkipDefaultSinks(),
	)
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("multi sink test")

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "multi sink test") {
		t.Errorf("file should contain message: %q", string(data))
	}
}

func TestGetHostname(t *testing.T) {
	host := getHostname()
	if host == "" {
		t.Error("hostname should not be empty")
	}
}

// --- Test helpers ---

type testHook struct {
	beforeCount int
	afterCount  int
}

func (h *testHook) BeforeWrite(_ context.Context, r slog.Record) (slog.Record, error) {
	h.beforeCount++
	return r, nil
}
func (h *testHook) AfterWrite(_ context.Context, _ slog.Record, _ []byte, _ error) {
	h.afterCount++
}
func (h *testHook) Name() string { return "test" }

type blockFilter struct {
	blocked string
}

func (f *blockFilter) Allow(_ context.Context, _ core.Level, msg string) bool {
	return !strings.Contains(msg, f.blocked)
}
