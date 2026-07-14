package sink

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

func BenchmarkConsoleSink_Write(b *testing.B) {
	s := NewConsoleSinkFile(os.Stderr, "bench")
	defer s.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark test","trace_id":"abc123"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Write(context.Background(), payload)
	}
}

func BenchmarkFileSink_Write(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "bench.log")
	s, _ := NewFileSink(path)
	defer s.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark test","trace_id":"abc123"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Write(context.Background(), payload)
	}
}

func BenchmarkMultiSink_2Sinks(b *testing.B) {
	tmpDir := b.TempDir()
	fs, _ := NewFileSink(filepath.Join(tmpDir, "bench_multi.log"))
	defer fs.Close()
	console := NewConsoleSinkFile(os.Stderr, "bench")
	defer console.Close()
	multi := NewMultiSink([]Sink{fs, console})
	defer multi.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		multi.Write(context.Background(), payload)
	}
}

func BenchmarkAsyncSink_Write(b *testing.B) {
	tmpDir := b.TempDir()
	fs, _ := NewFileSink(filepath.Join(tmpDir, "bench_async.log"))
	async := NewAsyncSink(fs, WithAsyncBufferSize(4096), WithAsyncWorkers(2))
	defer async.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		async.Write(context.Background(), payload)
	}
}

func BenchmarkRateLimitSink_Write(b *testing.B) {
	rl := NewRateLimitSink(&countingSink{}, 0) // unlimited
	defer rl.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Write(context.Background(), payload)
	}
}

func BenchmarkValidatorSink_Write(b *testing.B) {
	v := NewValidatorSink(&countingSink{})
	defer v.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Write(context.Background(), payload)
	}
}

func BenchmarkAuditSink_Write(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "bench_audit.log")
	s, _ := NewAuditSink(path)
	defer s.Close()
	payload := core.AuditPayload{
		Timestamp: "2026-01-01T00:00:00Z",
		User:      "user-123",
		Action:    "user.update",
		IP:        "10.0.0.1",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.WriteAudit(context.Background(), payload)
	}
}

func BenchmarkBatchSink_Write(b *testing.B) {
	batch := NewBatchSink(&countingSink{}, WithBatchSize(1000), WithBatchFlushInterval(999*time.Second))
	defer batch.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch.Write(context.Background(), payload)
	}
}

func BenchmarkRetrySink_Success(b *testing.B) {
	retry := NewRetrySink(&countingSink{}, RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Nanosecond,
		MaxDelay:     1 * time.Nanosecond,
		Multiplier:   1.0,
		Jitter:       false,
	})
	defer retry.Close()
	payload := []byte(`{"level":"INFO","msg":"benchmark"}` + "\n")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retry.Write(context.Background(), payload)
	}
}

func BenchmarkCircuitBreaker_Allow(b *testing.B) {
	cb := newCircuitBreaker(5, 30*time.Second)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.allow()
	}
}

func BenchmarkSinkMetrics_RecordWrite(b *testing.B) {
	m := NewSinkMetrics()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordWrite(100, 1000)
	}
}

func BenchmarkSinkMetrics_Snapshot(b *testing.B) {
	m := NewSinkMetrics()
	for i := 0; i < 1000; i++ {
		m.RecordWrite(100, 1000)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Snapshot()
	}
}
