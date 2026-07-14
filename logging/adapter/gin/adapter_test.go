//go:build gin

package gin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// newTestLogger creates a logger that writes JSON to a temp file and returns
// a cleanup function plus a reader for the file contents.
func newTestLogger(t *testing.T) (*logger.Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "test.log")
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

func newEngine(log *logger.Logger, opts ...Option) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware(log, opts...))
	return r
}

func TestMiddleware_BasicRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log)
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("User-Agent", "test-agent")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	output := read()
	checks := []string{
		"request completed",
		`"method":"GET"`,
		`"path":"/ping"`,
		`"status_code":200`,
		`"user_agent":"test-agent"`,
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Errorf("log output missing %q\noutput: %s", want, output)
		}
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithSkipPaths("/healthz"))
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	// Skipped path should not be logged.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(read(), "request completed") {
		t.Errorf("skip path should not be logged, got: %s", read())
	}

	// Non-skipped path should be logged.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w2, req2)
	if !strings.Contains(read(), "/ping") {
		t.Errorf("non-skip path should be logged")
	}
}

func TestMiddleware_ServerErrorLogsWarn(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log)
	r.GET("/boom", func(c *gin.Context) { c.String(http.StatusInternalServerError, "err") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, `"status_code":500`) {
		t.Errorf("expected status 500 in log: %s", output)
	}
}

func TestMiddleware_SlowRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithSlowThreshold(5*time.Millisecond))
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(20 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, "slow request") {
		t.Errorf("expected slow request log: %s", output)
	}
	if !strings.Contains(output, `"is_slow_request":true`) {
		t.Errorf("expected is_slow_request flag: %s", output)
	}
}

func TestMiddleware_FastRequestNotSlow(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithSlowThreshold(10*time.Second))
	r.GET("/fast", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	r.ServeHTTP(w, req)

	output := read()
	if strings.Contains(output, "slow request") {
		t.Errorf("fast request should not be marked slow: %s", output)
	}
}

func TestMiddleware_LogBody(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithLogBody(1024))
	r.POST("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	body := `{"hello":"world"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, `"request_body":`) {
		t.Errorf("expected request_body field: %s", output)
	}
	// The body is JSON-encoded inside the log record, so quotes are escaped.
	if !strings.Contains(output, `hello`) || !strings.Contains(output, `world`) {
		t.Errorf("expected body content in log: %s", output)
	}
}

func TestMiddleware_LogBodyTruncation(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithLogBody(4))
	r.POST("/echo", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	body := "abcdefghij"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, "truncated") {
		t.Errorf("expected truncation marker: %s", output)
	}
}

func TestMiddleware_LogBodyRestoresBody(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithLogBody(1024))
	var received string
	r.POST("/echo", func(c *gin.Context) {
		buf := make([]byte, 1024)
		n, _ := c.Request.Body.Read(buf)
		received = string(buf[:n])
		c.String(http.StatusOK, "ok")
	})

	body := `{"hello":"world"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(body))
	r.ServeHTTP(w, req)

	if received != body {
		t.Errorf("downstream handler should receive full body, got %q want %q", received, body)
	}
}

func TestMiddleware_NilLoggerPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for nil logger")
		}
	}()
	_ = Middleware(nil)
}

func TestMiddleware_ClientIP(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log)
	r.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, `"ip":"1.2.3.4"`) {
		t.Errorf("expected client IP in log: %s", output)
	}
}

func TestMiddleware_POSTRequest(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log)
	r.POST("/submit", func(c *gin.Context) { c.String(http.StatusCreated, "created") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("data"))
	r.ServeHTTP(w, req)

	output := read()
	if !strings.Contains(output, `"method":"POST"`) {
		t.Errorf("expected POST method: %s", output)
	}
	if !strings.Contains(output, `"status_code":201`) {
		t.Errorf("expected 201 status: %s", output)
	}
}

func TestOptions(t *testing.T) {
	cfg := &Config{}
	WithSkipPaths("/a", "/b")(cfg)
	if len(cfg.SkipPaths) != 2 || cfg.SkipPaths[0] != "/a" || cfg.SkipPaths[1] != "/b" {
		t.Errorf("WithSkipPaths failed: %v", cfg.SkipPaths)
	}

	cfg2 := &Config{}
	WithLogBody(2048)(cfg2)
	if !cfg2.LogBody || cfg2.MaxBodyBytes != 2048 {
		t.Errorf("WithLogBody failed: %+v", cfg2)
	}

	// WithLogBody with non-positive value should enable and keep zero (defaulted later).
	cfg3 := &Config{}
	WithLogBody(0)(cfg3)
	if !cfg3.LogBody {
		t.Errorf("WithLogBody(0) should enable LogBody")
	}

	cfg4 := &Config{}
	WithSlowThreshold(100 * time.Millisecond)(cfg4)
	if cfg4.SlowRequestThreshold != 100*time.Millisecond {
		t.Errorf("WithSlowThreshold failed: %v", cfg4.SlowRequestThreshold)
	}

	// WithSlowThreshold with non-positive value should be ignored.
	cfg5 := &Config{SlowRequestThreshold: defaultSlowThreshold}
	WithSlowThreshold(0)(cfg5)
	if cfg5.SlowRequestThreshold != defaultSlowThreshold {
		t.Errorf("WithSlowThreshold(0) should not override default")
	}
}

func TestMiddleware_DefaultSlowThreshold(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	// No slow threshold option -> default 500ms. A ~0ms request is not slow.
	r := newEngine(log)
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	output := read()
	if strings.Contains(output, "slow request") {
		t.Errorf("fast request with default threshold should not be slow: %s", output)
	}
}

func TestMiddleware_UsesRequestContext(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	type ctxKey struct{}
	var capturedOK bool

	r := newEngine(log)
	r.GET("/ctx", func(c *gin.Context) {
		// Verify the request context is alive and carries values.
		if v := c.Request.Context().Value(ctxKey{}); v != nil {
			capturedOK = true
		}
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ctx", nil)
	ctx := req.Context()
	req = req.WithContext(context.WithValue(ctx, ctxKey{}, "value"))
	r.ServeHTTP(w, req)

	if !capturedOK {
		t.Errorf("request context value should be visible to handler")
	}
	output := read()
	if !strings.Contains(output, "request completed") {
		t.Errorf("expected request log: %s", output)
	}
}

func TestMiddleware_LogBodyReadError(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	r := newEngine(log, WithLogBody(1024))
	r.POST("/echo", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Use a body reader that errors.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/echo", errReader{})
	r.ServeHTTP(w, req)

	// Should still complete and log the request without the body.
	output := read()
	if !strings.Contains(output, "request completed") {
		t.Errorf("expected request log even on body read error: %s", output)
	}
}

// errReader is an io.ReadCloser that always returns an error.
type errReader struct{}

func (errReader) Read(p []byte) (int, error) { return 0, errRead }
func (errReader) Close() error               { return nil }

var errRead = &readErr{}

type readErr struct{}

func (readErr) Error() string { return "simulated read error" }
