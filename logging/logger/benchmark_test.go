package logger

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
)

func BenchmarkLogger_Info(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "bench.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, _ := New(cfg)
	defer log.Close()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info(ctx, "benchmark message", String("key", "value"))
	}
}

func BenchmarkLogger_InfoWithCarrier(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "bench_ctx.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, _ := New(cfg)
	defer log.Close()
	ctx := core.WithTraceID(context.Background(), "trace-bench")
	ctx = core.WithUser(ctx, "user-1", "john", "admin")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info(ctx, "benchmark with carrier")
	}
}

func BenchmarkLogger_InfoWithMasking(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "bench_mask.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, _ := New(cfg)
	defer log.Close()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info(ctx, "benchmark",
			String("password", "secret123"),
			String("token", "eyJhbGci..."),
		)
	}
}

func BenchmarkLogger_With(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "bench_with.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, _ := New(cfg)
	defer log.Close()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reqLog := log.With(String("request_id", fmt.Sprintf("req-%d", i)))
		reqLog.Info(ctx, "with benchmark")
	}
}

func BenchmarkLogger_Enabled(b *testing.B) {
	tmpDir := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "bench_enabled.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

	log, _ := New(cfg)
	defer log.Close()
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// DEBUG at INFO level — should be filtered quickly.
		log.Debug(ctx, "filtered")
	}
}

func BenchmarkLogger_AttrConstructors(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = String("key", "value")
		_ = Int("n", 42)
		_ = Bool("b", true)
		_ = Error(fmt.Errorf("err"))
	}
}

// Ensure slog import is used.
var _ = slog.LevelInfo
