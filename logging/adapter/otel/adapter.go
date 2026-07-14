//go:build otel

// Package otel provides OpenTelemetry integration helpers for the
// enterprise logger, bridging OTel span context into structured log
// records.
//
// This file is only compiled when the "otel" build tag is enabled.
// Without the tag, adapter_stub.go is used instead and all functions
// are no-ops.
//
// # Enabling
//
//	go build -tags otel
//
// The go.opentelemetry.io/otel/trace dependency must be available in
// the module (e.g. `go get go.opentelemetry.io/otel/trace`).
package otel

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// SpanLogger wraps a *logger.Logger with an OpenTelemetry span so that
// every log record emitted through it automatically carries the
// trace_id and span_id of the associated span.
//
// Create one with the SpanLogger constructor:
//
//	sl := otel.SpanLogger(log, span)
//	sl.Log(ctx, "processing message")
type SpanLogger struct {
	log  *logger.Logger
	span trace.Span
}

// NewSpanLogger returns a *SpanLogger that wraps the given logger and
// span. Log records emitted via the returned SpanLogger include the
// span's trace_id and span_id as structured fields.
//
// If log or span is nil, the returned SpanLogger's Log method is a
// no-op.
//
// Note: the constructor is named NewSpanLogger (idiomatic Go) because
// the type and function cannot share the name SpanLogger in the same
// package.
func NewSpanLogger(log *logger.Logger, span trace.Span) *SpanLogger {
	return &SpanLogger{log: log, span: span}
}

// Log emits a log record at INFO level with the span's trace_id and
// span_id attached as structured fields. Additional attributes are
// appended after the span context fields.
//
// It is a no-op if the SpanLogger or its underlying logger/span is nil.
func (sl *SpanLogger) Log(ctx context.Context, msg string, attrs ...slog.Attr) {
	if sl == nil || sl.log == nil || sl.span == nil {
		return
	}
	attrs = appendSpanAttrs(sl.span, attrs)
	sl.log.Info(ctx, msg, attrs...)
}

// LogSpan logs a message at INFO level with the given span's trace_id
// and span_id attached as structured fields. This is a one-shot helper
// for cases where a persistent SpanLogger is not needed.
//
// If log or span is nil, the call is a no-op.
func LogSpan(log *logger.Logger, span trace.Span, msg string, attrs ...slog.Attr) {
	if log == nil || span == nil {
		return
	}
	attrs = appendSpanAttrs(span, attrs)
	log.Info(context.Background(), msg, attrs...)
}

// InjectTraceID returns a new *logger.Logger with the trace_id from the
// OpenTelemetry span in ctx attached as a persistent structured field.
// This allows all subsequent log records from the returned logger to
// carry the trace_id for correlation with distributed traces.
//
// If ctx contains no active span, the original logger is returned
// unchanged.
func InjectTraceID(ctx context.Context, log *logger.Logger) *logger.Logger {
	if log == nil {
		return nil
	}
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return log
	}
	traceID := span.SpanContext().TraceID().String()
	if traceID == "" {
		return log
	}
	return log.With(core.TraceIDAttr(traceID))
}

// appendSpanAttrs returns attrs with the span's trace_id and span_id
// prepended. It allocates a new slice so the caller's slice is not
// mutated.
func appendSpanAttrs(span trace.Span, attrs []slog.Attr) []slog.Attr {
	sc := span.SpanContext()
	out := make([]slog.Attr, 0, len(attrs)+2)
	if sc.HasTraceID() {
		out = append(out, slog.String(string(core.FieldTraceID), sc.TraceID().String()))
	}
	if sc.HasSpanID() {
		out = append(out, slog.String("span_id", sc.SpanID().String()))
	}
	out = append(out, attrs...)
	return out
}
