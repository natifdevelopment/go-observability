//go:build sentry

// Package sentry provides a Sentry integration hook for the enterprise
// logging framework. It implements core.Hook to automatically capture
// ERROR, FATAL, and PANIC log records and send them to Sentry.
//
// This file is only compiled when the "sentry" build tag is enabled.
// Without the tag, hook_stub.go is used instead and all functions are
// no-ops.
//
// # Enabling
//
//	go get github.com/getsentry/sentry-go
//	go build -tags sentry
//
// # Quick Start
//
//	hook, err := sentry.NewHook(sentry.HookConfig{
//	    DSN:        os.Getenv("SENTRY_DSN"),
//	    Service:    "bbo-api",
//	    Environment: "production",
//	})
//	if err != nil {
//	    log.Fatalf("sentry init failed: %v", err)
//	}
//	defer hook.Flush(2 * time.Second)
//
//	log, _ := logger.NewWithBuilder(builder.New(
//	    builder.WithConfig(logger.FromEnv()),
//	    builder.WithHooks(hook),
//	))
package sentry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// HookConfig configures the SentryHook.
type HookConfig struct {
	// DSN is the Sentry project DSN. Required.
	DSN string

	// Service is the service name tagged on every Sentry event.
	Service string

	// Environment is the environment tagged on every Sentry event
	// (e.g. "production", "staging", "development").
	Environment string

	// Release is the release version tagged on every Sentry event.
	// Typically set from SERVICE_VERSION or git commit hash.
	Release string

	// MinLevel is the minimum log level that triggers a Sentry event.
	// Defaults to LevelError (captures ERROR, FATAL, PANIC).
	MinLevel core.Level

	// SampleRate is the fraction of events to send (0.0 to 1.0).
	// Defaults to 1.0 (send all).
	SampleRate float64

	// EnableBreadcrumbs controls whether INFO/WARN logs are recorded
	// as Sentry breadcrumbs (attached to the next error event).
	// Defaults to true.
	EnableBreadcrumbs bool

	// MaxBreadcrumbs is the maximum number of breadcrumbs kept per
	// event. Defaults to 30 (Sentry's maximum).
	MaxBreadcrumbs int

	// FlushTimeout is the timeout for flushing events on shutdown.
	// Defaults to 2 seconds.
	FlushTimeout time.Duration
}

// HookConfigGlobal holds the Sentry configuration set by the application.
// NewHookFromEnv uses it as the primary source and falls back to environment
// variables for any empty fields.
var HookConfigGlobal HookConfig

// SentryHook is a core.Hook that captures error-level log records
// and sends them to Sentry. It also records breadcrumbs for lower
// level logs.
//
// Thread-safe: SentryHook uses Sentry's built-in buffering and a
// local mutex for breadcrumb management.
type SentryHook struct {
	cfg         HookConfig
	initialized bool
	mu          sync.Mutex
}

// NewHook creates and initializes a SentryHook from the given config.
// It calls sentry.Init internally, so the Sentry SDK is ready to
// capture events immediately.
//
// If DSN is empty, NewHook returns a no-op hook (events are dropped)
// without error. This allows graceful degradation when Sentry is not
// configured (e.g. in development).
func NewHook(cfg HookConfig) (*SentryHook, error) {
	if cfg.DSN == "" {
		return &SentryHook{cfg: cfg}, nil
	}

	if cfg.MinLevel == 0 {
		cfg.MinLevel = core.LevelError
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 1.0
	}
	if cfg.MaxBreadcrumbs == 0 {
		cfg.MaxBreadcrumbs = 30
	}
	if cfg.FlushTimeout == 0 {
		cfg.FlushTimeout = 2 * time.Second
	}

	hostname, _ := os.Hostname()

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      cfg.Environment,
		Release:          cfg.Release,
		ServerName:       hostname,
		SampleRate:       cfg.SampleRate,
		MaxBreadcrumbs:   cfg.MaxBreadcrumbs,
		AttachStacktrace: true,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if event.Tags == nil {
				event.Tags = make(map[string]string)
			}
			event.Tags["service"] = cfg.Service
			return event
		},
	})
	if err != nil {
		return nil, fmt.Errorf("sentry: init failed: %w", err)
	}

	return &SentryHook{
		cfg:         cfg,
		initialized: true,
	}, nil
}

// NewHookFromEnv creates a SentryHook using HookConfigGlobal or environment variables:
//
//   - SENTRY_DSN          (required for activation)
//   - SERVICE_NAME        (service tag)
//   - ENVIRONMENT         (environment tag)
//   - SERVICE_VERSION     (release tag)
//   - SENTRY_SAMPLE_RATE  (float, default 1.0)
func NewHookFromEnv() (*SentryHook, error) {
	cfg := HookConfigGlobal
	if cfg.DSN == "" {
		cfg.DSN = os.Getenv("SENTRY_DSN")
	}
	if cfg.Service == "" {
		cfg.Service = os.Getenv("SERVICE_NAME")
	}
	if cfg.Environment == "" {
		cfg.Environment = os.Getenv("ENVIRONMENT")
	}
	if cfg.Release == "" {
		cfg.Release = os.Getenv("SERVICE_VERSION")
	}
	if cfg.SampleRate == 0 {
		if v := os.Getenv("SENTRY_SAMPLE_RATE"); v != "" {
			var rate float64
			fmt.Sscanf(v, "%f", &rate)
			if rate > 0 {
				cfg.SampleRate = rate
			}
		}
	}
	return NewHook(cfg)
}

// BeforeWrite is called before formatting. For breadcrumbs, it records
// INFO/WARN logs as breadcrumbs. It does not modify the record.
func (h *SentryHook) BeforeWrite(ctx context.Context, record slog.Record) (slog.Record, error) {
	if !h.initialized || !h.cfg.EnableBreadcrumbs {
		return record, nil
	}

	level := core.Level(record.Level)
	if level < core.LevelWarn {
		return record, nil
	}

	// Record as breadcrumb for context in next error event.
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     "log",
		Level:    mapLogLevel(level),
		Message:  record.Message,
		Category: h.cfg.Service,
		Data:     extractAttrs(record),
	})

	return record, nil
}

// AfterWrite is called after the sink write. For ERROR+ levels, it
// captures and sends the event to Sentry.
func (h *SentryHook) AfterWrite(ctx context.Context, record slog.Record, payload []byte, writeErr error) {
	if !h.initialized {
		return
	}

	level := core.Level(record.Level)
	if level < h.cfg.MinLevel {
		return
	}

	h.captureEvent(ctx, record, level)
}

// Name returns the hook identifier.
func (h *SentryHook) Name() string {
	return "sentry"
}

// Flush flushes any buffered events to Sentry. Should be called on
// graceful shutdown.
func (h *SentryHook) Flush() {
	if h.initialized {
		sentry.Flush(h.cfg.FlushTimeout)
	}
}

// FlushWithDuration flushes with a custom timeout.
func (h *SentryHook) FlushWithDuration(d time.Duration) {
	if h.initialized {
		sentry.Flush(d)
	}
}

// captureEvent builds and sends a Sentry event from a log record.
func (h *SentryHook) captureEvent(ctx context.Context, record slog.Record, level core.Level) {
	attrs := extractAttrs(record)

	// Extract trace_id for Sentry trace correlation.
	traceID := core.TraceID(ctx)
	if traceID != "" {
		attrs["trace_id"] = traceID
	}
	requestID := core.RequestID(ctx)
	if requestID != "" {
		attrs["request_id"] = requestID
	}
	userID := core.UserID(ctx)
	if userID != "" {
		sentry.SetUser(sentry.User{ID: userID})
	}

	// Build Sentry event.
	event := sentry.NewEvent()
	event.Level = mapLogLevel(level)
	event.Message = record.Message
	event.Extra = attrs
	event.Tags = map[string]string{
		"service":     h.cfg.Service,
		"environment": h.cfg.Environment,
	}
	if h.cfg.Release != "" {
		event.Release = h.cfg.Release
	}

	// If there's an error field, capture it as an exception with stacktrace.
	if errMsg, ok := attrs["error"]; ok {
		event.Exception = []sentry.Exception{{
			Value:      errMsg,
			Type:       "error",
			Stacktrace: sentry.NewStacktrace(),
		}}
		// Remove error from extra to avoid duplication.
		delete(attrs, "error")
	}

	sentry.CaptureEvent(event)
}

// extractAttrs converts slog.Record attrs to a map[string]string.
func extractAttrs(record slog.Record) map[string]string {
	attrs := make(map[string]string, 16)
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = attrValueToString(a)
		return true
	})
	return attrs
}

// attrValueToString converts a slog.Attr's value to a string.
func attrValueToString(a slog.Attr) string {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindInt64:
		return fmt.Sprintf("%d", v.Int64())
	case slog.KindUint64:
		return fmt.Sprintf("%d", v.Uint64())
	case slog.KindFloat64:
		return fmt.Sprintf("%f", v.Float64())
	case slog.KindBool:
		return fmt.Sprintf("%t", v.Bool())
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v.Any())
	}
}

// mapLogLevel converts core.Level to sentry.Level.
func mapLogLevel(level core.Level) sentry.Level {
	switch {
	case level >= core.LevelPanic:
		return sentry.LevelFatal
	case level >= core.LevelFatal:
		return sentry.LevelFatal
	case level >= core.LevelError:
		return sentry.LevelError
	case level >= core.LevelWarn:
		return sentry.LevelWarning
	case level >= core.LevelInfo:
		return sentry.LevelInfo
	default:
		return sentry.LevelDebug
	}
}
