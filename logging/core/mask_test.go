package core

import (
	"log/slog"
	"testing"
)

func TestMaskEngine_Password(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("password", "supersecret123"),
		slog.String("username", "john"),
	}
	masked := engine.MaskAttrs(attrs)

	if masked[0].Value.String() == "supersecret123" {
		t.Error("password should be masked")
	}
	if masked[1].Value.String() != "john" {
		t.Error("username should not be masked")
	}
}

func TestMaskEngine_CVV_Dropped(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("cvv", "123"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() != "[REDACTED]" {
		t.Errorf("CVV should be redacted, got %q", masked[0].Value.String())
	}
}

func TestMaskEngine_Email_Partial(t *testing.T) {
	engine := NewDefaultMaskEngine()
	attrs := []slog.Attr{
		slog.String("email", "john.doe@example.com"),
	}
	masked := engine.MaskAttrs(attrs)
	val := masked[0].Value.String()
	if val == "john.doe@example.com" {
		t.Error("email should be masked")
	}
	// Partial masking preserves first 2 and last 2 chars.
	if len(val) < 4 {
		t.Errorf("masked email too short: %q", val)
	}
}

func TestMaskEngine_Disabled(t *testing.T) {
	engine := NewMaskEngine(NewDisabledMaskConfig())
	attrs := []slog.Attr{
		slog.String("password", "supersecret123"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() != "supersecret123" {
		t.Error("with disabled masking, password should be visible")
	}
}

func TestMaskEngine_NestedStruct(t *testing.T) {
	engine := NewDefaultMaskEngine()

	type User struct {
		Username string
		Password string
		Email    string
	}

	user := User{
		Username: "john",
		Password: "secret",
		Email:    "john@test.com",
	}

	attrs := []slog.Attr{
		slog.Any("user", user),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("masked struct should not be nil")
	}
}

func TestMaskEngine_MapWithPassword(t *testing.T) {
	engine := NewDefaultMaskEngine()

	data := map[string]any{
		"username": "john",
		"password": "secret123",
	}

	attrs := []slog.Attr{
		slog.Any("data", data),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.Any() == nil {
		t.Error("masked map should not be nil")
	}
}

func TestMaskEngine_ValueDetection_JWT(t *testing.T) {
	engine := NewDefaultMaskEngine()
	// Valid JWT format (3 base64url segments).
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	attrs := []slog.Attr{
		slog.String("token", jwt),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == jwt {
		t.Error("JWT value should be masked by value detection")
	}
}

func TestMaskEngine_ValueDetection_CreditCard(t *testing.T) {
	engine := NewDefaultMaskEngine()
	// Valid Luhn credit card number (test: 4111111111111111).
	cc := "4111111111111111"
	attrs := []slog.Attr{
		slog.String("card_data", cc),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == cc {
		t.Error("credit card value should be masked by value detection")
	}
}

func TestMaskEngine_LuhnCheck(t *testing.T) {
	// Valid Luhn numbers.
	valid := []string{
		"4111111111111111", // Visa test
		"5500000000000004", // Mastercard test
	}
	for _, n := range valid {
		if !luhnCheck(n) {
			t.Errorf("luhnCheck(%q) should be true", n)
		}
	}
	// Invalid Luhn.
	if luhnCheck("4111111111111112") {
		t.Error("luhnCheck should be false for invalid number")
	}
}

func TestMaskEngine_CustomPattern(t *testing.T) {
	cfg := NewDefaultMaskConfig()
	cfg.CustomPatterns = map[string]MaskPattern{
		"custom_token": {
			Name:       "custom_token",
			KeyMatch:   []string{"custom_token"},
			MaskStrategy: MaskStrategyFull,
		},
	}
	engine := NewMaskEngine(cfg)
	attrs := []slog.Attr{
		slog.String("custom_token", "abcdef123456"),
	}
	masked := engine.MaskAttrs(attrs)
	if masked[0].Value.String() == "abcdef123456" {
		t.Error("custom_token should be masked")
	}
}

func TestMaskPartialString(t *testing.T) {
	result := maskPartialString("hello world", 2, 2, "*")
	if result != "he*******ld" {
		t.Errorf("maskPartialString = %q, want 'he*******ld'", result)
	}
	// Edge case: preserve more than length.
	result = maskPartialString("abc", 5, 5, "*")
	if result != "***" {
		t.Errorf("maskPartialString = %q, want '***'", result)
	}
}

func TestMaskValue_PublicAPI(t *testing.T) {
	engine := NewDefaultMaskEngine()
	masked := engine.MaskValue(MaskPassword, "secret123")
	if masked == "secret123" {
		t.Error("MaskValue should mask the password")
	}
	// Disabled category.
	cfg := NewDefaultMaskConfig()
	cfg.Categories[MaskPassword] = false
	engine2 := NewMaskEngine(cfg)
	masked2 := engine2.MaskValue(MaskPassword, "secret123")
	if masked2 != "secret123" {
		t.Error("MaskValue with disabled category should not mask")
	}
}
