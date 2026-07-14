package core

import (
	"log/slog"
	"testing"
)

func TestMaskEngine_NilAttrs(t *testing.T) {
	engine := NewDefaultMaskEngine()
	result := engine.MaskAttrs(nil)
	if result != nil {
		t.Error("MaskAttrs(nil) should return nil")
	}
}

func TestMaskEngine_EmptyAttrs(t *testing.T) {
	engine := NewDefaultMaskEngine()
	result := engine.MaskAttrs([]slog.Attr{})
	if len(result) != 0 {
		t.Error("MaskAttrs with empty slice should return empty")
	}
}

func TestMaskEngine_GroupAttrs(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.Group("user",
			slog.String("password", "secret"),
			slog.String("name", "john"),
		),
	}
	masked := engine.MaskAttrs(attrs)
	// Group should be recursed into.
	if masked[0].Value.Kind() != slog.KindGroup {
		t.Fatal("group attr should remain group kind")
	}
	inner := masked[0].Value.Group()
	if inner[0].Value.String() == "secret" {
		t.Error("password inside group should be masked")
	}
	if inner[1].Value.String() != "john" {
		t.Error("name inside group should not be masked")
	}
}

func TestMaskEngine_SliceValue(t *testing.T) {
	engine := NewDefaultMaskEngine()
	data := []map[string]any{
		{"password": "p1", "name": "n1"},
		{"password": "p2", "name": "n2"},
	}
	attrs := []slog.Attr{
		slog.Any("items", data),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("masked slice should not be nil")
	}
}

func TestMaskEngine_NonStringNonSensitive(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.Int("count", 42),
		slog.Bool("enabled", true),
		slog.Float64("rate", 0.5),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Int64() != 42 {
		t.Error("int value should not be masked")
	}
	if !masked[1].Value.Bool() {
		t.Error("bool value should not be masked")
	}
}

func TestMaskEngine_Pin(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("pin", "1234"),
		slog.String("pin_code", "5678"),
	}
	masked := engine.MaskAttrs(attrs)
	for i, m := range masked {
		if m.Value.String() == "1234" || m.Value.String() == "5678" {
			t.Errorf("attr[%d] PIN should be masked", i)
		}
	}
}

func TestMaskEngine_OTP(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("otp", "987654"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == "987654" {
		t.Error("OTP should be masked")
	}
}

func TestMaskEngine_APIKey(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("api_key", "abcdef0123456789abcdef0123456789"),
		slog.String("x_api_key", "key123"),
	}
	masked := engine.MaskAttrs(attrs)
	for i, m := range masked {
		orig := []string{"abcdef0123456789abcdef0123456789", "key123"}
		if m.Value.String() == orig[i] {
			t.Errorf("attr[%d] API key should be masked", i)
		}
	}
}

func TestMaskEngine_PhonePartial(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("phone", "+628123456789"),
	}
	masked := engine.MaskAttrs(attrs)
	val := masked[0].Value.String()
	if val == "+628123456789" {
		t.Error("phone should be masked")
	}
}

func TestMaskEngine_NIK(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("nik", "1234567890123456"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == "1234567890123456" {
		t.Error("NIK should be masked")
	}
}

func TestMaskEngine_NPWP(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("npwp", "12.345.678.9-012.345"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == "12.345.678.9-012.345" {
		t.Error("NPWP should be masked")
	}
}

func TestMaskEngine_Secret(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("client_secret", "my-secret-value"),
		slog.String("private_key", "private-key-data"),
	}
	masked := engine.MaskAttrs(attrs)
	for _, m := range masked {
		if m.Value.String() == "my-secret-value" || m.Value.String() == "private-key-data" {
			t.Error("secret/private_key should be masked")
		}
	}
}

func TestMaskEngine_VaultSecret(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("vault_secret", "vault-data-123"),
		slog.String("vault_token", "vault-token-xyz"),
	}
	masked := engine.MaskAttrs(attrs)
	for _, m := range masked {
		if m.Value.String() == "vault-data-123" || m.Value.String() == "vault-token-xyz" {
			t.Error("vault fields should be masked")
		}
	}
}

func TestMaskEngine_AccessToken(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("access_token", "token-abc-123"),
		slog.String("bearer", "bearer-token"),
	}
	masked := engine.MaskAttrs(attrs)
	for _, m := range masked {
		if m.Value.String() == "token-abc-123" || m.Value.String() == "bearer-token" {
			t.Error("access token fields should be masked")
		}
	}
}

func TestMaskEngine_RefreshToken(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("refresh_token", "refresh-xyz"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == "refresh-xyz" {
		t.Error("refresh_token should be masked")
	}
}

func TestMaskConfig_IsEnabled_FailSecure(t *testing.T) {
	cfg := MaskConfig{
		Enabled:    true,
		Categories: map[MaskCategory]bool{}, // empty map
	}
	// Category not in map should default to enabled (fail-secure).
	if !cfg.IsEnabled(MaskPassword) {
		t.Error("unknown category should be enabled (fail-secure)")
	}
}

func TestMaskConfig_IsEnabled_DisabledGlobal(t *testing.T) {
	cfg := MaskConfig{
		Enabled:    false,
		Categories: map[MaskCategory]bool{MaskPassword: true},
	}
	if cfg.IsEnabled(MaskPassword) {
		t.Error("should be disabled when global Enabled=false")
	}
}

func TestDetectCategoryByValue_Email(t *testing.T) {
	cat, ok := DetectCategoryByValue("john.doe@example.com")
	if !ok || cat != MaskEmail {
		t.Errorf("DetectCategoryByValue(email) = %q, %v, want MaskEmail, true", cat, ok)
	}
}

func TestDetectCategoryByValue_NonSensitive(t *testing.T) {
	_, ok := DetectCategoryByValue("hello world")
	if ok {
		t.Error("non-sensitive value should not detect any category")
	}
}

func TestAllMaskCategories(t *testing.T) {
	cats := AllMaskCategories()
	if len(cats) != 15 {
		t.Errorf("AllMaskCategories count = %d, want 15", len(cats))
	}
}
