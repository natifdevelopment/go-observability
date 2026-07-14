package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// newTestLogger creates a file-based logger at DEBUG level and returns it
// along with a cleanup function and a reader that returns the log file
// contents. Using a temp file avoids polluting test output and lets us
// assert on the emitted structured fields.
func newTestLogger(t *testing.T) (*logger.Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Level = core.LevelDebug
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "http.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false
	cfg.MaskEnabled = false

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	cleanup := func() { _ = log.Close() }
	readFn := func() string {
		data, _ := os.ReadFile(cfg.FilePath)
		return string(data)
	}
	return log, cleanup, readFn
}

// --- Options tests ---

func TestWithSkipPaths(t *testing.T) {
	cfg := &Config{}
	WithSkipPaths("/healthz", "/metrics")(cfg)
	if len(cfg.SkipPaths) != 2 {
		t.Fatalf("expected 2 skip paths, got %d", len(cfg.SkipPaths))
	}
	if cfg.SkipPaths[0] != "/healthz" || cfg.SkipPaths[1] != "/metrics" {
		t.Errorf("unexpected skip paths: %v", cfg.SkipPaths)
	}
}

func TestWithSkipPaths_Appends(t *testing.T) {
	cfg := &Config{SkipPaths: []string{"/ready"}}
	WithSkipPaths("/healthz")(cfg)
	if len(cfg.SkipPaths) != 2 {
		t.Fatalf("expected appended 2 skip paths, got %d", len(cfg.SkipPaths))
	}
}

func TestWithLogBody(t *testing.T) {
	cfg := &Config{}
	WithLogBody(2048)(cfg)
	if !cfg.LogBody {
		t.Error("expected LogBody=true")
	}
	if cfg.MaxBodyBytes != 2048 {
		t.Errorf("expected MaxBodyBytes=2048, got %d", cfg.MaxBodyBytes)
	}
}

func TestWithLogBody_DefaultMaxBytes(t *testing.T) {
	cfg := &Config{}
	WithLogBody(0)(cfg)
	if !cfg.LogBody {
		t.Error("expected LogBody=true")
	}
	if cfg.MaxBodyBytes != core.DefaultBodyMaxBytes {
		t.Errorf("expected default MaxBodyBytes, got %d", cfg.MaxBodyBytes)
	}
}

func TestWithLogBody_NegativeMaxBytes(t *testing.T) {
	cfg := &Config{}
	WithLogBody(-1)(cfg)
	if !cfg.LogBody {
		t.Error("expected LogBody=true")
	}
	if cfg.MaxBodyBytes != core.DefaultBodyMaxBytes {
		t.Errorf("expected default MaxBodyBytes for negative, got %d", cfg.MaxBodyBytes)
	}
}

func TestWithSlowThreshold(t *testing.T) {
	cfg := &Config{}
	WithSlowThreshold(500 * time.Millisecond)(cfg)
	if cfg.SlowRequestThreshold != 500*time.Millisecond {
		t.Errorf("expected 500ms, got %v", cfg.SlowRequestThreshold)
	}
}

func TestWithSlowThreshold_ZeroOrNegative(t *testing.T) {
	cfg := &Config{SlowRequestThreshold: 1 * time.Second}
	WithSlowThreshold(0)(cfg)
	if cfg.SlowRequestThreshold != 1*time.Second {
		t.Errorf("zero should not override existing threshold, got %v", cfg.SlowRequestThreshold)
	}
	WithSlowThreshold(-1 * time.Second)(cfg)
	if cfg.SlowRequestThreshold != 1*time.Second {
		t.Errorf("negative should not override existing threshold, got %v", cfg.SlowRequestThreshold)
	}
}

// --- ResponseRecorder tests ---

func TestResponseRecorder_DefaultStatusCode(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder(), false, 0)
	if rec.StatusCode() != http.StatusOK {
		t.Errorf("expected default 200, got %d", rec.StatusCode())
	}
	if rec.BytesWritten() != 0 {
		t.Errorf("expected 0 bytes, got %d", rec.BytesWritten())
	}
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	rec.WriteHeader(http.StatusCreated)
	if rec.StatusCode() != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.StatusCode())
	}
	if w.Code != http.StatusCreated {
		t.Errorf("underlying writer should receive 201, got %d", w.Code)
	}
}

func TestResponseRecorder_WriteHeader_OnlyOnce(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	rec.WriteHeader(http.StatusTeapot)
	rec.WriteHeader(http.StatusOK) // should be ignored
	if rec.StatusCode() != http.StatusTeapot {
		t.Errorf("expected first status 418, got %d", rec.StatusCode())
	}
	if w.Code != http.StatusTeapot {
		t.Errorf("underlying writer should keep first 418, got %d", w.Code)
	}
}

func TestResponseRecorder_Write_ImpliesHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	_, _ = rec.Write([]byte("hello"))
	if rec.StatusCode() != http.StatusOK {
		t.Errorf("Write should imply 200, got %d", rec.StatusCode())
	}
	if rec.BytesWritten() != 5 {
		t.Errorf("expected 5 bytes, got %d", rec.BytesWritten())
	}
	if w.Body.String() != "hello" {
		t.Errorf("underlying body should be 'hello', got %q", w.Body.String())
	}
}

func TestResponseRecorder_BodyCapture(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, true, 1024)
	_, _ = rec.Write([]byte("response-body"))
	if string(rec.Body()) != "response-body" {
		t.Errorf("captured body = %q, want %q", rec.Body(), "response-body")
	}
}

func TestResponseRecorder_BodyTruncated(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, true, 4)
	_, _ = rec.Write([]byte("hello world"))
	if string(rec.Body()) != "hell" {
		t.Errorf("truncated body = %q, want %q", rec.Body(), "hell")
	}
	if rec.BytesWritten() != 11 {
		t.Errorf("bytes written should be full 11, got %d", rec.BytesWritten())
	}
}

func TestResponseRecorder_BodyDisabled(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	_, _ = rec.Write([]byte("data"))
	if rec.Body() != nil {
		t.Errorf("expected nil body when disabled, got %q", rec.Body())
	}
}

func TestResponseRecorder_Flush(t *testing.T) {
	// Use a recorder that implements Flusher via httptest.ResponseRecorder.
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	// Should not panic.
	rec.Flush()
}

func TestResponseRecorder_Unwrap(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w, false, 0)
	if rec.Unwrap() != w {
		t.Error("Unwrap should return the underlying ResponseWriter")
	}
}

// --- Middleware tests ---

func TestMiddleware_BasicRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := httptest.NewServer(Middleware(log)(mux))
	defer server.Close()

	resp, err := http.Get(server.URL + "/api")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	for _, want := range []string{
		"http request started",
		"http request completed",
		`"method":"GET"`,
		`"path":"/api"`,
		`"status_code":200`,
		`"response_size":2`,
		`"user_agent":"Go-http-client/1.1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestMiddleware_HandlerConvenience(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Handler(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, "http request completed") {
		t.Errorf("output should contain completion log: %s", out)
	}
	if !strings.Contains(out, `"status_code":204`) {
		t.Errorf("output should contain 204: %s", out)
	}
}

func TestMiddleware_NilLogger(t *testing.T) {
	// A nil logger should return the handler unchanged (still functional,
	// just no logging). Verify by exercising the handler end-to-end.
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped := Middleware(nil)(h)
	server := httptest.NewServer(wrapped)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if !called {
		t.Error("handler should still be called with nil logger")
	}

	// Handler convenience with nil logger.
	called2 := false
	h2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called2 = true
		w.WriteHeader(http.StatusOK)
	})
	wrapped2 := Handler(nil, h2)
	server2 := httptest.NewServer(wrapped2)
	defer server2.Close()
	resp2, err := http.Get(server2.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()
	if !called2 {
		t.Error("Handler with nil logger should still call handler")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := Middleware(log, WithSkipPaths("/healthz"))(mux)
	server := httptest.NewServer(h)
	defer server.Close()

	// Hit the skipped path.
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	// Hit a non-skipped path.
	resp2, err := http.Get(server.URL + "/api")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp2.Body.Close()

	out := read()
	// /healthz should NOT appear in logs.
	if strings.Contains(out, `"/healthz"`) {
		t.Errorf("/healthz should be skipped from logging: %s", out)
	}
	// /api should appear.
	if !strings.Contains(out, `"/api"`) {
		t.Errorf("/api should be logged: %s", out)
	}
}

func TestMiddleware_ServerErrorStatus(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"status_code":500`) {
		t.Errorf("expected 500 status: %s", out)
	}
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level for 5xx: %s", out)
	}
}

func TestMiddleware_ClientErrorStatus(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"status_code":404`) {
		t.Errorf("expected 404 status: %s", out)
	}
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Errorf("expected WARN level for 4xx: %s", out)
	}
}

func TestMiddleware_SlowRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log, WithSlowThreshold(20*time.Millisecond))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	out := read()
	if !strings.Contains(out, "slow http request") {
		t.Errorf("expected slow request log: %s", out)
	}
	if !strings.Contains(out, `"is_slow_request":true`) {
		t.Errorf("expected is_slow_request=true: %s", out)
	}
	if !strings.Contains(out, `"threshold":"20ms"`) {
		t.Errorf("expected threshold field: %s", out)
	}
}

func TestMiddleware_NotSlowRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log, WithSlowThreshold(1*time.Second))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()

	out := read()
	if strings.Contains(out, "slow http request") {
		t.Errorf("should not log slow request: %s", out)
	}
}

func TestMiddleware_ClientIP_FromXForwardedFor(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"ip":"203.0.113.5"`) {
		t.Errorf("expected client IP from X-Forwarded-For: %s", out)
	}
}

func TestMiddleware_LogBody_RequestAndResponse(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log, WithLogBody(1024))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("response-payload"))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Post(server.URL+"/", "text/plain", bytes.NewReader([]byte("request-payload")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"request_body":"request-payload"`) {
		t.Errorf("expected request body in start log: %s", out)
	}
	if !strings.Contains(out, `"response_body":"response-payload"`) {
		t.Errorf("expected response body in completion log: %s", out)
	}
}

func TestMiddleware_LogBody_Truncated(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log, WithLogBody(5))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello-world-response"))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Post(server.URL+"/", "text/plain", bytes.NewReader([]byte("hello-world-request")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	// Request body should be truncated to 5 bytes.
	if !strings.Contains(out, `"request_body":"hello"`) {
		t.Errorf("expected truncated request body 'hello': %s", out)
	}
	// Response body should be truncated to 5 bytes.
	if !strings.Contains(out, `"response_body":"hello"`) {
		t.Errorf("expected truncated response body 'hello': %s", out)
	}
	// The downstream handler should still receive the full request body.
}

func TestMiddleware_LogBody_DownstreamReceivesFullBody(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	var received string
	h := Middleware(log, WithLogBody(5))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		received = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Post(server.URL+"/", "text/plain", bytes.NewReader([]byte("hello-world")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if received != "hello-world" {
		t.Errorf("downstream handler should receive full body, got %q", received)
	}
}

func TestMiddleware_TraceContextExtraction(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The trace_id should be available in the request context.
		if got := core.TraceID(r.Context()); got != "0af7651916cd43dd8448eb211c80319c" {
			t.Errorf("expected trace_id in context, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/", nil)
	// Valid W3C traceparent: version-trace_id-parent_id-flags
	req.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"trace_id":"0af7651916cd43dd8448eb211c80319c"`) {
		t.Errorf("expected trace_id in logs: %s", out)
	}
}

func TestMiddleware_ExistingCarrierPreserved(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	// Pre-populate context with a request_id via a wrapping handler.
	inner := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithRequestID(r.Context(), "req-abc-123")
		inner.ServeHTTP(w, r.WithContext(ctx))
	})
	server := httptest.NewServer(wrapped)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"request_id":"req-abc-123"`) {
		t.Errorf("expected request_id from existing carrier in logs: %s", out)
	}
}

func TestMiddleware_POSTMethod(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Post(server.URL+"/", "application/json", bytes.NewReader([]byte(`{"k":"v"}`)))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"method":"POST"`) {
		t.Errorf("expected POST method in logs: %s", out)
	}
	if !strings.Contains(out, `"status_code":201`) {
		t.Errorf("expected 201 status: %s", out)
	}
}

func TestMiddleware_RedirectStatus(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/new", http.StatusFound)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	// Don't follow redirect.
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"status_code":302`) {
		t.Errorf("expected 302 status: %s", out)
	}
	// 3xx should be INFO level.
	if !strings.Contains(out, `"level":"INFO"`) {
		t.Errorf("expected INFO level for 3xx: %s", out)
	}
}

func TestMiddleware_FlushSupport(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = w.Write([]byte("flushed"))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestMiddleware_NilBodyRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log, WithLogBody(1024))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	// GET request has no body.
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, "http request completed") {
		t.Errorf("expected completion log even with no body: %s", out)
	}
}

func TestMiddleware_ContextPassedToHandler(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	type ctxKey struct{}
	key := ctxKey{}

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := r.Context().Value(key)
		if v != "test-value" {
			t.Errorf("expected context value to propagate, got %v", v)
		}
		w.WriteHeader(http.StatusOK)
	}))

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), key, "test-value")
		h.ServeHTTP(w, r.WithContext(ctx))
	})

	server := httptest.NewServer(wrapped)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
}

func TestMiddleware_MultipleOptions(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log,
		WithSkipPaths("/healthz"),
		WithLogBody(512),
		WithSlowThreshold(10*time.Millisecond),
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	// Skipped path.
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	// Logged path (slow).
	resp2, err := http.Post(server.URL+"/", "text/plain", bytes.NewReader([]byte("input")))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp2.Body.Close()

	out := read()
	if strings.Contains(out, `"/healthz"`) {
		t.Errorf("/healthz should be skipped: %s", out)
	}
	if !strings.Contains(out, "slow http request") {
		t.Errorf("expected slow request log: %s", out)
	}
	if !strings.Contains(out, `"request_body":"input"`) {
		t.Errorf("expected request body: %s", out)
	}
}

func TestMiddleware_WriteErrorStillCounts(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	h := Middleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data"))
	}))
	server := httptest.NewServer(h)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"response_size":4`) {
		t.Errorf("expected response_size=4: %s", out)
	}
}
