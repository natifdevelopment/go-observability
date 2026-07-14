// Package request provides HTTP request/response logging middleware and helpers.
//
// It wraps the main logger with convenience methods for recording HTTP request
// lifecycle events (start, completion, error, slow) and an HTTP middleware
// that automatically logs every request flowing through a handler.
//
// # Quick Start
//
//	log, _ := logger.New(logger.FromEnv())
//	defer log.Close()
//
//	reqLog := request.New(log)
//	reqLog.LogRequest(ctx, "GET", "/users", 200, 42*time.Millisecond)
//
//	// Middleware
//	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    w.WriteHeader(http.StatusOK)
//	})
//	server := &http.Server{Handler: reqLog.Middleware(handler)}
//
// # Configurable Middleware
//
//	mw := request.NewMiddleware(log, request.Config{
//	    SkipPaths:   []string{"/healthz", "/metrics"},
//	    SkipMethods: []string{"OPTIONS"},
//	    LogBody:     true,
//	    MaxBodyBytes: 4096,
//	})
//	server := &http.Server{Handler: mw(handler)}
package request

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/helper"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Facade wraps a *logger.Logger and provides HTTP request logging helpers.
// It is safe for concurrent use.
type Facade struct {
	logger *logger.Logger
}

// New creates a new request logging Facade wrapping the provided logger.
// The logger must not be nil.
func New(l *logger.Logger) *Facade {
	return &Facade{logger: l}
}

// LogRequest logs a completed HTTP request at INFO level (or WARN/ERROR for
// 4xx/5xx status codes).
//
// Standard fields recorded: method, path, status_code, duration_ms, duration.
func (f *Facade) LogRequest(ctx context.Context, method, path string, status int, duration time.Duration, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMethod), method),
		slog.String(string(core.FieldPath), path),
		slog.Int(string(core.FieldStatusCode), status),
		slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
		slog.Duration(string(core.FieldDuration), duration),
	)
	msg := "request completed"
	level := helper.StatusLevel(status)
	switch level {
	case core.LevelError:
		f.logger.Error(ctx, msg, attrs...)
	case core.LevelWarn:
		f.logger.Warn(ctx, msg, attrs...)
	default:
		f.logger.Info(ctx, msg, attrs...)
	}
}

// LogRequestStart logs the beginning of an HTTP request at DEBUG level.
//
// Standard fields recorded: method, path.
func (f *Facade) LogRequestStart(ctx context.Context, method, path string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMethod), method),
		slog.String(string(core.FieldPath), path),
	)
	f.logger.Debug(ctx, "request started", attrs...)
}

// LogError logs an HTTP request error at ERROR level, including the error
// message and status code.
//
// Standard fields recorded: method, path, status_code, error.
func (f *Facade) LogError(ctx context.Context, method, path string, status int, err error, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMethod), method),
		slog.String(string(core.FieldPath), path),
		slog.Int(string(core.FieldStatusCode), status),
	)
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	f.logger.Error(ctx, "request error", attrs...)
}

// LogSlow logs a slow HTTP request at WARN level when its duration exceeds the
// configured threshold.
//
// Standard fields recorded: method, path, duration_ms, duration.
func (f *Facade) LogSlow(ctx context.Context, method, path string, duration, threshold time.Duration, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMethod), method),
		slog.String(string(core.FieldPath), path),
		slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
		slog.Duration(string(core.FieldDuration), duration),
		slog.Duration("threshold", threshold),
	)
	f.logger.Warn(ctx, "slow request detected", attrs...)
}

// Middleware returns an http.Handler middleware that logs each request's
// start and completion using the Facade's logger. It uses a default
// configuration (no skipped paths/methods, no body logging).
func (f *Facade) Middleware(next http.Handler) http.Handler {
	return f.middlewareWithConfig(Config{})(next)
}

// Config configures the behavior of the request logging middleware.
type Config struct {
	// SkipPaths are URL paths for which logging is skipped entirely.
	SkipPaths []string
	// SkipMethods are HTTP methods for which logging is skipped entirely.
	SkipMethods []string
	// LogBody enables logging of the request body (up to MaxBodyBytes).
	LogBody bool
	// MaxBodyBytes is the maximum number of request body bytes to log.
	// If 0 and LogBody is true, a default of 4096 is used.
	MaxBodyBytes int
}

// NewMiddleware returns a configurable middleware factory using the provided
// logger and configuration.
func NewMiddleware(l *logger.Logger, cfg Config) func(http.Handler) http.Handler {
	f := New(l)
	return f.middlewareWithConfig(cfg)
}

// middlewareWithConfig returns a middleware factory honoring the given Config.
func (f *Facade) middlewareWithConfig(cfg Config) func(http.Handler) http.Handler {
	skipPaths := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}
	skipMethods := make(map[string]bool, len(cfg.SkipMethods))
	for _, m := range cfg.SkipMethods {
		skipMethods[m] = true
	}
	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 4096
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipPaths[r.URL.Path] || skipMethods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Capture request body if enabled.
			var bodySnippet string
			if cfg.LogBody && r.Body != nil {
				// Read the entire body so we can restore it for downstream handlers.
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					if len(bodyBytes) > maxBody {
						bodySnippet = string(bodyBytes[:maxBody]) + "...(truncated)"
					} else {
						bodySnippet = string(bodyBytes)
					}
					// Restore the full body for downstream handlers.
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// Log request start.
			startAttrs := []slog.Attr{
				slog.String(string(core.FieldMethod), r.Method),
				slog.String(string(core.FieldPath), r.URL.Path),
				slog.String(string(core.FieldIP), helper.ClientIP(r)),
				slog.String(string(core.FieldUserAgent), r.UserAgent()),
			}
			if bodySnippet != "" {
				startAttrs = append(startAttrs, slog.String(string(core.FieldRequestBody), bodySnippet))
			}
			f.logger.Debug(r.Context(), "request started", startAttrs...)

			// Wrap response writer to capture status and size.
			rw := &responseRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)
			attrs := []slog.Attr{
				slog.String(string(core.FieldMethod), r.Method),
				slog.String(string(core.FieldPath), r.URL.Path),
				slog.String(string(core.FieldIP), helper.ClientIP(r)),
				slog.String(string(core.FieldUserAgent), r.UserAgent()),
				slog.Int(string(core.FieldStatusCode), rw.status),
				slog.Int64(string(core.FieldResponseSize), rw.size),
				slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
				slog.Duration(string(core.FieldDuration), duration),
			}
			msg := "request completed"
			level := helper.StatusLevel(rw.status)
			switch level {
			case core.LevelError:
				f.logger.Error(r.Context(), msg, attrs...)
			case core.LevelWarn:
				f.logger.Warn(r.Context(), msg, attrs...)
			default:
				f.logger.Info(r.Context(), msg, attrs...)
			}
		})
	}
}

// responseRecorder wraps http.ResponseWriter to capture the status code and
// the number of bytes written.
type responseRecorder struct {
	http.ResponseWriter
	status      int
	size        int64
	wroteHeader bool
}

// WriteHeader captures the status code then delegates to the embedded writer.
func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written then delegates to the embedded
// writer.
func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += int64(n)
	return n, err
}

// Flush proxies Flush if the underlying writer supports http.Flusher.
func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
