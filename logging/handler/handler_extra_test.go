package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// --- Formatter edge cases ---

func TestJSONFormatter_FloatValue(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Float64("rate", 0.75))

	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["rate"] != 0.75 {
		t.Errorf("rate = %v, want 0.75", result["rate"])
	}
}

func TestJSONFormatter_EmptyMessage(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "", 0)
	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["msg"] != "" {
		t.Errorf("msg = %v, want empty", result["msg"])
	}
}

func TestJSONFormatter_SpecialCharsInMessage(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, `msg with "quotes" and \backslash`, 0)
	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("invalid JSON due to escaping: %v\n%s", err, string(payload))
	}
}

func TestJSONFormatter_UnicodeInKey(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.String("key\"with\"quotes", "val"))
	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	// Should be valid JSON despite special chars in key.
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, string(payload))
	}
}

func TestJSONFormatter_AnyComplexValue(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Any("data", map[string]any{"nested": "value", "num": 42}))

	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)
	data, ok := result["data"].(map[string]any)
	if !ok {
		t.Fatal("data should be a map")
	}
	if data["nested"] != "value" {
		t.Errorf("nested = %v", data["nested"])
	}
}

func TestJSONFormatter_LogValuer(t *testing.T) {
	f := NewJSONFormatter()
	type customValuer struct{}
	// slog.LogValuer interface
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Any("custom", "simple_string"))
	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["custom"] != "simple_string" {
		t.Errorf("custom = %v", result["custom"])
	}
}

func TestConsoleFormatter_FloatValue(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Float64("rate", 0.5))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, "rate=0.5") {
		t.Errorf("console should contain rate=0.5: %q", s)
	}
}

func TestConsoleFormatter_DurationValue(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Duration("elapsed", 5*time.Second))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, "elapsed=5s") {
		t.Errorf("console should contain elapsed=5s: %q", s)
	}
}

func TestConsoleFormatter_NilValue(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Any("nil_val", nil))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, "nil_val=<nil>") {
		t.Errorf("console should contain nil_val=<nil>: %q", s)
	}
}

func TestConsoleFormatter_TimeValue(t *testing.T) {
	f := NewConsoleFormatter()
	now := time.Now()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Time("event_time", now))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, "event_time=") {
		t.Errorf("console should contain event_time: %q", s)
	}
}

func TestConsoleFormatter_NestedGroup(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Group("user", slog.String("id", "123")))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, "user.id=123") {
		t.Errorf("console should contain user.id=123: %q", s)
	}
}

func TestConsoleFormatter_EmptyString(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.String("empty", ""))
	payload, _ := f.Format(record)
	s := string(payload)
	if !contains(s, `empty=""`) {
		t.Errorf("console should contain empty=\"\": %q", s)
	}
}

func TestConsoleFormatter_AllColors(t *testing.T) {
	f := NewConsoleFormatterColor()
	levels := []slog.Level{
		slog.LevelDebug - 4, // TRACE
		slog.LevelDebug,     // DEBUG
		slog.LevelInfo,      // INFO
		slog.LevelWarn,      // WARN
		slog.LevelError,     // ERROR
		slog.LevelError + 2, // FATAL
		slog.LevelError + 4, // PANIC
	}
	for _, l := range levels {
		record := slog.NewRecord(time.Now(), l, "msg", 0)
		payload, _ := f.Format(record)
		if len(payload) == 0 {
			t.Errorf("color output empty for level %d", l)
		}
	}
}

// --- Handler options coverage ---

func TestWithFormatter(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(),
		WithFormatter(NewConsoleFormatter()),
	)
	if h.formatter.Name() != "console" {
		t.Error("WithFormatter not applied")
	}
}

func TestWithMaskEngine(t *testing.T) {
	engine := core.NewDefaultMaskEngine()
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithMaskEngine(engine))
	if h.maskEngine != engine {
		t.Error("WithMaskEngine not applied")
	}
}

func TestWithCaller(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithCaller(true))
	if !h.caller {
		t.Error("WithCaller(true) not applied")
	}
}

func TestWithStacktrace(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithStacktrace(false))
	if h.stacktrace {
		t.Error("WithStacktrace(false) not applied")
	}
}

func TestWithHooks(t *testing.T) {
	hook := &countingHook{}
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithHooks(hook))
	if len(h.hooks) != 1 {
		t.Error("WithHooks not applied")
	}
}

func TestWithFilters(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(),
		WithFilters(&messageBlocker{blocked: "test"}),
	)
	if len(h.filters) != 1 {
		t.Error("WithFilters not applied")
	}
}

func TestEnterpriseHandler_WithMultipleGroups(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h).WithGroup("a").WithGroup("b")
	logger.Info("test", slog.String("key", "val"))

	output := read()
	if !contains(output, "a.b.key") {
		t.Errorf("output should contain 'a.b.key': %q", output)
	}
}

func TestEnterpriseHandler_WithGroupAndAttrs(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h).WithGroup("module").With(slog.String("version", "1.0"))
	logger.Info("test", slog.String("name", "handler"))

	output := read()
	if !contains(output, "module.version") {
		t.Errorf("output should contain 'module.version': %q", output)
	}
	if !contains(output, "module.name") {
		t.Errorf("output should contain 'module.name': %q", output)
	}
}

func TestEnterpriseHandler_Stacktrace(t *testing.T) {
	h, cleanup, read := newTestHandler(t, WithStacktrace(true))
	defer cleanup()
	logger := slog.New(h)

	logger.Error("error with stacktrace")

	output := read()
	if !contains(output, "stacktrace") {
		t.Errorf("output should contain stacktrace: %q", output)
	}
}

func TestEnterpriseHandler_HookError(t *testing.T) {
	errorHook := &failingHook{}
	h, cleanup, read := newTestHandler(t, WithHooks(errorHook))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("test with hook error")

	output := read()
	// Should still write despite hook error.
	if !contains(output, "test with hook error") {
		t.Errorf("output should contain message despite hook error: %q", output)
	}
}

func TestEnterpriseHandler_WithDisabledMasking(t *testing.T) {
	cfg := core.NewDefaultMaskConfig()
	cfg.Enabled = false
	engine := core.NewMaskEngine(cfg)
	h, cleanup, read := newTestHandler(t, WithMaskEngine(engine))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("test", slog.String("password", "secret123"))

	output := read()
	// With masking disabled, password should be in plaintext.
	if !contains(output, "secret123") {
		t.Errorf("password should not be masked when disabled: %q", output)
	}
}

func TestEnterpriseHandler_Close(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink())
	if err := h.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// --- Helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type failingHook struct{}

func (h *failingHook) BeforeWrite(_ context.Context, r slog.Record) (slog.Record, error) {
	return r, errHookError
}
func (h *failingHook) AfterWrite(_ context.Context, _ slog.Record, _ []byte, _ error) {}
func (h *failingHook) Name() string                                                    { return "failing" }

var errHookError = &hookError{}

type hookError struct{}

func (e *hookError) Error() string { return "hook error" }
