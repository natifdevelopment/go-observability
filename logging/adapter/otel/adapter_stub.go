//go:build !otel

// Package otel provides OpenTelemetry integration helpers for the
// enterprise logger.
//
// This is the stub file used when the "otel" build tag is NOT enabled.
// All functions are no-ops so that code importing this package compiles
// without the go.opentelemetry.io/otel/trace dependency. External types
// (trace.Span) are replaced with any so the stub has no third-party
// imports.
//
// # Enabling the real implementation
//
// To enable the real OpenTelemetry adapter, build with the "otel" tag
// and add the dependency:
//
//	go get go.opentelemetry.io/otel/trace
//	go build -tags otel
//
// When the tag is active, adapter.go (instead of this file) is compiled
// and the functions emit structured log records with span context.
package otel

import (
	"context"
	"log/slog"

	"github.com/natifdevelopment/go-observability/logging/logger"
)

// SpanLogger is a no-op stub that wraps a logger with span context.
// When the "otel" build tag is enabled, this type carries the
// OpenTelemetry span and attaches trace_id/span_id to log records.
//
// To enable the real implementation:
//
//	go get go.opentelemetry.io/otel/trace
//	go build -tags otel
type SpanLogger struct{}

// NewSpanLogger returns a no-op *SpanLogger stub.
//
// Note: the constructor is named NewSpanLogger (idiomatic Go) because
// the type and function cannot share the name SpanLogger in the same
// package.
//
// To enable the real implementation, build with the "otel" tag:
//
//	go get go.opentelemetry.io/otel/trace
//	go build -tags otel
func NewSpanLogger(log *logger.Logger, span any) *SpanLogger {
	return &SpanLogger{}
}

// Log is a no-op stub for emitting a log record with span context.
func (sl *SpanLogger) Log(ctx context.Context, msg string, attrs ...slog.Attr) {}

// LogSpan is a no-op stub for logging with span context.
//
// To enable the real implementation, build with the "otel" tag:
//
//	go get go.opentelemetry.io/otel/trace
//	go build -tags otel
func LogSpan(log *logger.Logger, span any, msg string, attrs ...slog.Attr) {}

// InjectTraceID is a no-op stub that returns the logger unchanged.
//
// To enable the real implementation, build with the "otel" tag:
//
//	go get go.opentelemetry.io/otel/trace
//	go build -tags otel
func InjectTraceID(ctx context.Context, log *logger.Logger) *logger.Logger {
	return log
}
