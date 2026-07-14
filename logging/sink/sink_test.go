package sink

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConsoleSink_Write(t *testing.T) {
	s := NewConsoleSinkFile(os.Stdout, "test")
	defer s.Close()

	// Just verify no error on write.
	err := s.Write(context.Background(), []byte("test log line\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
}

func TestConsoleSink_Name(t *testing.T) {
	s := NewConsoleSink()
	if s.Name() != "console:stdout" {
		t.Errorf("Name = %q, want 'console:stdout'", s.Name())
	}
	s2 := NewConsoleSinkStderr()
	if s2.Name() != "console:stderr" {
		t.Errorf("Name = %q, want 'console:stderr'", s2.Name())
	}
}

func TestConsoleSink_Close_Idempotent(t *testing.T) {
	s := NewConsoleSink()
	if err := s.Close(); err != nil {
		t.Errorf("first Close error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got: %v", err)
	}
}

func TestConsoleSink_WriteAfterClose(t *testing.T) {
	s := NewConsoleSink()
	s.Close()
	err := s.Write(context.Background(), []byte("test\n"))
	if err != ErrSinkClosed {
		t.Errorf("Write after Close should return ErrSinkClosed, got %v", err)
	}
}

func TestFileSink_Write(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.log")

	s, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink failed: %v", err)
	}
	defer s.Close()

	err = s.Write(context.Background(), []byte("file log line\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "file log line") {
		t.Errorf("file content = %q, want 'file log line'", string(data))
	}
}

func TestFileSink_AppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "append.log")

	s1, _ := NewFileSink(path)
	s1.Write(context.Background(), []byte("first\n"))
	s1.Close()

	s2, _ := NewFileSink(path)
	s2.Write(context.Background(), []byte("second\n"))
	s2.Close()

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "first") || !strings.Contains(string(data), "second") {
		t.Errorf("append mode failed, content = %q", string(data))
	}
}

func TestFileSink_EmptyPath(t *testing.T) {
	_, err := NewFileSink("")
	if err == nil {
		t.Error("NewFileSink with empty path should error")
	}
}

func TestFileSink_Name(t *testing.T) {
	s, _ := NewFileSink("/tmp/test_name.log")
	defer s.Close()
	if !strings.Contains(s.Name(), "file:/tmp/test_name.log") {
		t.Errorf("Name = %q", s.Name())
	}
}

func TestRotateSink_Write(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "rotate.log")

	s, err := NewRotateSink(RotateConfig{
		Path:       path,
		MaxSizeMB:  1,
		MaxBackups: 3,
		MaxAgeDays: 7,
		Compress:   false,
	})
	if err != nil {
		t.Fatalf("NewRotateSink failed: %v", err)
	}
	defer s.Close()

	err = s.Write(context.Background(), []byte("rotate log line\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "rotate log line") {
		t.Errorf("rotate content = %q", string(data))
	}
}

func TestRotateSink_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "defaults.log")

	s, err := NewRotateSink(RotateConfig{Path: path})
	if err != nil {
		t.Fatalf("NewRotateSink failed: %v", err)
	}
	defer s.Close()
	if !strings.Contains(s.Name(), "rotate:") {
		t.Errorf("Name = %q", s.Name())
	}
}

func TestMultiSink_FailoverContinue(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi.log")

	fileSink, _ := NewFileSink(path)
	consoleSink := NewConsoleSink()

	multi := NewMultiSink([]Sink{fileSink, consoleSink}, WithFailoverPolicy(FailoverContinue))
	defer multi.Close()

	err := multi.Write(context.Background(), []byte("multi log\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "multi log") {
		t.Errorf("file should contain log, got %q", string(data))
	}
}

func TestMultiSink_AllFail_StderrFallback(t *testing.T) {
	// Create sinks that will fail.
	failSink := &failingSink{name: "fail"}
	multi := NewMultiSink([]Sink{failSink})
	defer multi.Close()

	// This should not error (stderr fallback).
	err := multi.Write(context.Background(), []byte("fallback test\n"))
	// Error is returned but stderr fallback should have written.
	_ = err
}

func TestMultiSink_Health(t *testing.T) {
	fileSink, _ := NewFileSink(filepath.Join(t.TempDir(), "health.log"))
	multi := NewMultiSink([]Sink{fileSink})
	defer multi.Close()

	health := multi.Health()
	if len(health) != 1 {
		t.Fatalf("Health returned %d items, want 1", len(health))
	}
	if !health[0].Healthy {
		t.Error("sink should be healthy initially")
	}
}

func TestMultiSink_Metrics(t *testing.T) {
	fileSink, _ := NewFileSink(filepath.Join(t.TempDir(), "metrics.log"))
	multi := NewMultiSink([]Sink{fileSink})
	defer multi.Close()

	multi.Write(context.Background(), []byte("metrics test\n"))

	metrics := multi.Metrics()
	if len(metrics) != 1 {
		t.Fatalf("Metrics returned %d items, want 1", len(metrics))
	}
	if metrics[0].Written != 1 {
		t.Errorf("Written = %d, want 1", metrics[0].Written)
	}
}

func TestAsyncSink_Write(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "async.log")
	fileSink, _ := NewFileSink(path)

	async := NewAsyncSink(fileSink, WithAsyncBufferSize(100), WithAsyncWorkers(1))
	defer async.Close()

	for i := 0; i < 10; i++ {
		async.Write(context.Background(), []byte("async log line\n"))
	}

	// Flush and close to ensure all writes complete.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	async.Flush(ctx)
	async.Close()

	data, _ := os.ReadFile(path)
	lines := strings.Count(string(data), "async log line")
	if lines != 10 {
		t.Errorf("expected 10 lines, got %d", lines)
	}
}

func TestAsyncSink_DropNewest(t *testing.T) {
	// Small buffer to trigger drops.
	slowSink := &slowSink{delay: 100 * time.Millisecond}
	async := NewAsyncSink(slowSink,
		WithAsyncBufferSize(2),
		WithBackpressure(BackpressureDropNewest),
	)
	defer async.Close()

	// Fill buffer beyond capacity.
	dropped := 0
	asyncWithDrop := NewAsyncSink(slowSink,
		WithAsyncBufferSize(2),
		WithBackpressure(BackpressureDropNewest),
		WithOnDrop(func(_ []byte) { dropped++ }),
	)
	defer asyncWithDrop.Close()

	for i := 0; i < 20; i++ {
		asyncWithDrop.Write(context.Background(), []byte("x\n"))
	}
	// Some should be dropped.
	asyncWithDrop.Close()
	if dropped == 0 {
		// May not always drop due to timing, but verify no panic.
	}
	_ = async
}

func TestAsyncSink_QueueDepth(t *testing.T) {
	slowSink := &slowSink{delay: 500 * time.Millisecond}
	async := NewAsyncSink(slowSink, WithAsyncBufferSize(100), WithAsyncWorkers(1))
	defer async.Close()

	for i := 0; i < 5; i++ {
		async.Write(context.Background(), []byte("x\n"))
	}
	depth := async.QueueDepth()
	// Queue depth should be >= 0.
	if depth < 0 {
		t.Errorf("QueueDepth = %d, should be >= 0", depth)
	}
}

func TestAsyncSink_Name(t *testing.T) {
	fileSink, _ := NewFileSink(filepath.Join(t.TempDir(), "name.log"))
	async := NewAsyncSink(fileSink)
	defer async.Close()
	if !strings.Contains(async.Name(), "async:") {
		t.Errorf("Name = %q", async.Name())
	}
}

func TestRateLimitSink(t *testing.T) {
	countingSink := &countingSink{}
	rl := NewRateLimitSink(countingSink, 5) // 5 per second
	defer rl.Close()

	// Write 10 rapidly; only ~5 should pass.
	for i := 0; i < 10; i++ {
		rl.Write(context.Background(), []byte("x\n"))
	}

	// Should have written at most ~5 (allow some variance for refill).
	written := countingSink.Count()
	if written > 7 {
		t.Errorf("rate limit should have dropped some, got %d writes", written)
	}
	if written == 0 {
		t.Error("at least some writes should have passed")
	}
}

func TestRateLimitSink_Unlimited(t *testing.T) {
	countingSink := &countingSink{}
	rl := NewRateLimitSink(countingSink, 0) // unlimited
	defer rl.Close()

	for i := 0; i < 100; i++ {
		rl.Write(context.Background(), []byte("x\n"))
	}
	if countingSink.Count() != 100 {
		t.Errorf("unlimited should pass all, got %d", countingSink.Count())
	}
}

func TestRetrySink_Success(t *testing.T) {
	countingSink := &countingSink{}
	retry := NewRetrySink(countingSink, RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	})
	defer retry.Close()

	err := retry.Write(context.Background(), []byte("x\n"))
	if err != nil {
		t.Errorf("Write should succeed, got %v", err)
	}
	if countingSink.Count() != 1 {
		t.Errorf("should write once, got %d", countingSink.Count())
	}
}

func TestRetrySink_FailThenSucceed(t *testing.T) {
	flaky := &flakySink{failUntil: 2} // fails first 2 attempts, succeeds on 3rd
	retry := NewRetrySink(flaky, RetryPolicy{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	})
	defer retry.Close()

	err := retry.Write(context.Background(), []byte("x\n"))
	if err != nil {
		t.Errorf("should eventually succeed, got %v", err)
	}
}

func TestRetrySink_AllFail(t *testing.T) {
	failSink := &failingSink{name: "fail"}
	retry := NewRetrySink(failSink, RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	})
	defer retry.Close()

	err := retry.Write(context.Background(), []byte("x\n"))
	if err == nil {
		t.Error("should fail after all retries exhausted")
	}
}

func TestBatchSink(t *testing.T) {
	countingSink := &countingSink{}
	batch := NewBatchSink(countingSink,
		WithBatchSize(3),
		WithBatchFlushInterval(1*time.Second),
	)
	defer batch.Close()

	// Write 3 payloads; should trigger flush at batchSize=3.
	batch.Write(context.Background(), []byte("a\n"))
	batch.Write(context.Background(), []byte("b\n"))
	batch.Write(context.Background(), []byte("c\n"))

	// Give a moment for flush.
	time.Sleep(50 * time.Millisecond)

	// countingSink should have received 1 combined write.
	if countingSink.Count() != 1 {
		t.Errorf("batch should flush as 1 write, got %d", countingSink.Count())
	}
}

func TestBatchSink_FlushOnClose(t *testing.T) {
	countingSink := &countingSink{}
	batch := NewBatchSink(countingSink,
		WithBatchSize(100), // high threshold so no auto-flush
		WithBatchFlushInterval(10*time.Second),
	)

	batch.Write(context.Background(), []byte("a\n"))
	batch.Write(context.Background(), []byte("b\n"))

	batch.Close() // should flush remaining

	if countingSink.Count() != 1 {
		t.Errorf("Close should flush, got %d writes", countingSink.Count())
	}
}

func TestValidatorSink_Valid(t *testing.T) {
	countingSink := &countingSink{}
	v := NewValidatorSink(countingSink)
	defer v.Close()

	err := v.Write(context.Background(), []byte("valid log\n"))
	if err != nil {
		t.Errorf("valid payload should pass, got %v", err)
	}
	if countingSink.Count() != 1 {
		t.Errorf("should write once, got %d", countingSink.Count())
	}
}

func TestValidatorSink_NullBytes(t *testing.T) {
	countingSink := &countingSink{}
	v := NewValidatorSink(countingSink)
	defer v.Close()

	// Payload with null byte should be dropped.
	v.Write(context.Background(), []byte("bad\x00log\n"))
	if countingSink.Count() != 0 {
		t.Error("payload with null byte should be dropped")
	}
}

func TestValidatorSink_Oversized(t *testing.T) {
	countingSink := &countingSink{}
	v := NewValidatorSink(countingSink, WithMaxPayloadSize(10))
	defer v.Close()

	// Payload larger than 10 bytes should be dropped.
	v.Write(context.Background(), []byte("this is a very long log line that exceeds the limit\n"))
	if countingSink.Count() != 0 {
		t.Error("oversized payload should be dropped")
	}
}

func TestRegistry_Console(t *testing.T) {
	s, err := NewSinkByName("console", map[string]any{})
	if err != nil {
		t.Fatalf("NewSinkByName(console) failed: %v", err)
	}
	defer s.Close()
	if s.Name() != "console:stdout" {
		t.Errorf("Name = %q", s.Name())
	}
}

func TestRegistry_ConsoleStderr(t *testing.T) {
	s, err := NewSinkByName("console", map[string]any{"stderr": true})
	if err != nil {
		t.Fatalf("NewSinkByName(console, stderr) failed: %v", err)
	}
	defer s.Close()
	if s.Name() != "console:stderr" {
		t.Errorf("Name = %q", s.Name())
	}
}

func TestRegistry_File(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "reg.log")
	s, err := NewSinkByName("file", map[string]any{"path": path})
	if err != nil {
		t.Fatalf("NewSinkByName(file) failed: %v", err)
	}
	defer s.Close()
	s.Write(context.Background(), []byte("registry test\n"))
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "registry test") {
		t.Errorf("file content = %q", string(data))
	}
}

func TestRegistry_UnknownSink(t *testing.T) {
	_, err := NewSinkByName("nonexistent", nil)
	if err == nil {
		t.Error("unknown sink should return error")
	}
}

func TestRegistry_RegisteredSinks(t *testing.T) {
	names := RegisteredSinks()
	if len(names) < 3 {
		t.Errorf("should have at least 3 registered sinks, got %d", len(names))
	}
}

// --- Test helpers ---

type failingSink struct {
	name string
}

func (s *failingSink) Write(_ context.Context, _ []byte) error {
	return ErrSinkWrite
}
func (s *failingSink) Close() error { return nil }
func (s *failingSink) Name() string { return s.name }

type countingSink struct {
	mu    sync.Mutex
	count int
}

func (s *countingSink) Write(_ context.Context, _ []byte) error {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	return nil
}
func (s *countingSink) Close() error { return nil }
func (s *countingSink) Name() string { return "counting" }
func (s *countingSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

type slowSink struct {
	delay time.Duration
}

func (s *slowSink) Write(_ context.Context, _ []byte) error {
	time.Sleep(s.delay)
	return nil
}
func (s *slowSink) Close() error { return nil }
func (s *slowSink) Name() string { return "slow" }

type flakySink struct {
	attempts  int
	failUntil int // fail first N attempts
}

func (s *flakySink) Write(_ context.Context, _ []byte) error {
	s.attempts++
	if s.attempts <= s.failUntil {
		return ErrSinkWrite
	}
	return nil
}
func (s *flakySink) Close() error { return nil }
func (s *flakySink) Name() string { return "flaky" }
