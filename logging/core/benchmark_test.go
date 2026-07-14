package core

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// Benchmark MaskEngine.MaskAttrs with no sensitive fields (hot path).
func BenchmarkMaskAttrs_NoSensitive(b *testing.B) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("service", "api"),
		slog.String("module", "user"),
		slog.Int("count", 42),
		slog.String("duration", "150ms"),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = engine.MaskAttrs(attrs)
	}
}

// Benchmark MaskEngine.MaskAttrs with sensitive fields (masking path).
func BenchmarkMaskAttrs_WithSensitive(b *testing.B) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("password", "supersecret123"),
		slog.String("email", "john.doe@example.com"),
		slog.String("api_key", "abcdef0123456789abcdef0123456789"),
		slog.String("service", "api"),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = engine.MaskAttrs(attrs)
	}
}

// Benchmark MaskEngine.MaskAttrs with disabled config (fastest path).
func BenchmarkMaskAttrs_Disabled(b *testing.B) {
	engine := NewMaskEngine(NewDisabledMaskConfig())
	attrs := []slog.Attr{
		slog.String("password", "supersecret123"),
		slog.String("email", "john.doe@example.com"),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = engine.MaskAttrs(attrs)
	}
}

// Benchmark ComputeAuditHash.
func BenchmarkComputeAuditHash(b *testing.B) {
	payload := AuditPayload{
		Timestamp: "2026-07-14T12:00:00Z",
		User:      "user-1",
		Action:    "user.update",
		Resource:  "user:456",
		IP:        "10.0.0.1",
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ComputeAuditHash(ZeroHash, payload)
	}
}

// Benchmark VerifyAuditChain with 1000 entries.
func BenchmarkVerifyAuditChain_1000(b *testing.B) {
	records := make([]AuditChainEntry, 1000)
	prev := ZeroHash
	for i := 0; i < 1000; i++ {
		p := AuditPayload{
			Timestamp: "2026-07-14T12:00:00Z",
			User:      "user-1",
			Action:    "action",
			IP:        "10.0.0.1",
		}
		h, _ := ComputeAuditHash(prev, p)
		records[i] = AuditChainEntry{
			Seq:      uint64(i + 1),
			PrevHash: prev,
			Payload:  p,
			CurrHash: h,
		}
		prev = h
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = VerifyAuditChain(records)
	}
}

// Benchmark CarrierFrom (context extraction).
func BenchmarkCarrierFrom(b *testing.B) {
	ctx := WithTraceID(context.Background(), "trace-123")
	ctx = WithUser(ctx, "u-1", "john", "admin")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CarrierFrom(ctx)
	}
}

// Benchmark ResolveCaller (with cache).
func BenchmarkResolveCaller_Cached(b *testing.B) {
	ClearCallerCache()
	// Warm up cache.
	_ = ResolveCaller(1, true)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ResolveCaller(1, true)
	}
}

// Benchmark LevelFilter.Allow.
func BenchmarkLevelFilter(b *testing.B) {
	f := LevelFilter{Min: LevelInfo}
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = f.Allow(ctx, LevelInfo, "msg")
	}
}

// Benchmark SamplingFilter.Allow.
func BenchmarkSamplingFilter(b *testing.B) {
	f := NewSamplingFilter(0.5)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = f.Allow(ctx, LevelInfo, "msg")
	}
}

// Benchmark DedupFilter.Allow (cache hit).
func BenchmarkDedupFilter_Hit(b *testing.B) {
	f := NewDedupFilter(5*60*1e9, 100, 10000) // 5 min window
	ctx := context.Background()
	// Pre-populate.
	for i := 0; i < 100; i++ {
		f.Allow(ctx, LevelInfo, "msg")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = f.Allow(ctx, LevelInfo, "msg")
	}
}

// Benchmark slog.Record creation (baseline for handler benchmarks).
func BenchmarkSlogRecord(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := slog.NewRecord(time.Now(), LevelInfo, "test message", 0)
		r.AddAttrs(slog.String("key", "value"))
		_ = r
	}
}

// Benchmark DynamicLevel.Get (lock-free read).
func BenchmarkDynamicLevel_Get(b *testing.B) {
	dl := NewDynamicLevel(LevelInfo)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = dl.Get()
	}
}

// Benchmark DynamicLevel.Enabled.
func BenchmarkDynamicLevel_Enabled(b *testing.B) {
	dl := NewDynamicLevel(LevelInfo)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = dl.Enabled(ctx, LevelInfo)
	}
}
