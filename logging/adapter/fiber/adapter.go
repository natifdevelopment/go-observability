//go:build fiber

// Package fiber provides Fiber middleware for the enterprise logging framework.
//
// It wraps the main logger with a Fiber-compatible middleware that logs each
// HTTP request's method, path, status code, duration, and (optionally) the
// request body. Slow requests exceeding a configurable threshold are logged
// at WARN level.
//
// # Quick Start
//
//	log, _ := logger.New(logger.FromEnv())
//	defer log.Close()
//
//	app := fiber.New()
//	app.Use(fiberadapter.Middleware(log))
//
// # With Options
//
//	app.Use(fiberadapter.Middleware(log,
//	    fiberadapter.WithSkipPaths("/healthz", "/metrics"),
//	    fiberadapter.WithLogBody(true),
//	    fiberadapter.WithSlowThreshold(2*time.Second),
//	))
//
// # Enabling
//
// This adapter is guarded by the "fiber" build tag. Install the dependency and
// build with the tag:
//
//	go get github.com/gofiber/fiber/v2
//	go build -tags fiber ./...
package fiber

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Config configures the behavior of the Fiber logging middleware.
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

// Option configures the Fiber middleware.
type Option func(*Config)

// WithSkipPaths sets the URL paths to skip during logging.
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.SkipPaths = append(c.SkipPaths, paths...)
	}
}

// WithLogBody enables request body logging up to maxBytes.
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

// Middleware returns a Fiber handler that logs each request using the provided
// logger. Options can be used to customize skip paths, body logging, and slow
// request detection.
func Middleware(log *logger.Logger, opts ...Option) fiber.Handler {
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

	return func(c *fiber.Ctx) error {
		path := string(c.Path())

		if skipPaths[path] {
			return c.Next()
		}

		start := time.Now()

		// Capture request body if enabled.
		var bodySnippet string
		if cfg.LogBody {
			bodyBytes := c.Body()
			if len(bodyBytes) > maxBody {
				bodySnippet = string(bodyBytes[:maxBody]) + "...(truncated)"
			} else {
				bodySnippet = string(bodyBytes)
			}
		}

		// Log request start.
		startAttrs := []slog.Attr{
			slog.String(string(core.FieldMethod), c.Method()),
			slog.String(string(core.FieldPath), path),
			slog.String(string(core.FieldIP), c.IP()),
			slog.String(string(core.FieldUserAgent), string(c.Request().Header.UserAgent())),
		}
		if bodySnippet != "" {
			startAttrs = append(startAttrs, slog.String(string(core.FieldRequestBody), bodySnippet))
		}
		log.Debug(c.Context(), "request started", startAttrs...)

		// Execute the handler.
		err := c.Next()

		duration := time.Since(start)
		status := c.Response().StatusCode()

		attrs := []slog.Attr{
			slog.String(string(core.FieldMethod), c.Method()),
			slog.String(string(core.FieldPath), path),
			slog.String(string(core.FieldIP), c.IP()),
			slog.String(string(core.FieldUserAgent), string(c.Request().Header.UserAgent())),
			slog.Int(string(core.FieldStatusCode), status),
			slog.Int64(string(core.FieldResponseSize), int64(len(c.Response().Body()))),
			slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
			slog.Duration(string(core.FieldDuration), duration),
		}
		if err != nil {
			attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
		}

		msg := "request completed"
		level := statusLevel(status)
		switch level {
		case core.LevelError:
			log.Error(c.Context(), msg, attrs...)
		case core.LevelWarn:
			log.Warn(c.Context(), msg, attrs...)
		default:
			log.Info(c.Context(), msg, attrs...)
		}

		// Log slow request if threshold is configured and exceeded.
		if cfg.SlowRequestThreshold > 0 && duration > cfg.SlowRequestThreshold {
			slowAttrs := []slog.Attr{
				slog.String(string(core.FieldMethod), c.Method()),
				slog.String(string(core.FieldPath), path),
				slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
				slog.Duration(string(core.FieldDuration), duration),
				slog.Duration("threshold", cfg.SlowRequestThreshold),
			}
			log.Warn(c.Context(), "slow request detected", slowAttrs...)
		}

		return err
	}
}

// statusLevel returns the appropriate log level for an HTTP status code.
func statusLevel(statusCode int) core.Level {
	switch {
	case statusCode >= 500:
		return core.LevelError
	case statusCode >= 400:
		return core.LevelWarn
	default:
		return core.LevelInfo
	}
}
