package external

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

func newTestLogger(t *testing.T) (*logger.Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "test.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	cleanup := func() { log.Close() }
	readFn := func() string {
		data, _ := os.ReadFile(cfg.FilePath)
		return string(data)
	}
	return log, cleanup, readFn
}

func TestNew(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	f := New(log)
	if f == nil {
		t.Fatal("New should not return nil")
	}
}

func TestNew_NilLogger(t *testing.T) {
	f := New(nil)
	if f != nil {
		t.Error("New(nil) should return nil")
	}
}

func TestFacade_LogCall(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogCall(context.Background(), "payment-api", "POST /charge", "POST", 200, 150*time.Millisecond)

	output := read()
	if !strings.Contains(output, "external: service call") {
		t.Error("output should contain 'external: service call'")
	}
	if !strings.Contains(output, "payment-api") {
		t.Error("output should contain service name")
	}
	if !strings.Contains(output, "POST /charge") {
		t.Error("output should contain endpoint")
	}
	if !strings.Contains(output, "200") {
		t.Error("output should contain status")
	}
}

func TestFacade_LogError(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogError(context.Background(), "payment-api", "POST /charge", errors.New("connection refused"))

	output := read()
	if !strings.Contains(output, "external: service error") {
		t.Error("output should contain 'external: service error'")
	}
	if !strings.Contains(output, "connection refused") {
		t.Error("output should contain error message")
	}
}

func TestFacade_LogError_NilErr(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogError(context.Background(), "svc", "endpoint", nil)

	output := read()
	if !strings.Contains(output, "external: service error") {
		t.Error("output should still contain message")
	}
}

func TestFacade_LogTimeout(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogTimeout(context.Background(), "payment-api", "POST /charge", 5*time.Second)

	output := read()
	if !strings.Contains(output, "external: service timeout") {
		t.Error("output should contain 'external: service timeout'")
	}
	if !strings.Contains(output, "5s") {
		t.Error("output should contain timeout duration")
	}
}

func TestFacade_LogRetry(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogRetry(context.Background(), "payment-api", "POST /charge", 2, 3)

	output := read()
	if !strings.Contains(output, "external: service retry") {
		t.Error("output should contain 'external: service retry'")
	}
	if !strings.Contains(output, "payment-api") {
		t.Error("output should contain service name")
	}
}

func TestFacade_LogCircuitBreaker(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogCircuitBreaker(context.Background(), "payment-api", "open")

	output := read()
	if !strings.Contains(output, "circuit breaker") {
		t.Error("output should contain 'circuit breaker'")
	}
	if !strings.Contains(output, "open") {
		t.Error("output should contain state")
	}
}

func TestCallTimer_Stop(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	timer := Start(context.Background(), f, "svc", "GET /data", "GET")
	time.Sleep(10 * time.Millisecond)
	duration := timer.Stop(200)

	if duration < 10*time.Millisecond {
		t.Errorf("duration = %v, should be >= 10ms", duration)
	}

	output := read()
	if !strings.Contains(output, "external: service call") {
		t.Error("output should contain call log")
	}
}

func TestCallTimer_NilFacade(t *testing.T) {
	timer := Start(context.Background(), nil, "svc", "endpoint", "GET")
	duration := timer.Stop(200)
	if duration < 0 {
		t.Error("duration should be >= 0 even with nil facade")
	}
}

func TestCallTimer_NilTimer(t *testing.T) {
	var timer *CallTimer
	duration := timer.Stop(200)
	if duration != 0 {
		t.Error("nil timer Stop should return 0")
	}
}

func TestFacade_NilReceiver(t *testing.T) {
	var f *Facade
	// All methods should be safe on nil receiver.
	f.LogCall(context.Background(), "svc", "endpoint", "GET", 200, time.Millisecond)
	f.LogError(context.Background(), "svc", "endpoint", errors.New("err"))
	f.LogTimeout(context.Background(), "svc", "endpoint", time.Second)
	f.LogRetry(context.Background(), "svc", "endpoint", 1, 3)
	f.LogCircuitBreaker(context.Background(), "svc", "open")
}

func TestFacade_WithExtraAttrs(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogCall(context.Background(), "svc", "endpoint", "GET", 200, time.Millisecond,
		logger.String("request_id", "req-123"),
	)

	output := read()
	if !strings.Contains(output, "req-123") {
		t.Error("output should contain extra attr")
	}
}
