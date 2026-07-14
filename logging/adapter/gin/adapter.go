//go:build gin

// Package gin provides a Gin middleware adapter for the enterprise logging framework.
//
// It logs HTTP request completion with structured fields (method, path, status,
// duration, client IP) and supports optional request body capture and slow
// request detection.
//
// # Usage
//
//	import ginadapter "github.com/natifdevelopment/go-observability/logging/adapter/gin"
//
//	r := gin.New()
//	r.Use(ginadapter.Middleware(log,
//	    ginadapter.WithSkipPaths("/healthz", "/metrics"),
//	    ginadapter.WithLogBody(4096),
//	    ginadapter.WithSlowThreshold(500*time.Millisecond),
//	))
//
// # Build
//
// This adapter requires gin-gonic/gin which is not a default dependency.
// Install and build with the gin tag:
//
//	go get github.com/gin-gonic/gin
//	go build -tags gin
package gin

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Config holds the configuration for the Gin logging middleware.
type Config struct {
	// SkipPaths is a list of exact request paths that should not be logged.
	// Matching is performed against c.Request.URL.Path.
	SkipPaths []string

	// LogBody enables capturing of the request body. When true, the request
	// body is read (up to MaxBodyBytes) and logged under the request_body field.
	LogBody bool

	// MaxBodyBytes is the maximum number of bytes to capture from the request
	// body when LogBody is enabled. Bodies larger than this are truncated and
	// a truncation marker is appended. Defaults to 4096 when LogBody is enabled
	// and MaxBodyBytes is zero.
	MaxBodyBytes int

	// SlowRequestThreshold is the duration above which a request is considered
	// slow and logged at WARN level instead of INFO. Defaults to 500ms.
	SlowRequestThreshold time.Duration
}

// Option configures the Gin middleware Config.
type Option func(*Config)

// WithSkipPaths adds request paths that should be excluded from logging.
// Matching is exact against c.Request.URL.Path.
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.SkipPaths = append(c.SkipPaths, paths...)
	}
}

// WithLogBody enables request body capture up to maxBytes. If maxBytes is
// less than or equal to zero, a default of 4096 bytes is used.
func WithLogBody(maxBytes int) Option {
	return func(c *Config) {
		c.LogBody = true
		if maxBytes > 0 {
			c.MaxBodyBytes = maxBytes
		}
	}
}

// WithSlowThreshold sets the duration above which a request is logged at WARN
// level as a slow request. If d is zero or negative, the default of 500ms is
// used.
func WithSlowThreshold(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.SlowRequestThreshold = d
		}
	}
}

// defaultSlowThreshold is the default duration above which a request is
// considered slow.
const defaultSlowThreshold = 500 * time.Millisecond

// defaultMaxBodyBytes is the default maximum body size captured when LogBody
// is enabled but MaxBodyBytes is not set.
const defaultMaxBodyBytes = 4096

// Middleware returns a Gin middleware that logs request completion using the
// provided logger.
//
// The middleware records the HTTP method, path, status code, duration, and
// client IP for every request (except those matching SkipPaths). When
// WithLogBody is enabled, the request body is captured and restored so that
// downstream handlers can still read it. Requests slower than
// SlowRequestThreshold are logged at WARN level; all others at INFO.
//
// The request context (c.Request.Context()) is used as the context carrier so
// that trace IDs, request IDs, and other context values are extracted by the
// logger automatically.
func Middleware(log *logger.Logger, opts ...Option) gin.HandlerFunc {
	if log == nil {
		panic("gin: Middleware requires a non-nil logger")
	}

	cfg := &Config{
		SlowRequestThreshold: defaultSlowThreshold,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.LogBody && cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = defaultMaxBodyBytes
	}

	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip logging for excluded paths but still execute the handler chain.
		if _, ok := skip[path]; ok {
			c.Next()
			return
		}

		start := time.Now()

		// Optionally capture the request body before downstream handlers run.
		var bodyAttr slog.Attr
		hasBodyAttr := false
		if cfg.LogBody && c.Request != nil && c.Request.Body != nil {
			bodyAttr, hasBodyAttr = captureBody(c, cfg.MaxBodyBytes)
		}

		// Execute the remainder of the handler chain.
		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		ip := c.ClientIP()
		ctx := c.Request.Context()

		attrs := []slog.Attr{
			core.MethodAttr(method),
			core.PathAttr(path),
			core.StatusCodeAttr(status),
			core.IPAttr(ip),
			core.DurationMsAttr(duration.Milliseconds()),
			slog.String(string(core.FieldUserAgent), c.Request.UserAgent()),
			slog.Int64(string(core.FieldRequestSize), c.Request.ContentLength),
		}
		if hasBodyAttr {
			attrs = append(attrs, bodyAttr)
		}

		msg := "request completed"
		if duration > cfg.SlowRequestThreshold {
			msg = "slow request"
			attrs = append(attrs, slog.Bool("is_slow_request", true))
			attrs = append(attrs, slog.String(string(core.FieldDuration), duration.String()))
			log.Warn(ctx, msg, attrs...)
			return
		}

		// Log server errors at WARN, client errors at WARN too but keep INFO
		// for successful responses.
		switch {
		case status >= 500:
			log.Warn(ctx, msg, attrs...)
		default:
			log.Info(ctx, msg, attrs...)
		}
	}
}

// captureBody reads up to maxBytes from the request body, restores the body so
// downstream handlers can still read it, and returns a slog.Attr containing
// the captured (possibly truncated) body.
func captureBody(c *gin.Context, maxBytes int) (slog.Attr, bool) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(maxBytes)+1))
	if err != nil {
		// Restore an empty body so downstream handlers don't see a consumed
		// stream unexpectedly.
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return slog.String(string(core.FieldRequestBody), ""), false
	}

	truncated := false
	if len(body) > maxBytes {
		body = body[:maxBytes]
		truncated = true
	}

	// Restore the body for downstream handlers by concatenating what we read
	// with the remainder of the original stream.
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), c.Request.Body))

	value := string(body)
	if truncated {
		value += "...[truncated]"
	}
	return slog.String(string(core.FieldRequestBody), value), true
}
