//go:build !sentry

// Package sentry provides a Sentry integration hook for the enterprise
// logging framework.
//
// This is the stub file used when the "sentry" build tag is NOT enabled.
// All functions are no-ops so that code importing this package compiles
// without the github.com/getsentry/sentry-go dependency.
//
// # Enabling the real implementation
//
// To enable the real Sentry hook, build with the "sentry" tag and add
// the dependency:
//
//	go get github.com/getsentry/sentry-go
//	go build -tags sentry
//
// When the tag is active, hook.go (instead of this file) is compiled
// and the hook captures error-level log records and sends them to Sentry.
package sentry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// HookConfig configures the SentryHook. See hook.go (build tag "sentry")
// for full documentation.
type HookConfig struct {
	DSN               string
	Service           string
	Environment       string
	Release           string
	MinLevel          core.Level
	SampleRate        float64
	EnableBreadcrumbs bool
	MaxBreadcrumbs    int
	FlushTimeout      time.Duration
}

// HookConfigGlobal holds the Sentry configuration set by the application.
// See hook.go (build tag "sentry") for the implementation.
var HookConfigGlobal HookConfig

// SentryHook is a no-op stub that implements core.Hook.
// When the "sentry" build tag is enabled, this type captures
// error-level log records and sends them to Sentry.
//
// To enable the real implementation:
//
//	go get github.com/getsentry/sentry-go
//	go build -tags sentry
type SentryHook struct {
	cfg HookConfig
}

// NewHook creates a no-op SentryHook stub.
//
// If DSN is empty, the stub is returned without error (graceful
// degradation). If DSN is non-empty, a warning is printed to stderr
// reminding the user to build with the "sentry" tag.
//
// To enable the real implementation:
//
//	go get github.com/getsentry/sentry-go
//	go build -tags sentry
func NewHook(cfg HookConfig) (*SentryHook, error) {
	if cfg.DSN != "" {
		fmt.Fprintln(os.Stderr, "sentry: SENTRY_DSN is set but the 'sentry' build tag is not enabled; build with -tags sentry to activate Sentry integration")
	}
	return &SentryHook{cfg: cfg}, nil
}

// NewHookFromEnv creates a no-op SentryHook stub from HookConfigGlobal or
// environment variables.
//
// To enable the real implementation:
//
//	go get github.com/getsentry/sentry-go
//	go build -tags sentry
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
	return NewHook(cfg)
}

// BeforeWrite is a no-op stub.
func (h *SentryHook) BeforeWrite(ctx context.Context, record slog.Record) (slog.Record, error) {
	return record, nil
}

// AfterWrite is a no-op stub.
func (h *SentryHook) AfterWrite(ctx context.Context, record slog.Record, payload []byte, writeErr error) {
}

// Name returns the hook identifier.
func (h *SentryHook) Name() string {
	return "sentry"
}

// Flush is a no-op stub.
func (h *SentryHook) Flush() {}

// FlushWithDuration is a no-op stub.
func (h *SentryHook) FlushWithDuration(d time.Duration) {}
