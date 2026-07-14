package helper

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

func TestHTTPRequestAttrs(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users?page=1", nil)
	req.Host = "example.com"
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "https://google.com")

	attrs := HTTPRequestAttrs(req)
	if len(attrs) < 4 {
		t.Errorf("expected at least 4 attrs, got %d", len(attrs))
	}

	// Verify specific attrs.
	found := map[string]string{}
	for _, attr := range attrs {
		found[attr.Key] = attr.Value.String()
	}
	if found["method"] != "GET" {
		t.Errorf("method = %q", found["method"])
	}
	if found["path"] != "/api/users" {
		t.Errorf("path = %q", found["path"])
	}
	if found["user_agent"] != "test-agent" {
		t.Errorf("user_agent = %q", found["user_agent"])
	}
}

func TestHTTPRequestAttrs_WithContentLength(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/users", strings.NewReader("body"))
	req.ContentLength = 4
	attrs := HTTPRequestAttrs(req)
	found := false
	for _, attr := range attrs {
		if attr.Key == "content_length" {
			found = true
		}
	}
	if !found {
		t.Error("content_length should be present when > 0")
	}
}

func TestHTTPResponseAttrs(t *testing.T) {
	attrs := HTTPResponseAttrs(200, 1024, 50*time.Millisecond)
	if len(attrs) != 3 {
		t.Errorf("expected 3 attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "status_code" {
		t.Errorf("first attr should be status_code")
	}
}

func TestClientIP_Direct(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	ip := ClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("ClientIP = %q, want '192.168.1.1'", ip)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	req.RemoteAddr = "192.168.1.1:12345"
	ip := ClientIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("ClientIP = %q, want '10.0.0.1'", ip)
	}
}

func TestClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.99")
	req.RemoteAddr = "192.168.1.1:12345"
	ip := ClientIP(req)
	if ip != "10.0.0.99" {
		t.Errorf("ClientIP = %q, want '10.0.0.99'", ip)
	}
}

func TestClientIP_NoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1"
	ip := ClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("ClientIP = %q, want '192.168.1.1'", ip)
	}
}

func TestStatusLevel(t *testing.T) {
	tests := []struct {
		code  int
		level core.Level
	}{
		{200, core.LevelInfo},
		{301, core.LevelInfo},
		{404, core.LevelWarn},
		{500, core.LevelError},
		{503, core.LevelError},
	}
	for _, tt := range tests {
		if got := StatusLevel(tt.code); got != tt.level {
			t.Errorf("StatusLevel(%d) = %s, want %s", tt.code, core.LevelLabel(got), core.LevelLabel(tt.level))
		}
	}
}

func TestErrorAttrs(t *testing.T) {
	err := errors.New("test error")
	attrs := ErrorAttrs(err)
	if len(attrs) < 1 {
		t.Fatal("should have at least 1 attr")
	}
	if attrs[0].Key != "error" {
		t.Errorf("first attr key = %q, want 'error'", attrs[0].Key)
	}
	if attrs[0].Value.String() != "test error" {
		t.Errorf("error value = %q", attrs[0].Value.String())
	}
}

func TestErrorAttrs_Nil(t *testing.T) {
	attrs := ErrorAttrs(nil)
	if attrs != nil {
		t.Error("ErrorAttrs(nil) should return nil")
	}
}

func TestErrorAttrs_CustomType(t *testing.T) {
	err := &customError{msg: "custom"}
	attrs := ErrorAttrs(err)
	found := false
	for _, attr := range attrs {
		if attr.Key == "error_type" {
			found = true
		}
	}
	if !found {
		t.Error("custom error type should include error_type")
	}
}

func TestErrorWithStack(t *testing.T) {
	err := errors.New("test")
	attrs := ErrorWithStack(err)
	foundStack := false
	for _, attr := range attrs {
		if attr.Key == "stacktrace" {
			foundStack = true
		}
	}
	if !foundStack {
		t.Error("ErrorWithStack should include stacktrace")
	}
}

func TestErrorWithStack_Nil(t *testing.T) {
	attrs := ErrorWithStack(nil)
	if attrs != nil {
		t.Error("ErrorWithStack(nil) should return nil")
	}
}

func TestTimer(t *testing.T) {
	ctx := context.Background()
	timer := StartTimer(ctx, nil, "test operation")
	time.Sleep(10 * time.Millisecond)
	elapsed := timer.Stop()
	if elapsed < 10*time.Millisecond {
		t.Errorf("elapsed = %v, should be >= 10ms", elapsed)
	}
}

func TestTimer_DoubleStop(t *testing.T) {
	timer := StartTimer(context.Background(), nil, "test")
	timer.Stop()
	elapsed := timer.Stop()
	if elapsed != 0 {
		t.Errorf("double stop should return 0, got %v", elapsed)
	}
}

func TestTimer_Elapsed(t *testing.T) {
	timer := StartTimer(context.Background(), nil, "test")
	time.Sleep(5 * time.Millisecond)
	elapsed := timer.Elapsed()
	if elapsed < 5*time.Millisecond {
		t.Errorf("elapsed = %v, should be >= 5ms", elapsed)
	}
	timer.Stop()
}

func TestTimer_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "timer.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	defer log.Close()

	timer := StartTimer(context.Background(), log.SLogger(), "timed operation")
	timer.Stop()

	data, _ := os.ReadFile(cfg.FilePath)
	if !strings.Contains(string(data), "timed operation") {
		t.Error("output should contain timer message")
	}
	if !strings.Contains(string(data), "duration") {
		t.Error("output should contain duration")
	}
}

func TestStringAttrs(t *testing.T) {
	attrs := StringAttrs("a", "1", "b", "2")
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
	if attrs[0].Key != "a" || attrs[0].Value.String() != "1" {
		t.Errorf("first attr = %v", attrs[0])
	}
}

func TestStringAttrs_OddArgs(t *testing.T) {
	attrs := StringAttrs("a", "1", "b")
	if attrs != nil {
		t.Error("odd args should return nil")
	}
}

func TestIntAttrs(t *testing.T) {
	attrs := IntAttrs(0, 10, 1, 20)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}
}

func TestIntAttrs_OddArgs(t *testing.T) {
	attrs := IntAttrs(0, 10, 1)
	if attrs != nil {
		t.Error("odd args should return nil")
	}
}

func TestRedactHeaderValue_Sensitive(t *testing.T) {
	result := RedactHeaderValue("Authorization", "Bearer abc123")
	if result != "***REDACTED***" {
		t.Errorf("Authorization should be redacted, got %q", result)
	}
}

func TestRedactHeaderValue_Normal(t *testing.T) {
	result := RedactHeaderValue("Content-Type", "application/json")
	if result != "application/json" {
		t.Errorf("Content-Type should not be redacted, got %q", result)
	}
}

func TestSafeHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-API-Key", "key123")

	safe := SafeHeaders(headers)
	if safe["Authorization"] != "***REDACTED***" {
		t.Error("Authorization should be redacted")
	}
	if safe["Content-Type"] != "application/json" {
		t.Error("Content-Type should not be redacted")
	}
	if safe["X-Api-Key"] != "***REDACTED***" {
		t.Error("X-API-Key should be redacted")
	}
}

func TestCallerInfo(t *testing.T) {
	file, line, fn := CallerInfo(0)
	if file == "" {
		t.Error("file should not be empty")
	}
	if line == 0 {
		t.Error("line should not be 0")
	}
	if fn == "" {
		t.Error("function should not be empty")
	}
}

func TestCallerAttr(t *testing.T) {
	attr := CallerAttr(0)
	if attr.Key != "caller" {
		t.Errorf("key = %q, want 'caller'", attr.Key)
	}
	if attr.Value.String() == "" {
		t.Error("caller value should not be empty")
	}
}

func TestContextAttrs_WithTraceID(t *testing.T) {
	ctx := core.WithTraceID(context.Background(), "trace-123")
	attrs := ContextAttrs(ctx)
	found := false
	for _, attr := range attrs {
		if attr.Key == "trace_id" && attr.Value.String() == "trace-123" {
			found = true
		}
	}
	if !found {
		t.Error("should contain trace_id")
	}
}

func TestContextAttrs_Empty(t *testing.T) {
	attrs := ContextAttrs(context.Background())
	if attrs != nil {
		t.Error("empty context should return nil attrs")
	}
}

func TestContextAttrs_Nil(t *testing.T) {
	attrs := ContextAttrs(nil)
	if attrs != nil {
		t.Error("nil context should return nil attrs")
	}
}

func TestMergeAttrs(t *testing.T) {
	a := []slog.Attr{slog.String("a", "1")}
	b := []slog.Attr{slog.String("b", "2"), slog.String("c", "3")}
	merged := MergeAttrs(a, b)
	if len(merged) != 3 {
		t.Errorf("merged should have 3 attrs, got %d", len(merged))
	}
}

func TestMergeAttrs_Empty(t *testing.T) {
	merged := MergeAttrs()
	if len(merged) != 0 {
		t.Error("empty merge should return empty slice")
	}
}

// --- helpers ---

type customError struct {
	msg string
}

func (e *customError) Error() string { return e.msg }
