package handler

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

func BenchmarkJSONFormatter_Format(b *testing.B) {
	f := NewJSONFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "benchmark message", 0)
	record.AddAttrs(
		slog.String("trace_id", "abc-123"),
		slog.String("user_id", "user-456"),
		slog.Int("count", 42),
		slog.String("password", "secret"),
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Format(record)
	}
}

func BenchmarkConsoleFormatter_Format(b *testing.B) {
	f := NewConsoleFormatter()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "benchmark message", 0)
	record.AddAttrs(
		slog.String("trace_id", "abc-123"),
		slog.String("user_id", "user-456"),
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f.Format(record)
	}
}

func BenchmarkEnterpriseHandler_Handle(b *testing.B) {
	tmpDir := b.TempDir()
	fs, _ := sink.NewFileSink(filepath.Join(tmpDir, "bench.log"))
	defer fs.Close()
	h := NewEnterpriseHandler(fs,
		WithLevel(slog.LevelInfo),
		WithCaller(false),
		WithStacktrace(false),
	)
	logger := slog.New(h)
	ctx := core.WithTraceID(context.Background(), "trace-bench")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "benchmark",
			slog.String("key", "value"),
			slog.Int("count", i),
		)
	}
}

func BenchmarkEnterpriseHandler_WithMasking(b *testing.B) {
	tmpDir := b.TempDir()
	fs, _ := sink.NewFileSink(filepath.Join(tmpDir, "bench_mask.log"))
	defer fs.Close()
	h := NewEnterpriseHandler(fs,
		WithLevel(slog.LevelInfo),
		WithCaller(false),
		WithStacktrace(false),
	)
	logger := slog.New(h)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark",
			slog.String("password", "secret123"),
			slog.String("token", "eyJhbGci..."),
			slog.String("normal", "value"),
		)
	}
}

func BenchmarkEnterpriseHandler_WithCarrier(b *testing.B) {
	tmpDir := b.TempDir()
	fs, _ := sink.NewFileSink(filepath.Join(tmpDir, "bench_ctx.log"))
	defer fs.Close()
	h := NewEnterpriseHandler(fs,
		WithLevel(slog.LevelInfo),
		WithCaller(false),
		WithStacktrace(false),
	)
	logger := slog.New(h)
	ctx := context.Background()
	ctx = core.WithTraceID(ctx, "trace-123")
	ctx = core.WithRequestID(ctx, "req-456")
	ctx = core.WithUser(ctx, "user-1", "john", "admin")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "benchmark")
	}
}

func BenchmarkEnterpriseHandler_Enabled(b *testing.B) {
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithLevel(slog.LevelInfo))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Enabled(ctx, slog.LevelInfo)
	}
}

func BenchmarkEnterpriseHandler_DynamicLevel_Enabled(b *testing.B) {
	dl := core.NewDynamicLevel(core.LevelInfo)
	h := NewEnterpriseHandler(sink.NewConsoleSink(), WithDynamicLevel(dl))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Enabled(ctx, slog.LevelInfo)
	}
}

func BenchmarkServiceFields(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ServiceFields("api", "1.0.0", "production", "host01")
	}
}

// Ensure os import is used.
var _ = os.Stdout
