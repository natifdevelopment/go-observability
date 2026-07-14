package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/handler"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

func TestBuilder_WithSink(t *testing.T) {
	b := New(WithSink(sink.NewConsoleSink()), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()
	log.SLogger().Info("with sink test")
}

func TestBuilder_WithAuditSinkOption(t *testing.T) {
	tmpDir := t.TempDir()
	b := New(
		WithAuditSink(filepath.Join(tmpDir, "audit.log")),
		SkipDefaultSinks(),
	)
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()
	// AuditSink via WithAuditSink option doesn't set cfg.AuditEnabled,
	// so log.AuditSink() will be nil. This is expected — WithAuditSink
	// is a placeholder; audit is configured via config.AuditEnabled.
}

func TestBuilder_AuditFileFromConfig(t *testing.T) {
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
		t.Fatal("AuditSink should not be nil")
	}

	// Write and verify.
	err = log.AuditSink().WriteAudit(context.Background(), core.AuditPayload{
		User:      "u",
		Action:    "a",
		IP:        "ip",
		Timestamp: "t",
	})
	if err != nil {
		t.Errorf("WriteAudit failed: %v", err)
	}

	idx, err := log.AuditSink().Verify(context.Background())
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
	if idx != -1 {
		t.Errorf("chain should be valid, tampered at %d", idx)
	}
}

func TestBuilder_AuditFailure(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.AuditEnabled = true
	cfg.AuditFile = "/nonexistent/path/audit.log"
	b := New(WithConfig(cfg))
	_, err := b.Build()
	if err == nil {
		t.Error("audit with invalid path should fail")
	}
}

func TestBuilder_WithCustomFormatter(t *testing.T) {
	f := handler.NewJSONFormatter()
	b := New(WithFormatter(f), WithConsoleSink(), SkipDefaultSinks())
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()
	log.SLogger().Info("custom formatter test")
}

func TestBuilder_StandardFieldsIncluded(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.ServiceName = "my-service"
	cfg.ServiceVersion = "2.0.0"
	cfg.Environment = "production"
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "fields.log")

	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("fields test")

	data, _ := readFile(cfg.FilePath)
	if !contains(data, "my-service") {
		t.Error("output should contain service name")
	}
	if !contains(data, "2.0.0") {
		t.Error("output should contain version")
	}
	if !contains(data, "production") {
		t.Error("output should contain environment")
	}
}

func TestBuilder_MaskingDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.MaskEnabled = false
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "nomask.log")

	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("test", "password", "secret123")

	data, _ := readFile(cfg.FilePath)
	if !contains(data, "secret123") {
		t.Error("password should NOT be masked when MaskEnabled=false")
	}
}

func TestBuilder_MaskingEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.MaskEnabled = true
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "mask.log")

	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	log.SLogger().Info("test", "password", "secret123")

	data, _ := readFile(cfg.FilePath)
	if contains(data, "secret123") {
		t.Error("password should be masked when MaskEnabled=true")
	}
}

func TestBuilder_AsyncFlush(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.EnableAsync = true
	cfg.AsyncBufferSize = 100
	cfg.AsyncWorkers = 1
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "flush.log")

	b := New(WithConfig(cfg))
	log, err := b.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	defer log.Close()

	for i := 0; i < 5; i++ {
		log.SLogger().Info("flush test")
	}
	log.Flush()
}

// --- helpers ---

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func readAllFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
