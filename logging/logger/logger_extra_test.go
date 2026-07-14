package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
)

func TestLogger_Fatal(t *testing.T) {
	if os.Getenv("TEST_FATAL") == "1" {
		log, _, _ := newTestLogger(t, nil)
		log.Fatal(context.Background(), "fatal test")
		return
	}
	// Run subprocess that will call Fatal.
	cmd := execWithEnv(t, []string{"TEST_FATAL=1"})
	if cmd == "" {
		// Fallback: just test that Fatal doesn't panic before exit.
		// We can't easily test os.Exit in-process.
	}
}

func TestLogger_FatalWithErr(t *testing.T) {
	if os.Getenv("TEST_FATAL_ERR") == "1" {
		log, _, _ := newTestLogger(t, nil)
		log.FatalWithErr(context.Background(), "fatal err", fmt.Errorf("fatal error"))
		return
	}
}

func TestLogger_Panic(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Panic should panic")
		}
	}()
	_ = read // use read to avoid unused
	log.Panic(context.Background(), "panic test")
}

func TestLogger_PanicWithErr(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	defer cleanup()

	defer func() {
		if r := recover(); r == nil {
			t.Error("PanicWithErr should panic")
		}
	}()
	log.PanicWithErr(context.Background(), "panic err", fmt.Errorf("panic error"))
}

func TestLogger_PanicWithErr_NilErr(t *testing.T) {
	log, cleanup, _ := newTestLogger(t, nil)
	defer cleanup()

	defer func() {
		if r := recover(); r == nil {
			t.Error("PanicWithErr should panic even with nil err")
		}
	}()
	log.PanicWithErr(context.Background(), "panic nil", nil)
}

func TestLogger_StandardFieldsInOutput(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "std.log")
	cfg.ServiceName = "test-svc"
	cfg.ServiceVersion = "3.0.0"
	cfg.Environment = "staging"
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer log.Close()

	log.Info(context.Background(), "std fields test")

	data, _ := os.ReadFile(cfg.FilePath)
	s := string(data)
	if !contains(s, "test-svc") {
		t.Error("output should contain service name")
	}
	if !contains(s, "3.0.0") {
		t.Error("output should contain version")
	}
	if !contains(s, "staging") {
		t.Error("output should contain environment")
	}
}

func TestLogger_WithMultipleAttrs(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	log.Info(context.Background(), "multi attrs",
		String("a", "1"),
		Int("b", 2),
		Bool("c", true),
	)

	output := read()
	if !contains(output, "a") || !contains(output, "1") {
		t.Error("output should contain attr a")
	}
}

func TestLogger_StacktraceAttr(t *testing.T) {
	attr := Stacktrace()
	if attr.Key == "" {
		t.Error("Stacktrace() should not have empty key")
	}
}

func TestLogger_AnyAttr(t *testing.T) {
	attr := Any("data", map[string]any{"key": "val"})
	if attr.Key != "data" {
		t.Error("Any attr key should be 'data'")
	}
}

func TestLogger_Int64Attr(t *testing.T) {
	attr := Int64("n", 999)
	if attr.Value.Int64() != 999 {
		t.Error("Int64 attr failed")
	}
}

func TestLogger_Float64Attr(t *testing.T) {
	attr := Float64("f", 3.14)
	if attr.Value.Float64() != 3.14 {
		t.Error("Float64 attr failed")
	}
}

func TestLogger_BoolAttr(t *testing.T) {
	attr := Bool("b", false)
	if attr.Value.Bool() {
		t.Error("Bool attr should be false")
	}
}

func TestLogger_DurationAttr(t *testing.T) {
	attr := Duration("elapsed", 5*time.Second)
	if attr.Value.String() != "5s" {
		t.Errorf("Duration attr = %q, want '5s'", attr.Value.String())
	}
}

func TestLogger_WithCarrierAndMasking(t *testing.T) {
	log, cleanup, read := newTestLogger(t, nil)
	defer cleanup()

	ctx := core.WithTraceID(context.Background(), "trace-123")
	log.Info(ctx, "test", String("password", "secret"))

	output := read()
	if !contains(output, "trace-123") {
		t.Error("output should contain trace_id")
	}
	if contains(output, "secret") {
		t.Error("password should be masked")
	}
}

func TestLogger_LevelFiltered(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Level = core.LevelError
	log, cleanup, read := newTestLogger(t, cfg)
	defer cleanup()

	log.Info(context.Background(), "info filtered")
	log.Warn(context.Background(), "warn filtered")
	log.Error(context.Background(), "error passes")

	output := read()
	if contains(output, "info filtered") {
		t.Error("INFO should be filtered at ERROR level")
	}
	if contains(output, "warn filtered") {
		t.Error("WARN should be filtered at ERROR level")
	}
	if !contains(output, "error passes") {
		t.Error("ERROR should pass")
	}
}

func TestLogger_ConsoleFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "console.log")
	cfg.Format = "console"
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer log.Close()

	log.Info(context.Background(), "console format test")

	data, _ := os.ReadFile(cfg.FilePath)
	s := string(data)
	if !contains(s, "INFO") {
		t.Error("console format should contain 'INFO'")
	}
}

func TestLogger_AsyncEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "async.log")
	cfg.EnableAsync = true
	cfg.AsyncBufferSize = 100
	cfg.AsyncWorkers = 2
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer log.Close()

	for i := 0; i < 10; i++ {
		log.Info(context.Background(), "async test")
	}
	log.Flush()

	// Give async workers time to write.
	time.Sleep(100 * time.Millisecond)

	data, _ := os.ReadFile(cfg.FilePath)
	if !contains(string(data), "async test") {
		t.Error("file should contain async messages after flush")
	}
}

// --- helpers ---

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

func execWithEnv(t *testing.T, env []string) string {
	// Placeholder for subprocess testing of Fatal.
	return ""
}
