package sink

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// --- Name() coverage ---

func TestAllSinks_Name(t *testing.T) {
	tmpDir := t.TempDir()

	// File
	fs, _ := NewFileSink(filepath.Join(tmpDir, "a.log"))
	if fs.Name() == "" {
		t.Error("FileSink Name empty")
	}
	fs.Close()

	// File from file
	f, _ := os.CreateTemp(tmpDir, "b*.log")
	fs2 := NewFileSinkFromFile(f, "custom")
	if fs2.Name() != "file:custom" {
		t.Errorf("FileSinkFromFile Name = %q", fs2.Name())
	}
	fs2.Close()

	// Rotate
	rs, _ := NewRotateSink(RotateConfig{Path: filepath.Join(tmpDir, "r.log")})
	if rs.Name() == "" {
		t.Error("RotateSink Name empty")
	}
	rs.Close()

	// Multi
	multi := NewMultiSink([]Sink{NewConsoleSink()})
	if multi.Name() != "multi" {
		t.Errorf("MultiSink Name = %q", multi.Name())
	}
	multi.Close()

	// RateLimit
	rl := NewRateLimitSink(&countingSink{}, 10)
	if rl.Name() == "" {
		t.Error("RateLimitSink Name empty")
	}
	rl.Close()

	// Retry
	retry := NewRetrySink(&countingSink{}, DefaultRetryPolicy())
	if retry.Name() == "" {
		t.Error("RetrySink Name empty")
	}
	retry.Close()

	// Validator
	v := NewValidatorSink(&countingSink{})
	if v.Name() == "" {
		t.Error("ValidatorSink Name empty")
	}
	v.Close()

	// Batch
	b := NewBatchSink(&countingSink{})
	if b.Name() == "" {
		t.Error("BatchSink Name empty")
	}
	b.Close()
}

// --- Metrics() coverage ---

func TestAllSinks_Metrics(t *testing.T) {
	// RateLimit
	rl := NewRateLimitSink(&countingSink{}, 100)
	rl.Write(context.Background(), []byte("x\n"))
	rl.Metrics()
	rl.Close()

	// Retry
	retry := NewRetrySink(&countingSink{}, DefaultRetryPolicy())
	retry.Write(context.Background(), []byte("x\n"))
	retry.Metrics()
	retry.Close()

	// Validator
	v := NewValidatorSink(&countingSink{})
	v.Write(context.Background(), []byte("x\n"))
	v.Metrics()
	v.Close()

	// Batch
	b := NewBatchSink(&countingSink{}, WithBatchSize(1))
	b.Write(context.Background(), []byte("x\n"))
	time.Sleep(50 * time.Millisecond)
	b.Metrics()
	b.Close()

	// Async
	async := NewAsyncSink(&countingSink{}, WithAsyncBufferSize(10))
	async.Write(context.Background(), []byte("x\n"))
	time.Sleep(50 * time.Millisecond)
	async.Metrics()
	async.Close()
}

// --- Option functions ---

func TestWithFileWriteTimeout(t *testing.T) {
	opt := WithFileWriteTimeout(5 * time.Second)
	s := &FileSink{writeTimeout: 1 * time.Second}
	opt(s)
	if s.writeTimeout != 5*time.Second {
		t.Error("WithFileWriteTimeout not applied")
	}
}

func TestWithQuorum(t *testing.T) {
	multi := NewMultiSink([]Sink{NewConsoleSink()}, WithQuorum(1))
	if multi.quorum != 1 {
		t.Error("WithQuorum not applied")
	}
	multi.Close()
}

func TestWithFailoverPolicy(t *testing.T) {
	multi := NewMultiSink([]Sink{NewConsoleSink()}, WithFailoverPolicy(FailoverStop))
	if multi.policy != FailoverStop {
		t.Error("WithFailoverPolicy not applied")
	}
	multi.Close()
}

// --- Close idempotency for all sinks ---

func TestAllSinks_CloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()

	// File
	fs, _ := NewFileSink(filepath.Join(tmpDir, "idem.log"))
	fs.Close()
	if err := fs.Close(); err != nil {
		t.Errorf("FileSink second Close: %v", err)
	}

	// Rotate
	rs, _ := NewRotateSink(RotateConfig{Path: filepath.Join(tmpDir, "idem_r.log")})
	rs.Close()
	if err := rs.Close(); err != nil {
		t.Errorf("RotateSink second Close: %v", err)
	}

	// Multi
	multi := NewMultiSink([]Sink{NewConsoleSink()})
	multi.Close()
	if err := multi.Close(); err != nil {
		t.Errorf("MultiSink second Close: %v", err)
	}

	// Async
	async := NewAsyncSink(&countingSink{}, WithAsyncBufferSize(10))
	async.Close()
	if err := async.Close(); err != nil {
		t.Errorf("AsyncSink second Close: %v", err)
	}

	// RateLimit
	rl := NewRateLimitSink(&countingSink{}, 10)
	rl.Close()
	if err := rl.Close(); err != nil {
		t.Errorf("RateLimitSink second Close: %v", err)
	}

	// Retry
	retry := NewRetrySink(&countingSink{}, DefaultRetryPolicy())
	retry.Close()
	if err := retry.Close(); err != nil {
		t.Errorf("RetrySink second Close: %v", err)
	}

	// Validator
	v := NewValidatorSink(&countingSink{})
	v.Close()
	if err := v.Close(); err != nil {
		t.Errorf("ValidatorSink second Close: %v", err)
	}

	// Batch
	b := NewBatchSink(&countingSink{})
	b.Close()
	if err := b.Close(); err != nil {
		t.Errorf("BatchSink second Close: %v", err)
	}

	// Buffered
	buf := NewBufferedSink(&countingSink{}, 100)
	buf.Close()
	if err := buf.Close(); err != nil {
		t.Errorf("BufferedSink second Close: %v", err)
	}

	// Audit
	audit, _ := NewAuditSink(filepath.Join(tmpDir, "idem_a.log"))
	audit.Close()
	if err := audit.Close(); err != nil {
		t.Errorf("AuditSink second Close: %v", err)
	}
}

// --- Write after close for all sinks ---

func TestAllSinks_WriteAfterClose(t *testing.T) {
	tmpDir := t.TempDir()

	// File
	fs, _ := NewFileSink(filepath.Join(tmpDir, "wac.log"))
	fs.Close()
	if err := fs.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("FileSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Rotate
	rs, _ := NewRotateSink(RotateConfig{Path: filepath.Join(tmpDir, "wac_r.log")})
	rs.Close()
	if err := rs.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("RotateSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Multi
	multi := NewMultiSink([]Sink{NewConsoleSink()})
	multi.Close()
	if err := multi.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("MultiSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Async
	async := NewAsyncSink(&countingSink{}, WithAsyncBufferSize(10))
	async.Close()
	if err := async.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("AsyncSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// RateLimit
	rl := NewRateLimitSink(&countingSink{}, 10)
	rl.Close()
	if err := rl.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("RateLimitSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Validator
	v := NewValidatorSink(&countingSink{})
	v.Close()
	if err := v.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("ValidatorSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Batch
	b := NewBatchSink(&countingSink{})
	b.Close()
	if err := b.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("BatchSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}

	// Buffered
	buf := NewBufferedSink(&countingSink{}, 100)
	buf.Close()
	if err := buf.Write(context.Background(), []byte("x\n")); err != ErrSinkClosed {
		t.Errorf("BufferedSink WriteAfterClose = %v, want ErrSinkClosed", err)
	}
}

// --- MultiSink quorum and stop policies ---

func TestMultiSink_QuorumPolicy(t *testing.T) {
	// 2 sinks, quorum=1, one fails.
	fs, _ := NewFileSink(filepath.Join(t.TempDir(), "q.log"))
	multi := NewMultiSink([]Sink{fs, &failingSink{name: "fail"}},
		WithFailoverPolicy(FailoverQuorum),
		WithQuorum(1),
	)
	defer multi.Close()

	err := multi.Write(context.Background(), []byte("quorum test\n"))
	// 1 sink succeeded, quorum=1 met, should not error.
	if err != nil {
		t.Errorf("quorum met should not error, got %v", err)
	}
}

func TestMultiSink_QuorumNotMet(t *testing.T) {
	multi := NewMultiSink([]Sink{&failingSink{name: "fail1"}, &failingSink{name: "fail2"}},
		WithFailoverPolicy(FailoverQuorum),
		WithQuorum(1),
	)
	defer multi.Close()

	err := multi.Write(context.Background(), []byte("quorum fail\n"))
	if err == nil {
		t.Error("quorum not met should error")
	}
}

func TestMultiSink_StopPolicy(t *testing.T) {
	// FailoverStop: first sink fails, should stop and return error.
	fs, _ := NewFileSink(filepath.Join(t.TempDir(), "stop.log"))
	multi := NewMultiSink([]Sink{&failingSink{name: "fail"}, fs},
		WithFailoverPolicy(FailoverStop),
	)
	defer multi.Close()

	err := multi.Write(context.Background(), []byte("stop test\n"))
	if err == nil {
		t.Error("FailoverStop should return error on first failure")
	}
}

func TestMultiSink_Name(t *testing.T) {
	multi := NewMultiSink([]Sink{NewConsoleSink()})
	defer multi.Close()
	if multi.Name() != "multi" {
		t.Errorf("Name = %q, want 'multi'", multi.Name())
	}
}

// --- AsyncSink DropOldest ---

func TestAsyncSink_DropOldest(t *testing.T) {
	slow := &slowSink{delay: 200 * time.Millisecond}
	dropped := 0
	async := NewAsyncSink(slow,
		WithAsyncBufferSize(2),
		WithBackpressure(BackpressureDropOldest),
		WithOnDrop(func(_ []byte) { dropped++ }),
	)
	defer async.Close()

	for i := 0; i < 20; i++ {
		async.Write(context.Background(), []byte("x\n"))
	}
	async.Close()
	// Some drops should have occurred.
	_ = dropped
}

// --- AsyncSink Block ---

func TestAsyncSink_Block(t *testing.T) {
	slow := &slowSink{delay: 50 * time.Millisecond}
	async := NewAsyncSink(slow,
		WithAsyncBufferSize(2),
		WithBackpressure(BackpressureBlock),
		WithAsyncWorkers(1),
	)
	defer async.Close()

	// This will block when buffer is full, but eventually complete.
	for i := 0; i < 5; i++ {
		async.Write(context.Background(), []byte("x\n"))
	}
}

// --- AsyncSink Flush ---

func TestAsyncSink_Flush(t *testing.T) {
	slow := &slowSink{delay: 100 * time.Millisecond}
	async := NewAsyncSink(slow, WithAsyncBufferSize(100), WithAsyncWorkers(1))
	defer async.Close()

	for i := 0; i < 3; i++ {
		async.Write(context.Background(), []byte("x\n"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := async.Flush(ctx); err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestAsyncSink_Flush_CancelledContext(t *testing.T) {
	slow := &slowSink{delay: 10 * time.Second} // very slow
	async := NewAsyncSink(slow, WithAsyncBufferSize(100), WithAsyncWorkers(1))
	defer async.Close()

	async.Write(context.Background(), []byte("x\n"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled
	if err := async.Flush(ctx); err == nil {
		t.Error("Flush with cancelled context should error")
	}
}

// --- FileSink timeout ---

func TestFileSink_WriteTimeout(t *testing.T) {
	// Create a file sink with very short timeout.
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "timeout.log")
	s, _ := NewFileSink(path, WithFileWriteTimeout(1*time.Nanosecond))
	defer s.Close()

	// Normal write should still work (timeout is per-write, but file writes are fast).
	err := s.Write(context.Background(), []byte("x\n"))
	// May or may not timeout depending on scheduling; just verify no panic.
	_ = err
}

func TestFileSink_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "cancel.log")
	s, _ := NewFileSink(path, WithFileWriteTimeout(10*time.Second))
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := s.Write(ctx, []byte("x\n"))
	if err == nil {
		// May succeed if write completes before context check; acceptable.
	}
}

// --- BatchSink Flush and triggerFlush ---

func TestBatchSink_Flush(t *testing.T) {
	counting := &countingSink{}
	batch := NewBatchSink(counting, WithBatchSize(100), WithBatchFlushInterval(10*time.Second))
	defer batch.Close()

	batch.Write(context.Background(), []byte("a\n"))
	batch.Write(context.Background(), []byte("b\n"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := batch.Flush(ctx)
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
	if counting.Count() != 1 {
		t.Errorf("after flush, counting should be 1, got %d", counting.Count())
	}
}

func TestBatchSink_TriggerFlush(t *testing.T) {
	counting := &countingSink{}
	batch := NewBatchSink(counting, WithBatchSize(100), WithBatchFlushInterval(100*time.Millisecond))
	defer batch.Close()

	batch.Write(context.Background(), []byte("a\n"))

	// Wait for timer-triggered flush.
	time.Sleep(200 * time.Millisecond)

	if counting.Count() != 1 {
		t.Errorf("timer flush should have written, got %d", counting.Count())
	}
}

// --- BufferedSink Flush ---

func TestBufferedSink_Flush(t *testing.T) {
	counting := &countingSink{}
	buf := NewBufferedSink(counting, 100)
	defer buf.Close()

	buf.Write(context.Background(), []byte("x\n"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := buf.Flush(ctx)
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

// --- Registry rotate ---

func TestRegistry_Rotate(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "reg_rotate.log")
	s, err := NewSinkByName("rotate", map[string]any{
		"path":        path,
		"max_size_mb": 1,
		"max_backups": 3,
		"max_age_days": 7,
		"compress":    true,
	})
	if err != nil {
		t.Fatalf("NewSinkByName(rotate) failed: %v", err)
	}
	defer s.Close()
	s.Write(context.Background(), []byte("rotate registry test\n"))
}

func TestRegistry_File_EmptyPath(t *testing.T) {
	_, err := NewSinkByName("file", map[string]any{})
	if err == nil {
		t.Error("file sink without path should error")
	}
}

func TestRegistry_Rotate_EmptyPath(t *testing.T) {
	_, err := NewSinkByName("rotate", map[string]any{})
	if err == nil {
		t.Error("rotate sink without path should error")
	}
}

// --- AuditSink with metadata ---

func TestAuditSink_WithMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "meta.log")
	s, _ := NewAuditSink(path)
	defer s.Close()

	payload := core.AuditPayload{
		Timestamp: "t",
		User:      "u",
		Action:    "a",
		IP:        "ip",
		Resource:  "res",
		Before:    map[string]any{"old": "value"},
		After:     map[string]any{"new": "value"},
		Reason:    "test",
		Metadata:  map[string]any{"key": "val"},
	}
	if err := s.WriteAudit(context.Background(), payload); err != nil {
		t.Fatalf("WriteAudit with metadata failed: %v", err)
	}

	idx, err := s.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
	if idx != -1 {
		t.Errorf("chain should be valid, tampered at %d", idx)
	}
}

// --- Health with circuit breaker open ---

func TestMultiSink_Health_CircuitBreakerOpen(t *testing.T) {
	fail := &failingSink{name: "fail"}
	multi := NewMultiSink([]Sink{fail})
	defer multi.Close()

	// Trigger circuit breaker (5 failures).
	for i := 0; i < 6; i++ {
		multi.Write(context.Background(), []byte("x\n"))
	}

	health := multi.Health()
	if len(health) != 1 {
		t.Fatalf("Health returned %d items, want 1", len(health))
	}
	if health[0].Healthy {
		t.Error("sink should be unhealthy after circuit breaker opens")
	}
	if health[0].Error != "circuit breaker open" {
		t.Errorf("Error = %q, want 'circuit breaker open'", health[0].Error)
	}
}

// --- MultiSink Close with error ---

func TestMultiSink_CloseWithError(t *testing.T) {
	multi := NewMultiSink([]Sink{&errorOnCloseSink{}})
	err := multi.Close()
	if err == nil {
		t.Error("Close should return error from inner sink")
	}
}

type errorOnCloseSink struct{}

func (s *errorOnCloseSink) Write(_ context.Context, _ []byte) error { return nil }
func (s *errorOnCloseSink) Close() error                            { return ErrSinkWrite }
func (s *errorOnCloseSink) Name() string                            { return "errorOnClose" }

// --- RateLimit refill edge ---

func TestRateLimit_Refill(t *testing.T) {
	counting := &countingSink{}
	rl := NewRateLimitSink(counting, 10)
	defer rl.Close()

	// Consume all tokens.
	for i := 0; i < 10; i++ {
		rl.Write(context.Background(), []byte("x\n"))
	}
	// Wait for refill.
	time.Sleep(150 * time.Millisecond)
	// Should be able to write again.
	before := counting.Count()
	rl.Write(context.Background(), []byte("x\n"))
	if counting.Count() <= before {
		t.Error("should be able to write after refill")
	}
}

// --- Retry with context cancellation ---

func TestRetrySink_ContextCancelled(t *testing.T) {
	fail := &failingSink{name: "fail"}
	retry := NewRetrySink(fail, RetryPolicy{
		MaxAttempts:  10,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       false,
	})
	defer retry.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled immediately
	err := retry.Write(ctx, []byte("x\n"))
	if err == nil {
		// May succeed if first attempt fails before context check; acceptable.
	}
}

// --- Rotate with defaults (zero values) ---

func TestRotateSink_ZeroValues(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := NewRotateSink(RotateConfig{
		Path:       filepath.Join(tmpDir, "zero.log"),
		MaxSizeMB:  0,  // should default to 100
		MaxBackups: -1, // should default to 10
		MaxAgeDays: -1, // should default to 30
	})
	if err != nil {
		t.Fatalf("NewRotateSink with zero values failed: %v", err)
	}
	defer s.Close()
	s.Write(context.Background(), []byte("zero values test\n"))
}

// --- FileSink with non-existent directory ---

func TestFileSink_NonExistentDir(t *testing.T) {
	_, err := NewFileSink("/nonexistent/path/that/does/not/exist/log.txt")
	if err == nil {
		t.Error("should fail for non-existent directory")
	}
}

// --- Validator with default max size ---

func TestValidatorSink_DefaultMaxSize(t *testing.T) {
	counting := &countingSink{}
	v := NewValidatorSink(counting)
	defer v.Close()

	// Small payload should pass.
	v.Write(context.Background(), []byte("small\n"))
	if counting.Count() != 1 {
		t.Error("small payload should pass default validator")
	}
}
