//go:build echo

// Package echo provides Echo middleware for the enterprise logging framework.
//
// It wraps the main logger with an Echo-compatible middleware that logs each
// HTTP request's method, path, status code, duration, and (optionally) the
// request body. Slow requests exceeding a configurable threshold are logged
// at WARN level.
//
// # Quick Start
//
//	log, _ := logger.New(logger.FromEnv())
//	defer log.Close()
//
//	e := echo.New()
//	e.Use(echoadapter.Middleware(log))
//
// # With Options
//
//	e.Use(echoadapter.Middleware(log,
//	    echoadapter.WithSkipPaths("/healthz", "/metrics"),
//	    echoadapter.WithLogBody(true),
//	    echoadapter.WithSlowThreshold(2*time.Second),
//	))
//
// # Enabling
//
// This adapter is guarded by the "echo" build tag. Install the dependency and
// build with the tag:
//
//	go get github.com/labstack/echo/v4
//	go build -tags echo ./...
package echo

import (
	"bytes"
	"io"
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/helper"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Config configures the behavior of the Echo logging middleware.
type Config struct {
	// SkipPaths are URL paths for which logging is skipped entirely.
	SkipPaths []string
	// LogBody enables logging of the request body (up to MaxBodyBytes).
	LogBody bool
	// MaxBodyBytes is the maximum number of request body bytes to log.
	// If 0 and LogBody is true, a default of 4096 is used.
	MaxBodyBytes int
	// SlowRequestThreshold is the duration above which a request is logged
	// as slow (at WARN level) in addition to the normal completion log.
	// If 0, slow request detection is disabled.
	SlowRequestThreshold time.Duration
}

// Option configures the Echo middleware.
type Option func(*Config)

// WithSkipPaths sets the URL paths to skip during logging.
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.SkipPaths = append(c.SkipPaths, paths...)
	}
}

// WithLogBody enables request body logging up to MaxBodyBytes.
// If maxBytes is 0, a default of 4096 is used.
func WithLogBody(maxBytes int) Option {
	return func(c *Config) {
		c.LogBody = true
		c.MaxBodyBytes = maxBytes
	}
}

// WithSlowThreshold sets the duration above which a request is considered slow.
// If 0, slow request detection is disabled.
func WithSlowThreshold(d time.Duration) Option {
	return func(c *Config) {
		c.SlowRequestThreshold = d
	}
}

// Middleware returns an Echo middleware that logs each request using the
// provided logger. Options can be used to customize skip paths, body logging,
// and slow request detection.
func Middleware(log *logger.Logger, opts ...Option) echo.MiddlewareFunc {
	cfg := Config{}
	for _, opt := range opts {
		opt(&cfg)
	}

	skipPaths := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}

	maxBody := cfg.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 4096
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			path := req.URL.Path

			if skipPaths[path] {
				return next(c)
			}

			start := time.Now()

			// Capture request body if enabled.
			var bodySnippet string
			if cfg.LogBody && req.Body != nil {
				bodyBytes, err := io.ReadAll(req.Body)
				if err == nil {
					if len(bodyBytes) > maxBody {
						bodySnippet = string(bodyBytes[:maxBody]) + "...(truncated)"
					} else {
						bodySnippet = string(bodyBytes)
					}
					// Restore the full body for downstream handlers.
					req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// Log request start.
			startAttrs := []slog.Attr{
				slog.String(string(core.FieldMethod), req.Method),
				slog.String(string(core.FieldPath), path),
				slog.String(string(core.FieldIP), helper.ClientIP(req)),
				slog.String(string(core.FieldUserAgent), req.UserAgent()),
			}
			if bodySnippet != "" {
				startAttrs = append(startAttrs, slog.String(string(core.FieldRequestBody), bodySnippet))
			}
			log.Debug(req.Context(), "request started", startAttrs...)

			// Execute the handler.
			err := next(c)

			duration := time.Since(start)
			status := c.Response().Status

			attrs := []slog.Attr{
				slog.String(string(core.FieldMethod), req.Method),
				slog.String(string(core.FieldPath), path),
				slog.String(string(core.FieldIP), helper.ClientIP(req)),
				slog.String(string(core.FieldUserAgent), req.UserAgent()),
				slog.Int(string(core.FieldStatusCode), status),
				slog.Int64(string(core.FieldResponseSize), c.Response().Size),
				slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
				slog.Duration(string(core.FieldDuration), duration),
			}
			if err != nil {
				attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
			}

			msg := "request completed"
			level := helper.StatusLevel(status)
			switch level {
			case core.LevelError:
				log.Error(req.Context(), msg, attrs...)
			case core.LevelWarn:
				log.Warn(req.Context(), msg, attrs...)
			default:
				log.Info(req.Context(), msg, attrs...)
			}

			// Log slow request if threshold is configured and exceeded.
			if cfg.SlowRequestThreshold > 0 && duration > cfg.SlowRequestThreshold {
				slowAttrs := []slog.Attr{
					slog.String(string(core.FieldMethod), req.Method),
					slog.String(string(core.FieldPath), path),
					slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
					slog.Duration(string(core.FieldDuration), duration),
					slog.Duration("threshold", cfg.SlowRequestThreshold),
				}
				log.Warn(req.Context(), "slow request detected", slowAttrs...)
			}

			return err
		}
	}
}
