package request

import (
	"context"
	"errors"
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

// newTestLogger creates a logger writing to a temp file and returns it along
// with a cleanup function and a function to read the log output.
func newTestLogger(t *testing.T) (*logger.Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "test.log")
	cfg.Level = core.LevelDebug
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false

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

func TestNew(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	f := New(log)
	if f == nil {
		t.Fatal("New returned nil")
	}
	if f.logger == nil {
		t.Fatal("Facade.logger is nil")
	}
}

func TestFacade_LogRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogRequest(context.Background(), "GET", "/users", 200, 42*time.Millisecond,
		core.RequestIDAttr("req-123"))

	out := read()
	if !strings.Contains(out, "request completed") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, `"method":"GET"`) {
		t.Errorf("output missing method: %q", out)
	}
	if !strings.Contains(out, `"path":"/users"`) {
		t.Errorf("output missing path: %q", out)
	}
	if !strings.Contains(out, `"status_code":200`) {
		t.Errorf("output missing status_code: %q", out)
	}
	if !strings.Contains(out, `"request_id":"req-123"`) {
		t.Errorf("output missing extra attr: %q", out)
	}
	if !strings.Contains(out, `"duration_ms":42`) {
		t.Errorf("output missing duration_ms: %q", out)
	}
}

func TestFacade_LogRequest_4xx_Warn(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogRequest(context.Background(), "GET", "/missing", 404, 5*time.Millisecond)

	out := read()
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Errorf("expected WARN level for 404: %q", out)
	}
}

func TestFacade_LogRequest_5xx_Error(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogRequest(context.Background(), "GET", "/boom", 500, 5*time.Millisecond)

	out := read()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level for 500: %q", out)
	}
}

func TestFacade_LogRequestStart(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogRequestStart(context.Background(), "POST", "/items")

	out := read()
	if !strings.Contains(out, "request started") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, `"method":"POST"`) {
		t.Errorf("output missing method: %q", out)
	}
	if !strings.Contains(out, `"path":"/items"`) {
		t.Errorf("output missing path: %q", out)
	}
	if !strings.Contains(out, `"level":"DEBUG"`) {
		t.Errorf("expected DEBUG level: %q", out)
	}
}

func TestFacade_LogError(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	err := errors.New("db connection refused")
	f.LogError(context.Background(), "GET", "/users", 500, err)

	out := read()
	if !strings.Contains(out, "request error") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, `"error":"db connection refused"`) {
		t.Errorf("output missing error field: %q", out)
	}
	if !strings.Contains(out, `"status_code":500`) {
		t.Errorf("output missing status_code: %q", out)
	}
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level: %q", out)
	}
}

func TestFacade_LogError_NilErr(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogError(context.Background(), "GET", "/users", 500, nil)

	out := read()
	if strings.Contains(out, `"error"`) {
		t.Errorf("output should not contain error field when err is nil: %q", out)
	}
}

func TestFacade_LogSlow(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LogSlow(context.Background(), "GET", "/heavy", 2*time.Second, 500*time.Millisecond)

	out := read()
	if !strings.Contains(out, "slow request detected") {
		t.Errorf("output missing message: %q", out)
	}
	if !strings.Contains(out, `"level":"WARN"`) {
		t.Errorf("expected WARN level: %q", out)
	}
	if !strings.Contains(out, `"duration_ms":2000`) {
		t.Errorf("output missing duration_ms: %q", out)
	}
	if !strings.Contains(out, `"threshold"`) {
		t.Errorf("output missing threshold: %q", out)
	}
}

func TestFacade_Middleware(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	})

	mw := f.Middleware(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/items", "text/plain", strings.NewReader("body"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, "request started") {
		t.Errorf("output missing start log: %q", out)
	}
	if !strings.Contains(out, "request completed") {
		t.Errorf("output missing completion log: %q", out)
	}
	if !strings.Contains(out, `"status_code":201`) {
		t.Errorf("output missing status_code 201: %q", out)
	}
	if !strings.Contains(out, `"method":"POST"`) {
		t.Errorf("output missing method: %q", out)
	}
}

func TestFacade_Middleware_5xx_ErrorLevel(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	mw := f.Middleware(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/fail")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"level":"ERROR"`) {
		t.Errorf("expected ERROR level for 500: %q", out)
	}
}

func TestNewMiddleware_SkipPaths(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := NewMiddleware(log, Config{
		SkipPaths: []string{"/healthz"},
	})(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if !called {
		t.Error("handler should still be called for skipped paths")
	}
	out := read()
	if strings.Contains(out, "request completed") {
		t.Errorf("skipped path should not be logged: %q", out)
	}
}

func TestNewMiddleware_SkipMethods(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := NewMiddleware(log, Config{
		SkipMethods: []string{"OPTIONS"},
	})(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/anything", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if strings.Contains(out, "request completed") {
		t.Errorf("skipped method should not be logged: %q", out)
	}
}

func TestNewMiddleware_LogBody(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := NewMiddleware(log, Config{
		LogBody:      true,
		MaxBodyBytes: 1024,
	})(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/echo", "text/plain", strings.NewReader("hello-body"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"request_body":"hello-body"`) {
		t.Errorf("output missing request_body: %q", out)
	}
}

func TestNewMiddleware_LogBody_Truncated(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Downstream handler should still see the full body.
		buf := make([]byte, 20)
		n, _ := r.Body.Read(buf)
		if n != 20 {
			t.Errorf("downstream handler expected 20 bytes, got %d", n)
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := NewMiddleware(log, Config{
		LogBody:      true,
		MaxBodyBytes: 5,
	})(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	longBody := strings.Repeat("a", 20)
	resp, err := http.Post(ts.URL+"/echo", "text/plain", strings.NewReader(longBody))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, "aaaaa...(truncated)") {
		t.Errorf("output missing truncated body: %q", out)
	}
}

func TestNewMiddleware_LogBody_DefaultMax(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// LogBody true but MaxBodyBytes 0 -> should default to 4096.
	mw := NewMiddleware(log, Config{
		LogBody: true,
	})(handler)
	ts := httptest.NewServer(mw)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/echo", "text/plain", strings.NewReader("small"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	out := read()
	if !strings.Contains(out, `"request_body":"small"`) {
		t.Errorf("output missing request_body: %q", out)
	}
}

func TestResponseRecorder_WriteHeaderOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: rec, status: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)
	rw.WriteHeader(http.StatusOK) // should be ignored

	if rw.status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rw.status)
	}
}

func TestResponseRecorder_Write_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: rec, status: http.StatusOK}

	_, _ = rw.Write([]byte("hello"))

	if rw.status != http.StatusOK {
		t.Errorf("expected default status 200, got %d", rw.status)
	}
	if rw.size != 5 {
		t.Errorf("expected size 5, got %d", rw.size)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("underlying recorder code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("underlying body = %q, want %q", rec.Body.String(), "hello")
	}
}

func TestResponseRecorder_Flush(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: rec, status: http.StatusOK}
	// httptest.ResponseRecorder implements http.Flusher.
	rw.Flush()
	// No panic means success.
}
