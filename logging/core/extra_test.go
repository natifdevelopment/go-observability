package core

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestCapturePanicCtx(t *testing.T) {
	ctx := context.Background()
	info, panicked := CapturePanicCtx(ctx, func() {
		panic("ctx panic")
	})
	if !panicked {
		t.Error("should have panicked")
	}
	if info.Message != "ctx panic" {
		t.Errorf("Message = %q, want 'ctx panic'", info.Message)
	}
}

func TestCapturePanicCtx_NoPanic(t *testing.T) {
	ctx := context.Background()
	_, panicked := CapturePanicCtx(ctx, func() {
		// no panic
	})
	if panicked {
		t.Error("should not have panicked")
	}
}

func TestSprintAny(t *testing.T) {
	if sprintAny(nil) != "<nil>" {
		t.Error("sprintAny(nil) should be '<nil>'")
	}
	if sprintAny("hello") != "hello" {
		t.Error("sprintAny string failed")
	}
	if sprintAny(42) != "42" {
		t.Error("sprintAny int failed")
	}
	if sprintAny(true) != "true" {
		t.Error("sprintAny bool failed")
	}
}

func TestParseFileLine(t *testing.T) {
	file, line := parseFileLine("/path/to/file.go:123")
	if file != "/path/to/file.go" {
		t.Errorf("file = %q", file)
	}
	if line != 123 {
		t.Errorf("line = %d, want 123", line)
	}
}

func TestParseFileLine_NoColon(t *testing.T) {
	file, line := parseFileLine("no-colon-here")
	if file != "no-colon-here" {
		t.Errorf("file = %q", file)
	}
	if line != 0 {
		t.Errorf("line = %d, want 0", line)
	}
}

func TestTrimFuncName(t *testing.T) {
	result := trimFuncName("github.com/natifdevelopment/go-observability/logging/core.foo")
	if result != "logging/core.foo" {
		t.Errorf("trimFuncName = %q, want 'logging/core.foo'", result)
	}
	// Single segment.
	result = trimFuncName("foo")
	if result != "foo" {
		t.Errorf("trimFuncName = %q, want 'foo'", result)
	}
}

func TestIsUserFrame(t *testing.T) {
	if isUserFrame("runtime.main") {
		t.Error("runtime frame should not be user frame")
	}
	if isUserFrame("panic.recover") {
		t.Error("panic frame should not be user frame")
	}
	if !isUserFrame("myapp.handler.ServeHTTP") {
		t.Error("myapp frame should be user frame")
	}
}

func TestItoa(t *testing.T) {
	if itoa(0) != "0" {
		t.Error("itoa(0) failed")
	}
	if itoa(42) != "42" {
		t.Error("itoa(42) failed")
	}
	if itoa(-5) != "-5" {
		t.Error("itoa(-5) failed")
	}
	if itoa(123456) != "123456" {
		t.Error("itoa(123456) failed")
	}
}

func TestIntToString(t *testing.T) {
	if intToString(0) != "0" {
		t.Error("intToString(0) failed")
	}
	if intToString(42) != "42" {
		t.Error("intToString(42) failed")
	}
	if intToString(-100) != "-100" {
		t.Error("intToString(-100) failed")
	}
}

func TestUintToString(t *testing.T) {
	if uintToString(0) != "0" {
		t.Error("uintToString(0) failed")
	}
	if uintToString(42) != "42" {
		t.Error("uintToString(42) failed")
	}
}

func TestDynamicLevel_LevelMethod(t *testing.T) {
	dl := NewDynamicLevel(LevelWarn)
	// Level() implements slog.Leveler interface.
	if dl.Level() != LevelWarn {
		t.Error("Level() should return Warn")
	}
}

func TestRandomSamplingFilter(t *testing.T) {
	f := NewRandomSamplingFilter(1.0)
	if !f.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("rate 1.0 should always allow")
	}
	f0 := NewRandomSamplingFilter(0.0)
	if f0.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("rate 0.0 should never allow")
	}
}

func TestDedupFilter_Eviction(t *testing.T) {
	// Small maxSize to trigger eviction.
	f := NewDedupFilter(5*time.Minute, 100, 3)
	ctx := context.Background()
	// Fill cache to capacity.
	f.Allow(ctx, LevelInfo, "msg1")
	f.Allow(ctx, LevelInfo, "msg2")
	f.Allow(ctx, LevelInfo, "msg3")
	// Adding 4th should trigger eviction of oldest.
	f.Allow(ctx, LevelInfo, "msg4")
	// All should still work (just testing no panic).
}

func TestCompilePattern(t *testing.T) {
	// Valid pattern.
	re := compilePattern(`\d+`)
	if re == nil {
		t.Error("valid pattern should compile")
	}
	// Empty pattern.
	if compilePattern("") != nil {
		t.Error("empty pattern should return nil")
	}
	// Invalid pattern.
	if compilePattern(`[invalid`) != nil {
		t.Error("invalid pattern should return nil")
	}
}

func TestHookChain_Name(t *testing.T) {
	chain := NewHookChain(NoopHook{})
	if chain.Name() != "hook_chain" {
		t.Error("HookChain.Name should be 'hook_chain'")
	}
}

func TestHookChain_Hooks(t *testing.T) {
	h1 := NoopHook{}
	h2 := NoopHook{}
	chain := NewHookChain(h1, h2)
	hooks := chain.Hooks()
	if len(hooks) != 2 {
		t.Errorf("Hooks() returned %d, want 2", len(hooks))
	}
}

func TestHookChain_AfterWrite(t *testing.T) {
	chain := NewHookChain(NoopHook{})
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	// Should not panic.
	chain.AfterWrite(context.Background(), record, []byte("payload"), nil)
}

func TestMaskEngine_MaskHash(t *testing.T) {
	// Test hash strategy via direct category that uses hash.
	// Currently no default category uses hash, but we can test maskValueByCategory
	// indirectly by checking that MaskValue works for all categories.
	engine := NewDefaultMaskEngine()
	for _, cat := range AllMaskCategories() {
		result := engine.MaskValue(cat, "test-value")
		if result == "" {
			t.Errorf("MaskValue(%s) returned empty", cat)
		}
	}
}

func TestNoopHook_AfterWrite(t *testing.T) {
	h := NoopHook{}
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	// Should not panic.
	h.AfterWrite(context.Background(), record, []byte("payload"), nil)
}

func TestMaskEngine_MaskHashStrategy(t *testing.T) {
	// Force hash strategy by using a custom pattern.
	cfg := NewDefaultMaskConfig()
	cfg.CustomPatterns = map[string]MaskPattern{
		"hash_field": {
			Name:         "hash_field",
			KeyMatch:     []string{"hash_field"},
			MaskStrategy: MaskStrategyHash,
		},
	}
	engine := NewMaskEngine(cfg)
	attrs := []slog.Attr{
		slog.String("hash_field", "sensitive-data"),
	}
	masked := engine.MaskAttrs(attrs)
	val := masked[0].Value.String()
	if val == "sensitive-data" {
		t.Error("hash_field should be masked")
	}
	if len(val) < 10 {
		t.Errorf("hash output should be longer, got %q", val)
	}
}

func TestValueMatchesPattern(t *testing.T) {
	// JWT pattern.
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	if !ValueMatchesPattern(MaskJWT, jwt) {
		t.Error("JWT value should match pattern")
	}
	if ValueMatchesPattern(MaskJWT, "not-a-jwt") {
		t.Error("non-JWT should not match")
	}
	// Email.
	if !ValueMatchesPattern(MaskEmail, "test@example.com") {
		t.Error("email should match")
	}
	// Phone.
	if !ValueMatchesPattern(MaskPhoneNumber, "+628123456789") {
		t.Error("phone should match")
	}
	// NIK.
	if !ValueMatchesPattern(MaskNIK, "1234567890123456") {
		t.Error("NIK should match")
	}
	// NPWP.
	if !ValueMatchesPattern(MaskNPWP, "12.345.678.9-012.345") {
		t.Error("NPWP should match")
	}
	// Category without pattern.
	if ValueMatchesPattern(MaskPassword, "secret") {
		t.Error("password category has no value pattern, should return false")
	}
}
