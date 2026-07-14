package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// helper creates a handler backed by a temp file and returns (handler, cleanup, readFn).
func newTestHandler(t *testing.T, opts ...HandlerOption) (*EnterpriseHandler, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.log")
	fs, err := sink.NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink failed: %v", err)
	}
	defaultOpts := []HandlerOption{
		WithLevel(slog.LevelInfo),
		WithCaller(false),
		WithStacktrace(false),
	}
	allOpts := append(defaultOpts, opts...)
	h := NewEnterpriseHandler(fs, allOpts...)
	cleanup := func() {
		h.Close()
	}
	readFn := func() string {
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(data)
	}
	return h, cleanup, readFn
}

// --- JSON Formatter ---

func TestJSONFormatter_Basic(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatalf("invalid JSON: %v\npayload: %s", err, string(payload))
	}

	if result["msg"] != "test message" {
		t.Errorf("msg = %v, want 'test message'", result["msg"])
	}
	if result["level"] != "INFO" {
		t.Errorf("level = %v, want 'INFO'", result["level"])
	}
	if result["key"] != "value" {
		t.Errorf("key = %v, want 'value'", result["key"])
	}
	if result["timestamp"] == nil {
		t.Error("timestamp should be present")
	}
}

func TestJSONFormatter_AllLevels(t *testing.T) {
	f := NewJSONFormatter()
	levels := []struct {
		level slog.Level
		label string
	}{
		{slog.LevelDebug - 4, "TRACE"},
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
		{slog.Level(core.LevelFatal), "FATAL"},
		{slog.Level(core.LevelPanic), "PANIC"},
	}
	for _, tt := range levels {
		record := slog.NewRecord(time.Now(), tt.level, "msg", 0)
		payload, _ := f.Format(record)
		var result map[string]any
		json.Unmarshal(payload, &result)
		if result["level"] != tt.label {
			t.Errorf("level %d -> label = %v, want %q", tt.level, result["level"], tt.label)
		}
	}
}

func TestJSONFormatter_NestedGroup(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Group("user", slog.String("id", "123"), slog.String("name", "john")))

	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)

	if result["user.id"] != "123" {
		t.Errorf("user.id = %v, want '123'", result["user.id"])
	}
	if result["user.name"] != "john" {
		t.Errorf("user.name = %v, want 'john'", result["user.name"])
	}
}

func TestJSONFormatter_IntAndBool(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Int("count", 42), slog.Bool("active", true))

	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)

	if result["count"].(float64) != 42 {
		t.Errorf("count = %v, want 42", result["count"])
	}
	if result["active"] != true {
		t.Errorf("active = %v, want true", result["active"])
	}
}

func TestJSONFormatter_Name(t *testing.T) {
	if NewJSONFormatter().Name() != "json" {
		t.Error("Name should be 'json'")
	}
}

func TestJSONFormatter_NilValue(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Any("nil_val", nil))

	payload, _ := f.Format(record)
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["nil_val"] != nil {
		t.Errorf("nil_val = %v, want nil", result["nil_val"])
	}
}

func TestJSONFormatter_DurationAndTime(t *testing.T) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(
		slog.Duration("elapsed", 5*time.Second),
		slog.Time("event_time", time.Now()),
	)

	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	var result map[string]any
	json.Unmarshal(payload, &result)
	if result["elapsed"] == nil {
		t.Error("elapsed should be present")
	}
	if result["event_time"] == nil {
		t.Error("event_time should be present")
	}
}

// --- Console Formatter ---

func TestConsoleFormatter_Basic(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
	record.AddAttrs(slog.String("key", "value"))

	payload, err := f.Format(record)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	s := string(payload)
	if !strings.Contains(s, "INFO") {
		t.Errorf("console output should contain 'INFO': %q", s)
	}
	if !strings.Contains(s, `msg="test message"`) {
		t.Errorf("console output should contain msg: %q", s)
	}
	if !strings.Contains(s, "key=value") {
		t.Errorf("console output should contain key=value: %q", s)
	}
}

func TestConsoleFormatter_Quoting(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg with spaces", 0)

	payload, _ := f.Format(record)
	s := string(payload)
	if !strings.Contains(s, `"msg with spaces"`) {
		t.Errorf("message with spaces should be quoted: %q", s)
	}
}

func TestConsoleFormatter_Color(t *testing.T) {
	f := NewConsoleFormatterColor()
	record := slog.NewRecord(time.Now(), slog.LevelError, "error msg", 0)

	payload, _ := f.Format(record)
	s := string(payload)
	if !strings.Contains(s, "\033[31m") {
		t.Errorf("color output should contain ANSI red: %q", s)
	}
}

func TestConsoleFormatter_Name(t *testing.T) {
	if NewConsoleFormatter().Name() != "console" {
		t.Error("Name should be 'console'")
	}
}

func TestConsoleFormatter_IntAndBool(t *testing.T) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.Int("count", 42), slog.Bool("active", true))

	payload, _ := f.Format(record)
	s := string(payload)
	if !strings.Contains(s, "count=42") {
		t.Errorf("console should contain count=42: %q", s)
	}
	if !strings.Contains(s, "active=true") {
		t.Errorf("console should contain active=true: %q", s)
	}
}

// --- Registry ---

func TestFormatterRegistry(t *testing.T) {
	names := RegisteredFormatters()
	if len(names) < 4 {
		t.Errorf("should have at least 4 registered formatters, got %d", len(names))
	}
}

func TestNewFormatterByName(t *testing.T) {
	f, err := NewFormatterByName("json")
	if err != nil {
		t.Fatalf("NewFormatterByName(json) failed: %v", err)
	}
	if f.Name() != "json" {
		t.Errorf("Name = %q", f.Name())
	}
}

func TestNewFormatterByName_Unknown(t *testing.T) {
	_, err := NewFormatterByName("unknown")
	if err == nil {
		t.Error("unknown formatter should error")
	}
}

// --- EnterpriseHandler ---

func TestEnterpriseHandler_Basic(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h)

	logger.Info("test message", slog.String("key", "value"))

	output := read()
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain message: %q", output)
	}
	if !strings.Contains(output, "key") {
		t.Errorf("output should contain key: %q", output)
	}
}

func TestEnterpriseHandler_LevelFilter(t *testing.T) {
	h, cleanup, read := newTestHandler(t, WithLevel(slog.LevelWarn))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := read()
	if strings.Contains(output, "info message") {
		t.Error("INFO should be filtered when level is WARN")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("WARN should pass")
	}
	if !strings.Contains(output, "error message") {
		t.Error("ERROR should pass")
	}
}

func TestEnterpriseHandler_DynamicLevel(t *testing.T) {
	dl := core.NewDynamicLevel(core.LevelInfo)
	h, cleanup, read := newTestHandler(t, WithDynamicLevel(dl))
	defer cleanup()
	logger := slog.New(h)

	logger.Debug("debug msg 1")
	if read() != "" {
		t.Error("DEBUG should be filtered at INFO level")
	}

	h.SetLevel(core.LevelDebug)
	logger.Debug("debug msg 2")
	output := read()
	if !strings.Contains(output, "debug msg 2") {
		t.Errorf("DEBUG should pass after level change, got: %q", output)
	}
}

func TestEnterpriseHandler_WithMasking(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h)

	logger.Info("test", slog.String("password", "secret123"))

	output := read()
	if strings.Contains(output, "secret123") {
		t.Error("password should be masked")
	}
	if !strings.Contains(output, "password") {
		t.Error("password key should still be present")
	}
}

func TestEnterpriseHandler_WithCarrier(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h)

	ctx := core.WithTraceID(context.Background(), "trace-abc-123")
	logger.InfoContext(ctx, "with trace")

	output := read()
	if !strings.Contains(output, "trace-abc-123") {
		t.Errorf("output should contain trace_id: %q", output)
	}
}

func TestEnterpriseHandler_WithStandardFields(t *testing.T) {
	h, cleanup, read := newTestHandler(t,
		WithStandardFields(ServiceFields("my-api", "1.0.0", "production", "host01")...),
	)
	defer cleanup()
	logger := slog.New(h)

	logger.Info("test")

	output := read()
	if !strings.Contains(output, "my-api") {
		t.Errorf("output should contain service name: %q", output)
	}
	if !strings.Contains(output, "1.0.0") {
		t.Errorf("output should contain version: %q", output)
	}
	if !strings.Contains(output, "production") {
		t.Errorf("output should contain environment: %q", output)
	}
}

func TestEnterpriseHandler_WithAttrs(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h).With(slog.String("request_id", "req-123"))

	logger.Info("test")

	output := read()
	if !strings.Contains(output, "req-123") {
		t.Errorf("output should contain request_id from WithAttrs: %q", output)
	}
}

func TestEnterpriseHandler_WithGroup(t *testing.T) {
	h, cleanup, read := newTestHandler(t)
	defer cleanup()
	logger := slog.New(h).WithGroup("module")

	logger.Info("test", slog.String("name", "handler"))

	output := read()
	if !strings.Contains(output, "module.name") {
		t.Errorf("output should contain grouped key 'module.name': %q", output)
	}
}

func TestEnterpriseHandler_Enabled(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithLevel(slog.LevelWarn))
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("INFO should not be enabled at WARN level")
	}
	if !h.Enabled(ctx, slog.LevelWarn) {
		t.Error("WARN should be enabled")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Error("ERROR should be enabled")
	}
}

func TestEnterpriseHandler_WithFilters(t *testing.T) {
	blockFilter := &messageBlocker{blocked: "filtered"}
	h, cleanup, read := newTestHandler(t, WithFilters(blockFilter))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("normal message")
	logger.Info("filtered message")

	output := read()
	if !strings.Contains(output, "normal message") {
		t.Errorf("normal message should pass: %q", output)
	}
	if strings.Contains(output, "filtered message") {
		t.Error("filtered message should be blocked")
	}
}

func TestEnterpriseHandler_Sink(t *testing.T) {
	s := sink.NewConsoleSink()
	h := NewEnterpriseHandler(s)
	if h.Sink() == nil {
		t.Error("Sink() should return the sink")
	}
}

func TestEnterpriseHandler_Metrics(t *testing.T) {
	h := NewEnterpriseHandler(sink.NewConsoleSink())
	if h.Metrics() == nil {
		t.Error("Metrics() should return metrics")
	}
}

func TestServiceFields(t *testing.T) {
	attrs := ServiceFields("api", "1.0", "prod", "host1")
	if len(attrs) != 4 {
		t.Errorf("ServiceFields returned %d attrs, want 4", len(attrs))
	}

	attrs = ServiceFields("", "", "", "")
	if len(attrs) != 0 {
		t.Errorf("ServiceFields with empty values returned %d attrs, want 0", len(attrs))
	}
}

func TestEnterpriseHandler_WithConsoleFormatter(t *testing.T) {
	h, cleanup, read := newTestHandler(t, WithFormatter(NewConsoleFormatter()))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("console format test")

	output := read()
	if !strings.Contains(output, "INFO") {
		t.Errorf("console format should contain 'INFO': %q", output)
	}
}

func TestEnterpriseHandler_WithHooks(t *testing.T) {
	hook := &countingHook{}
	h, cleanup, read := newTestHandler(t, WithHooks(hook))
	defer cleanup()
	logger := slog.New(h)

	logger.Info("hook test")

	output := read()
	if !strings.Contains(output, "hook test") {
		t.Errorf("output should contain message: %q", output)
	}
	if hook.beforeCount != 1 {
		t.Errorf("BeforeWrite should be called once, got %d", hook.beforeCount)
	}
	if hook.afterCount != 1 {
		t.Errorf("AfterWrite should be called once, got %d", hook.afterCount)
	}
}

// --- Helpers ---

type messageBlocker struct {
	blocked string
}

func (f *messageBlocker) Allow(_ context.Context, _ core.Level, msg string) bool {
	return !strings.Contains(msg, f.blocked)
}

type countingHook struct {
	beforeCount int
	afterCount  int
}

func (h *countingHook) BeforeWrite(_ context.Context, r slog.Record) (slog.Record, error) {
	h.beforeCount++
	return r, nil
}

func (h *countingHook) AfterWrite(_ context.Context, _ slog.Record, _ []byte, _ error) {
	h.afterCount++
}

func (h *countingHook) Name() string { return "counting" }
