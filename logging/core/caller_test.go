package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestResolveCaller(t *testing.T) {
	ClearCallerCache()
	info := ResolveCaller(2, true) // skip ResolveCaller + this test func
	if info.File == "" {
		t.Error("File should not be empty")
	}
	if info.Line == 0 {
		t.Error("Line should not be 0")
	}
	// tRunner is the test runner; the actual caller is the test function.
	// We just verify we got SOMETHING meaningful.
	if info.Function == "" {
		t.Error("Function should not be empty")
	}
}

func TestResolveCaller_Disabled(t *testing.T) {
	info := ResolveCaller(2, false)
	if info.File != "" || info.Line != 0 {
		t.Error("disabled caller should return zero value")
	}
}

func TestResolveCaller_Cached(t *testing.T) {
	ClearCallerCache()
	info1 := ResolveCaller(2, true)
	info2 := ResolveCaller(2, true)
	if info1.PC != info2.PC {
		t.Error("same call site should return same PC")
	}
	if info1.Line != info2.Line {
		t.Error("cached call should return same Line")
	}
	if CallerCacheSize() == 0 {
		t.Error("cache should have entries after calls")
	}
}

func TestStacktrace(t *testing.T) {
	s := Stacktrace()
	if s == "" {
		t.Error("Stacktrace should not be empty")
	}
	if !strings.Contains(s, "goroutine") {
		t.Error("stacktrace should contain 'goroutine'")
	}
}

func TestExtractTraceContext_Valid(t *testing.T) {
	h := http.Header{}
	h.Set(W3CTraceparentHeader, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	c := ExtractTraceContext(h)
	if c.TraceID != "0af7651916cd43dd8448eb211c80319c" {
		t.Errorf("TraceID = %q, want W3C trace ID", c.TraceID)
	}
}

func TestExtractTraceContext_Absent(t *testing.T) {
	h := http.Header{}
	c := ExtractTraceContext(h)
	if c.TraceID != "" {
		t.Error("TraceID should be empty when header absent")
	}
}

func TestExtractTraceContext_Malformed(t *testing.T) {
	h := http.Header{}
	h.Set(W3CTraceparentHeader, "invalid")
	c := ExtractTraceContext(h)
	if c.TraceID != "" {
		t.Error("TraceID should be empty for malformed header")
	}
}

func TestInjectTraceContext(t *testing.T) {
	ctx := WithTraceID(context.Background(), "0af7651916cd43dd8448eb211c80319c")
	h := http.Header{}
	h = InjectTraceContext(ctx, h)

	tp := h.Get(W3CTraceparentHeader)
	if tp == "" {
		t.Fatal("traceparent header should be set")
	}
	if !strings.Contains(tp, "0af7651916cd43dd8448eb211c80319c") {
		t.Errorf("traceparent should contain trace ID, got %q", tp)
	}
}

func TestInjectTraceContext_NoCarrier(t *testing.T) {
	ctx := context.Background()
	h := http.Header{}
	h = InjectTraceContext(ctx, h)
	if h.Get(W3CTraceparentHeader) != "" {
		t.Error("traceparent should not be set when no carrier")
	}
}

func TestInjectTraceContext_NilHeaders(t *testing.T) {
	ctx := WithTraceID(context.Background(), "abc123")
	h := InjectTraceContext(ctx, nil)
	if h == nil {
		t.Fatal("headers should not be nil after inject")
	}
	if h.Get(W3CTraceparentHeader) == "" {
		t.Error("traceparent should be set")
	}
}
