package core

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestMaskRecord_Basic(t *testing.T) {
	engine := NewDefaultMaskEngine()
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	record.AddAttrs(
		slog.String("password", "secret123"),
		slog.String("normal", "value"),
	)

	masked := engine.MaskRecord(record)

	var attrs []slog.Attr
	masked.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if len(attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(attrs))
	}

	// Password should be masked.
	found := false
	for _, attr := range attrs {
		if attr.Key == "password" {
			found = true
			if attr.Value.String() == "secret123" {
				t.Error("password should be masked")
			}
		}
	}
	if !found {
		t.Error("password attr should be present")
	}

	// Normal attr should be unchanged.
	for _, attr := range attrs {
		if attr.Key == "normal" && attr.Value.String() != "value" {
			t.Error("normal attr should be unchanged")
		}
	}
}

func TestMaskRecord_Disabled(t *testing.T) {
	cfg := NewDefaultMaskConfig()
	cfg.Enabled = false
	engine := NewMaskEngine(cfg)
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	record.AddAttrs(slog.String("password", "secret123"))

	masked := engine.MaskRecord(record)

	var attrs []slog.Attr
	masked.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if attrs[0].Value.String() != "secret123" {
		t.Error("password should not be masked when disabled")
	}
}

func TestMaskRecord_NoAttrs(t *testing.T) {
	engine := NewDefaultMaskEngine()
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	masked := engine.MaskRecord(record)
	if masked.Message != "test" {
		t.Error("message should be preserved")
	}
}

func TestCarrier_AddAttrsToRecord(t *testing.T) {
	c := Carrier{
		TraceID:      "trace-123",
		RequestID:    "req-456",
		CorrelationID: "corr-789",
		SessionID:    "sess-012",
		UserID:       "user-1",
		Username:     "john",
		Role:         "admin",
		IP:           "10.0.0.1",
		Method:       "GET",
		Path:         "/api/users",
		StatusCode:   200,
		Extra:        map[string]string{"custom": "val"},
	}

	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	c.AddAttrsToRecord(&record)

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if len(attrs) < 12 {
		t.Errorf("expected at least 12 attrs, got %d", len(attrs))
	}

	// Verify specific fields.
	found := map[string]bool{}
	for _, attr := range attrs {
		found[attr.Key] = true
	}
	for _, key := range []string{"trace_id", "request_id", "correlation_id", "session_id", "user_id", "username", "role", "ip", "method", "path", "status_code", "custom"} {
		if !found[key] {
			t.Errorf("attr %q should be present", key)
		}
	}
}

func TestCarrier_AddAttrsToRecord_Empty(t *testing.T) {
	c := Carrier{}
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	c.AddAttrsToRecord(&record)

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if len(attrs) != 0 {
		t.Errorf("empty carrier should add 0 attrs, got %d", len(attrs))
	}
}

func TestCarrier_AddAttrsToRecord_Partial(t *testing.T) {
	c := Carrier{
		TraceID: "trace-123",
		UserID:  "user-1",
	}
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	c.AddAttrsToRecord(&record)

	var attrs []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	if len(attrs) != 2 {
		t.Errorf("partial carrier should add 2 attrs, got %d", len(attrs))
	}
}

func TestMaskRecord_WithCarrier(t *testing.T) {
	engine := NewDefaultMaskEngine()
	ctx := WithTraceID(context.Background(), "trace-123")
	ctx = WithUser(ctx, "user-1", "john", "admin")

	carrier := CarrierFrom(ctx)
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	carrier.AddAttrsToRecord(&record)

	// Add a sensitive field.
	record.AddAttrs(slog.String("password", "secret"))

	masked := engine.MaskRecord(record)

	var attrs []slog.Attr
	masked.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	// trace_id and user_id should be present (not masked).
	foundTrace := false
	foundUser := false
	foundMaskedPassword := false
	for _, attr := range attrs {
		if attr.Key == "trace_id" && attr.Value.String() == "trace-123" {
			foundTrace = true
		}
		if attr.Key == "user_id" && attr.Value.String() == "user-1" {
			foundUser = true
		}
		if attr.Key == "password" && attr.Value.String() != "secret" {
			foundMaskedPassword = true
		}
	}
	if !foundTrace {
		t.Error("trace_id should be present and unmasked")
	}
	if !foundUser {
		t.Error("user_id should be present and unmasked")
	}
	if !foundMaskedPassword {
		t.Error("password should be masked")
	}
}
