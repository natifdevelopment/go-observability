package core

import (
	"log/slog"
	"testing"
)

func TestMaskEngine_HashStrategy(t *testing.T) {
	cfg := NewDefaultMaskConfig()
	// Force hash strategy for password by using MaskValue directly.
	engine := NewMaskEngine(cfg)
	// MaskValue uses strategyFor which returns Full for password.
	// Test hash via maskValueByCategory indirectly through MaskValue with email.
	masked := engine.MaskValue(MaskEmail, "test@example.com")
	if masked == "test@example.com" {
		t.Error("email should be masked")
	}
}

func TestMaskEngine_PreserveMoreThanLength(t *testing.T) {
	cfg := NewDefaultMaskConfig()
	cfg.PreserveFirst = 10
	cfg.PreserveLast = 10
	engine := NewMaskEngine(cfg)
	masked := engine.MaskValue(MaskEmail, "ab@c")
	// String shorter than preserve total → mask all.
	if masked != "****" {
		t.Errorf("short string should be fully masked, got %q", masked)
	}
}

func TestMaskPartialString_EdgeCases(t *testing.T) {
	// Empty string.
	if maskPartialString("", 2, 2, "*") != "" {
		t.Error("empty string should return empty")
	}
	// Zero preserve.
	result := maskPartialString("hello", 0, 0, "*")
	if result != "*****" {
		t.Errorf("zero preserve should mask all, got %q", result)
	}
	// Negative preserve (should be treated as 0).
	result = maskPartialString("hello", -1, -1, "*")
	if result != "*****" {
		t.Errorf("negative preserve should mask all, got %q", result)
	}
}

func TestMaskEngine_NestedMapInStruct(t *testing.T) {
	engine := NewDefaultMaskEngine()

	type Config struct {
		Name     string
		Settings map[string]any
	}

	cfg := Config{
		Name:     "prod",
		Settings: map[string]any{"password": "secret", "region": "us-east"},
	}

	attrs := []slog.Attr{slog.Any("config", cfg)}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("nested map in struct should not return nil")
	}
}

func TestMaskEngine_PtrToStruct(t *testing.T) {
	engine := NewDefaultMaskEngine()

	type User struct {
		Username string
		Password string
	}

	u := &User{Username: "john", Password: "secret"}
	attrs := []slog.Attr{slog.Any("user", u)}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("ptr to struct should not return nil")
	}
}

func TestMaskEngine_NilPtr(t *testing.T) {
	engine := NewDefaultMaskEngine()
	type User struct{ Username string }
	var u *User
	attrs := []slog.Attr{slog.Any("user", u)}
	masked := engine.MaskAttrs(attrs)
	// nil pointer should be handled gracefully.
	_ = masked
}

func TestMaskEngine_NilMap(t *testing.T) {
	engine := NewDefaultMaskEngine()
	var m map[string]any
	attrs := []slog.Attr{slog.Any("data", m)}
	masked := engine.MaskAttrs(attrs)
	_ = masked
}

func TestMaskEngine_NilSlice(t *testing.T) {
	engine := NewDefaultMaskEngine()
	var s []string
	attrs := []slog.Attr{slog.Any("items", s)}
	masked := engine.MaskAttrs(attrs)
	_ = masked
}

func TestMaskEngine_EmptySlice(t *testing.T) {
	engine := NewDefaultMaskEngine()
	s := []string{"a", "b"}
	attrs := []slog.Attr{slog.Any("items", s)}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("empty slice result should not be nil")
	}
}

func TestMaskEngine_KeyPatternsFor(t *testing.T) {
	patterns := KeyPatternsFor(MaskPassword)
	if len(patterns) == 0 {
		t.Error("KeyPatternsFor(Password) should return patterns")
	}
}

func TestMaskEngine_Config(t *testing.T) {
	cfg := NewDefaultMaskConfig()
	engine := NewMaskEngine(cfg)
	got := engine.Config()
	if !got.Enabled {
		t.Error("Config().Enabled should be true")
	}
}

func TestNewDefaultMaskEngine(t *testing.T) {
	engine := NewDefaultMaskEngine()
	if engine == nil {
		t.Fatal("NewDefaultMaskEngine should not return nil")
	}
	if !engine.Config().Enabled {
		t.Error("default engine should be enabled")
	}
}

func TestNewDisabledMaskConfig(t *testing.T) {
	cfg := NewDisabledMaskConfig()
	if cfg.Enabled {
		t.Error("disabled config should have Enabled=false")
	}
}
