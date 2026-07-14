// Package helper provides utility functions for common logging patterns.
//
// These helpers reduce boilerplate for repetitive logging tasks:
//
//   - HTTP request/response logging
//   - Error wrapping with context
//   - Duration tracking
//   - Field extraction from common types
//   - Redaction helpers
//   - Batch attribute construction
package helper

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// --- HTTP Helpers ---

// HTTPRequestAttrs extracts common HTTP request attributes.
func HTTPRequestAttrs(r *http.Request) []slog.Attr {
	attrs := []slog.Attr{
		slog.String(string(core.FieldMethod), r.Method),
		slog.String(string(core.FieldPath), r.URL.Path),
		slog.String(string(core.FieldIP), ClientIP(r)),
		slog.String(string(core.FieldUserAgent), r.UserAgent()),
	}
	if r.Host != "" {
		attrs = append(attrs, slog.String("host", r.Host))
	}
	if r.ContentLength > 0 {
		attrs = append(attrs, slog.Int64("content_length", r.ContentLength))
	}
	if ref := r.Referer(); ref != "" {
		attrs = append(attrs, slog.String("referer", ref))
	}
	return attrs
}

// HTTPResponseAttrs creates response attributes.
func HTTPResponseAttrs(statusCode int, contentLength int64, duration time.Duration) []slog.Attr {
	return []slog.Attr{
		slog.Int(string(core.FieldStatusCode), statusCode),
		slog.Int64("response_length", contentLength),
		slog.Duration(string(core.FieldDuration), duration),
	}
}

// ClientIP extracts the real client IP from request, checking X-Forwarded-For
// and X-Real-IP headers (common behind proxies/load balancers).
func ClientIP(r *http.Request) string {
	// Check X-Forwarded-For (may contain multiple IPs, first is the client).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fallback to RemoteAddr (strip port).
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx > 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// StatusLevel returns the appropriate log level for an HTTP status code.
func StatusLevel(statusCode int) core.Level {
	switch {
	case statusCode >= 500:
		return core.LevelError
	case statusCode >= 400:
		return core.LevelWarn
	case statusCode >= 300:
		return core.LevelInfo
	default:
		return core.LevelInfo
	}
}

// --- Error Helpers ---

// ErrorAttrs creates attributes from an error, including the error message
// and optionally the error type.
func ErrorAttrs(err error) []slog.Attr {
	if err == nil {
		return nil
	}
	attrs := []slog.Attr{
		slog.String(string(core.FieldError), err.Error()),
	}
	// Include error type if it's a custom type.
	typeName := fmt.Sprintf("%T", err)
	if typeName != "*errors.errorString" && typeName != "*fmt.wrapError" {
		attrs = append(attrs, slog.String("error_type", typeName))
	}
	return attrs
}

// ErrorWithStack creates error attributes with a stacktrace.
func ErrorWithStack(err error) []slog.Attr {
	attrs := ErrorAttrs(err)
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldStacktrace), core.Stacktrace()))
	}
	return attrs
}

// --- Duration Helpers ---

// Timer is a simple duration tracker that logs when stopped.
type Timer struct {
	start   time.Time
	logger  *slog.Logger
	ctx     context.Context
	msg     string
	attrs   []slog.Attr
	stopped bool
}

// StartTimer creates a new Timer.
func StartTimer(ctx context.Context, logger *slog.Logger, msg string, attrs ...slog.Attr) *Timer {
	return &Timer{
		start:  time.Now(),
		logger: logger,
		ctx:    ctx,
		msg:    msg,
		attrs:  attrs,
	}
}

// Stop stops the timer and logs the duration.
// Returns the elapsed duration.
func (t *Timer) Stop() time.Duration {
	if t.stopped {
		return 0
	}
	t.stopped = true
	elapsed := time.Since(t.start)
	attrs := append(t.attrs, slog.Duration(string(core.FieldDuration), elapsed))
	if t.logger != nil {
		args := make([]any, len(attrs))
		for i, a := range attrs {
			args[i] = a
		}
		t.logger.InfoContext(t.ctx, t.msg, args...)
	}
	return elapsed
}

// Elapsed returns the elapsed time without stopping the timer.
func (t *Timer) Elapsed() time.Duration {
	return time.Since(t.start)
}

// --- Field Helpers ---

// StringAttrs creates multiple string attributes at once.
func StringAttrs(kv ...string) []slog.Attr {
	if len(kv)%2 != 0 {
		return nil
	}
	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		attrs = append(attrs, slog.String(kv[i], kv[i+1]))
	}
	return attrs
}

// IntAttrs creates multiple int attributes at once.
func IntAttrs(kv ...int) []slog.Attr {
	if len(kv)%2 != 0 {
		return nil
	}
	attrs := make([]slog.Attr, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		attrs = append(attrs, slog.Int(fmt.Sprintf("key_%d", kv[i]), kv[i+1]))
	}
	return attrs
}

// --- Redaction Helpers ---

// RedactHeaders redacts sensitive HTTP headers for logging.
var sensitiveHeaders = map[string]bool{
	"Authorization":       true,
	"Cookie":              true,
	"Set-Cookie":          true,
	"X-Api-Key":           true,
	"X-Auth-Token":        true,
	"Proxy-Authorization": true,
	"X-CSRF-Token":        true,
}

// RedactHeaderValue redacts a sensitive header value, returning "***REDACTED***".
func RedactHeaderValue(key string, value string) string {
	if sensitiveHeaders[http.CanonicalHeaderKey(key)] {
		return "***REDACTED***"
	}
	return value
}

// SafeHeaders returns a map of headers with sensitive values redacted.
func SafeHeaders(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = RedactHeaderValue(key, values[0])
		}
	}
	return result
}

// --- Runtime Helpers ---

// CallerInfo returns the caller's file, line, and function name.
// skip is the number of stack frames to skip (0 = caller of CallerInfo).
func CallerInfo(skip int) (file string, line int, function string) {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", 0, ""
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return file, line, ""
	}
	return file, line, fn.Name()
}

// CallerAttr creates a caller attribute with file:line.
func CallerAttr(skip int) slog.Attr {
	file, line, _ := CallerInfo(skip + 1)
	if file == "" {
		return slog.String(string(core.FieldCaller), "unknown")
	}
	// Trim to just the filename for brevity.
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	return slog.String(string(core.FieldCaller), fmt.Sprintf("%s:%d", file, line))
}

// --- Context Helpers ---

// ContextAttrs extracts all carrier attributes from context.
func ContextAttrs(ctx context.Context) []slog.Attr {
	if ctx == nil {
		return nil
	}
	carrier := core.CarrierFrom(ctx)
	// Check if carrier is empty (all key fields empty).
	if carrier.TraceID == "" && carrier.RequestID == "" && carrier.UserID == "" &&
		carrier.Username == "" && carrier.IP == "" && carrier.SessionID == "" {
		return nil
	}
	attrs := []slog.Attr{}
	if carrier.TraceID != "" {
		attrs = append(attrs, slog.String(string(core.FieldTraceID), carrier.TraceID))
	}
	if carrier.RequestID != "" {
		attrs = append(attrs, slog.String(string(core.FieldRequestID), carrier.RequestID))
	}
	if carrier.UserID != "" {
		attrs = append(attrs, slog.String(string(core.FieldUserID), carrier.UserID))
	}
	if carrier.Username != "" {
		attrs = append(attrs, slog.String(string(core.FieldUsername), carrier.Username))
	}
	if carrier.IP != "" {
		attrs = append(attrs, slog.String(string(core.FieldIP), carrier.IP))
	}
	return attrs
}

// MergeAttrs merges multiple attribute slices.
func MergeAttrs(attrSlices ...[]slog.Attr) []slog.Attr {
	total := 0
	for _, s := range attrSlices {
		total += len(s)
	}
	result := make([]slog.Attr, 0, total)
	for _, s := range attrSlices {
		result = append(result, s...)
	}
	return result
}
