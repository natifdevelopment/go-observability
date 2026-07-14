package core

import (
	"context"
	"errors"
	"testing"
)

// Test all context With* and accessor functions.

func TestContextAllAccessors(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithCorrelationID(ctx, "corr-1")
	ctx = WithSessionID(ctx, "sess-1")
	ctx = WithUser(ctx, "u-1", "john", "admin")
	ctx = WithIP(ctx, "10.0.0.1")
	ctx = WithHTTP(ctx, "GET", "/api/users")
	ctx = WithStatusCode(ctx, 200)

	if TraceID(ctx) != "trace-1" {
		t.Error("TraceID mismatch")
	}
	if RequestID(ctx) != "req-1" {
		t.Error("RequestID mismatch")
	}
	if CorrelationID(ctx) != "corr-1" {
		t.Error("CorrelationID mismatch")
	}
	if SessionID(ctx) != "sess-1" {
		t.Error("SessionID mismatch")
	}
	if UserID(ctx) != "u-1" {
		t.Error("UserID mismatch")
	}
	if Username(ctx) != "john" {
		t.Error("Username mismatch")
	}
	if Role(ctx) != "admin" {
		t.Error("Role mismatch")
	}
	if IP(ctx) != "10.0.0.1" {
		t.Error("IP mismatch")
	}
	if Method(ctx) != "GET" {
		t.Error("Method mismatch")
	}
	if Path(ctx) != "/api/users" {
		t.Error("Path mismatch")
	}
	if StatusCode(ctx) != 200 {
		t.Error("StatusCode mismatch")
	}
}

func TestContextAccessors_Empty(t *testing.T) {
	ctx := context.Background()
	if TraceID(ctx) != "" {
		t.Error("TraceID should be empty")
	}
	if RequestID(ctx) != "" {
		t.Error("RequestID should be empty")
	}
	if StatusCode(ctx) != 0 {
		t.Error("StatusCode should be 0")
	}
}

func TestContextChaining(t *testing.T) {
	ctx := context.Background()
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithUser(ctx, "u-1", "john", "admin")
	ctx = WithIP(ctx, "10.0.0.1")

	// All fields should be preserved.
	if TraceID(ctx) != "trace-1" {
		t.Error("TraceID lost after chaining")
	}
	if UserID(ctx) != "u-1" {
		t.Error("UserID lost after chaining")
	}
	if IP(ctx) != "10.0.0.1" {
		t.Error("IP lost after chaining")
	}
}

func TestWrapError(t *testing.T) {
	original := errors.New("disk full")
	wrapped := WrapError(original, "failed to write to %s", "/var/log/app.log")
	if !errors.Is(wrapped, original) {
		t.Error("wrapped error should unwrap to original")
	}
	if wrapped.Error() == "" {
		t.Error("wrapped error message should not be empty")
	}
}

func TestWrapError_Nil(t *testing.T) {
	if WrapError(nil, "context") != nil {
		t.Error("WrapError(nil, ...) should return nil")
	}
}

func TestEventRegistry_List(t *testing.T) {
	r := NewEventRegistry()
	r.MustRegister(EventMeta{ID: "a.1", Category: EventCategoryBusiness, Name: "A1"})
	r.MustRegister(EventMeta{ID: "a.2", Category: EventCategoryBusiness, Name: "A2"})
	list := r.List()
	if len(list) != 2 {
		t.Errorf("List() returned %d, want 2", len(list))
	}
}

func TestEventRegistry_GetNotFound(t *testing.T) {
	r := NewEventRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent event")
	}
}

func TestEventRegistry_MustRegister_Panics(t *testing.T) {
	r := NewEventRegistry()
	r.MustRegister(EventMeta{ID: "dup", Category: EventCategoryBusiness, Name: "D"})

	defer func() {
		if recover() == nil {
			t.Error("MustRegister should panic on duplicate")
		}
	}()
	r.MustRegister(EventMeta{ID: "dup", Category: EventCategoryBusiness, Name: "D2"})
}

func TestEventRegistry_Count(t *testing.T) {
	r := NewEventRegistry()
	if r.Count() != 0 {
		t.Error("empty registry count should be 0")
	}
	r.MustRegister(EventMeta{ID: "x", Category: EventCategoryBusiness})
	if r.Count() != 1 {
		t.Errorf("count = %d, want 1", r.Count())
	}
}
