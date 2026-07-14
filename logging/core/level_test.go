package core

import (
	"context"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
		err   bool
	}{
		{"TRACE", LevelTrace, false},
		{"trace", LevelTrace, false},
		{"DEBUG", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"WARNING", LevelWarn, false},
		{"ERROR", LevelError, false},
		{"ERR", LevelError, false},
		{"FATAL", LevelFatal, false},
		{"PANIC", LevelPanic, false},
		{"INVALID", 0, true},
		{"", 0, true},
		{"  info  ", LevelInfo, false},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if tt.err {
			if err == nil {
				t.Errorf("ParseLevel(%q) expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLevel(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestLevelLabel(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelTrace, "TRACE"},
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{LevelPanic, "PANIC"},
	}
	for _, tt := range tests {
		got := LevelLabel(tt.level)
		if got != tt.want {
			t.Errorf("LevelLabel(%d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestLevelEnabled(t *testing.T) {
	if !LevelEnabled(LevelInfo, LevelError) {
		t.Error("LevelEnabled(Info, Error) should be true")
	}
	if LevelEnabled(LevelWarn, LevelDebug) {
		t.Error("LevelEnabled(Warn, Debug) should be false")
	}
}

func TestLevelToSlogStandard(t *testing.T) {
	if LevelToSlogStandard(LevelTrace) != LevelDebug {
		t.Error("TRACE should map to DEBUG")
	}
	if LevelToSlogStandard(LevelFatal) != LevelError {
		t.Error("FATAL should map to ERROR")
	}
	if LevelToSlogStandard(LevelPanic) != LevelError {
		t.Error("PANIC should map to ERROR")
	}
	if LevelToSlogStandard(LevelInfo) != LevelInfo {
		t.Error("INFO should stay INFO")
	}
}

func TestCarrierMerge(t *testing.T) {
	parent := Carrier{TraceID: "abc", UserID: "123"}
	override := Carrier{UserID: "456", IP: "10.0.0.1"}

	merged := MergeCarrier(parent, override)

	if merged.TraceID != "abc" {
		t.Errorf("TraceID should be preserved from parent, got %q", merged.TraceID)
	}
	if merged.UserID != "456" {
		t.Errorf("UserID should be overridden, got %q", merged.UserID)
	}
	if merged.IP != "10.0.0.1" {
		t.Errorf("IP should be set from override, got %q", merged.IP)
	}
}

func TestCarrierContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithUser(ctx, "user-1", "john", "admin")

	if TraceID(ctx) != "trace-123" {
		t.Errorf("TraceID = %q, want 'trace-123'", TraceID(ctx))
	}
	if UserID(ctx) != "user-1" {
		t.Errorf("UserID = %q, want 'user-1'", UserID(ctx))
	}
	if Username(ctx) != "john" {
		t.Errorf("Username = %q, want 'john'", Username(ctx))
	}
	if Role(ctx) != "admin" {
		t.Errorf("Role = %q, want 'admin'", Role(ctx))
	}
}

func TestCarrierIsZero(t *testing.T) {
	if !(Carrier{}).IsZero() {
		t.Error("empty Carrier should be zero")
	}
	c := Carrier{TraceID: "abc"}
	if c.IsZero() {
		t.Error("Carrier with TraceID should not be zero")
	}
}

func TestCarrierExtra(t *testing.T) {
	ctx := context.Background()
	ctx = WithExtra(ctx, "tenant_id", "t-001")

	if Extra(ctx, "tenant_id") != "t-001" {
		t.Errorf("Extra(tenant_id) = %q, want 't-001'", Extra(ctx, "tenant_id"))
	}
	if Extra(ctx, "nonexistent") != "" {
		t.Error("nonexistent extra should return empty string")
	}
}

func TestCarrierFromNilContext(t *testing.T) {
	c := CarrierFrom(nil)
	if !c.IsZero() {
		t.Error("CarrierFrom(nil) should return zero Carrier")
	}
}
