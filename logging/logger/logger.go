// Package logger is the main facade for the enterprise logging framework.
//
// It provides a simple, ergonomic API on top of log/slog with enterprise features:
//
//   - Context-first methods: Info(ctx, msg, attrs...), Error(ctx, msg, attrs...)
//   - 7 log levels: Trace, Debug, Info, Warn, Error, Fatal, Panic
//   - Automatic sensitive data masking
//   - Context carrier extraction (trace_id, request_id, user_id, etc.)
//   - Sub-facades: Audit(), Security(), Request(), Database(), External()
//   - Lifecycle: Close(), Flush(), Sync()
//   - Dynamic level: SetLevel()
//
// # Quick Start
//
//	log, err := logger.New(logger.FromEnv())
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
//	log.Info(ctx, "server started",
//	    logger.String("port", "8080"),
//	)
//
//	log.Error(ctx, "database connection failed",
//	    logger.Error(err),
//	    logger.String("host", dbHost),
//	)
//
// # Sub-facades
//
//	log.Audit().Log(ctx, auditRecord)
//	log.Security().LoginFailed(ctx, username, ip)
//	log.Request().LogRequest(ctx, req)
//
// # Dynamic Level
//
//	log.SetLevel(core.LevelDebug) // hot-reload level without restart
package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/natifdevelopment/go-observability/logging/builder"
	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/handler"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// Logger is the main facade for enterprise logging.
// It wraps slog.Logger with context-first methods and sub-facades.
type Logger struct {
	slogger    *slog.Logger
	builderLog *builder.Logger
	closed     atomic.Bool
}

// New creates a Logger from the given config.
// This is the primary entry point for the framework.
func New(cfg *config.Config) (*Logger, error) {
	b := builder.New(builder.WithConfig(cfg))
	bl, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("logger: build failed: %w", err)
	}
	return &Logger{
		slogger:    bl.SLogger(),
		builderLog: bl,
	}, nil
}

// NewWithBuilder creates a Logger from a custom builder.
// Use this for advanced configurations (custom sinks, hooks, filters).
func NewWithBuilder(b *builder.Builder) (*Logger, error) {
	bl, err := b.Build()
	if err != nil {
		return nil, fmt.Errorf("logger: build failed: %w", err)
	}
	return &Logger{
		slogger:    bl.SLogger(),
		builderLog: bl,
	}, nil
}

// FromEnv is a convenience function that loads config from environment variables.
// Returns a *config.Config for use with New().
func FromEnv() *config.Config {
	return config.FromEnv()
}

// --- Context-first logging methods ---

// Trace logs at TRACE level.
func (l *Logger) Trace(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelTrace, msg, attrs...)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelDebug, msg, attrs...)
}

// Info logs at INFO level.
func (l *Logger) Info(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelInfo, msg, attrs...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelWarn, msg, attrs...)
}

// Error logs at ERROR level with optional error attribute.
func (l *Logger) Error(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelError, msg, attrs...)
}

// ErrorWithErr logs at ERROR level, automatically wrapping the error.
func (l *Logger) ErrorWithErr(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	l.log(ctx, core.LevelError, msg, attrs...)
}

// Fatal logs at FATAL level and then calls os.Exit(1).
// Use for unrecoverable errors only.
func (l *Logger) Fatal(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelFatal, msg, attrs...)
	l.Flush()
	os.Exit(1)
}

// FatalWithErr logs at FATAL level with error and then calls os.Exit(1).
func (l *Logger) FatalWithErr(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	l.log(ctx, core.LevelFatal, msg, attrs...)
	l.Flush()
	os.Exit(1)
}

// Panic logs at PANIC level and then panics.
// The panic is recovered by middleware (if installed) or propagates up.
func (l *Logger) Panic(ctx context.Context, msg string, attrs ...slog.Attr) {
	l.log(ctx, core.LevelPanic, msg, attrs...)
	l.Flush()
	panic(msg)
}

// PanicWithErr logs at PANIC level with error and then panics.
func (l *Logger) PanicWithErr(ctx context.Context, msg string, err error, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	l.log(ctx, core.LevelPanic, msg, attrs...)
	l.Flush()
	panic(msg)
}

// log is the internal logging method.
func (l *Logger) log(ctx context.Context, level core.Level, msg string, attrs ...slog.Attr) {
	if l.closed.Load() {
		return // silently drop after close
	}
	// Check if enabled.
	if !l.slogger.Enabled(ctx, slog.Level(level)) {
		return
	}
	// Create record.
	record := slog.NewRecord(time.Now(), slog.Level(level), msg, 0)
	// Add attrs.
	if len(attrs) > 0 {
		record.AddAttrs(attrs...)
	}
	// Handle with context.
	_ = l.slogger.Handler().Handle(ctx, record)
}

// --- With methods (for request-scoped loggers) ---

// With returns a new Logger with the given attributes added to every log entry.
// Useful for request-scoped loggers:
//
//	reqLog := log.With(core.StringAttr("request_id", reqID))
//	reqLog.Info(ctx, "processing request")
func (l *Logger) With(attrs ...slog.Attr) *Logger {
	// Convert slog.Attr to slog args for .With() method.
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return &Logger{
		slogger:    l.slogger.With(args...),
		builderLog: l.builderLog,
	}
}

// WithGroup returns a new Logger with all attributes nested under the group name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		slogger:    l.slogger.WithGroup(name),
		builderLog: l.builderLog,
	}
}

// --- Sub-facades ---

// Audit returns the audit logging facade.
// Returns nil if audit logging is not enabled.
func (l *Logger) Audit() *AuditFacade {
	auditSink := l.builderLog.AuditSink()
	if auditSink == nil {
		return nil
	}
	return &AuditFacade{sink: auditSink}
}

// Security returns the security event logging facade.
func (l *Logger) Security() *SecurityFacade {
	return &SecurityFacade{logger: l}
}

// Request returns the HTTP request logging facade.
func (l *Logger) Request() *RequestFacade {
	return &RequestFacade{logger: l}
}

// --- Lifecycle ---

// Close closes all sinks and flushes buffers.
// Idempotent: safe to call multiple times.
func (l *Logger) Close() error {
	if l.closed.Swap(true) {
		return nil
	}
	return l.builderLog.Close()
}

// Flush flushes any async sinks.
func (l *Logger) Flush() error {
	return l.builderLog.Flush()
}

// SetLevel dynamically changes the log level.
func (l *Logger) SetLevel(level core.Level) {
	l.builderLog.SetLevel(level)
}

// SLogger returns the underlying *slog.Logger for direct use with
// third-party libraries that accept slog.Logger.
func (l *Logger) SLogger() *slog.Logger {
	return l.slogger
}

// Metrics returns the handler metrics for this logger, or nil if unavailable.
func (l *Logger) Metrics() *handler.HandlerMetrics {
	if l.builderLog == nil || l.builderLog.Handler() == nil {
		return nil
	}
	return l.builderLog.Handler().Metrics()
}

// --- Convenience attribute constructors ---

// String creates a string attribute.
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int creates an int attribute.
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Int64 creates an int64 attribute.
func Int64(key string, value int64) slog.Attr {
	return slog.Int64(key, value)
}

// Float64 creates a float64 attribute.
func Float64(key string, value float64) slog.Attr {
	return slog.Float64(key, value)
}

// Bool creates a bool attribute.
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Duration creates a duration attribute.
func Duration(key string, value interface{ String() string }) slog.Attr {
	return slog.String(key, value.String())
}

// Any creates an attribute with any value.
func Any(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

// Error creates an error attribute.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.String(string(core.FieldError), "")
	}
	return slog.String(string(core.FieldError), err.Error())
}

// ErrCode creates an error code attribute.
func ErrCode(code string) slog.Attr {
	return slog.String(string(core.FieldErrorCode), code)
}

// Stacktrace creates a stacktrace attribute.
func Stacktrace() slog.Attr {
	return slog.String(string(core.FieldStacktrace), core.Stacktrace())
}

// --- Sub-facade stubs (full implementations in sub-packages) ---

// AuditFacade provides audit logging methods.
type AuditFacade struct {
	sink *sink.AuditSink
}

// Log writes an audit record with hash chain.
func (a *AuditFacade) Log(ctx context.Context, payload core.AuditPayload) error {
	return a.sink.WriteAudit(ctx, payload)
}

// Verify verifies the audit hash chain integrity.
// Returns the index of the first tampered entry (-1 if all valid).
func (a *AuditFacade) Verify(ctx context.Context) (int, error) {
	return a.sink.Verify(ctx)
}

// SecurityFacade provides security event logging methods.
type SecurityFacade struct {
	logger *Logger
}

// LoginFailed logs a failed login attempt.
func (s *SecurityFacade) LoginFailed(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityLoginFailed)),
	)
	s.logger.Warn(ctx, "security: login failed", attrs...)
}

// LoginSuccess logs a successful login.
func (s *SecurityFacade) LoginSuccess(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), "security.auth.login_success"),
	)
	s.logger.Info(ctx, "security: login success", attrs...)
}

// Logout logs a logout event.
func (s *SecurityFacade) Logout(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), "security.auth.logout"),
	)
	s.logger.Info(ctx, "security: logout", attrs...)
}

// BruteForce logs a brute force detection event.
func (s *SecurityFacade) BruteForce(ctx context.Context, ip string, attemptCount int, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldIP), ip),
		slog.Int(string(core.FieldAttemptCount), attemptCount),
		slog.String(string(core.FieldEventID), string(core.EventSecurityBruteForce)),
	)
	s.logger.Error(ctx, "security: brute force detected", attrs...)
}

// SQLInjection logs a SQL injection attempt.
func (s *SecurityFacade) SQLInjection(ctx context.Context, query, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldQuery), query),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecuritySQLInjection)),
	)
	s.logger.Error(ctx, "security: SQL injection detected", attrs...)
}

// RequestFacade provides HTTP request logging methods.
type RequestFacade struct {
	logger *Logger
}

// LogRequest logs an HTTP request.
func (r *RequestFacade) LogRequest(ctx context.Context, method, path string, status int, duration interface{ String() string }, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMethod), method),
		slog.String(string(core.FieldPath), path),
		slog.Int(string(core.FieldStatusCode), status),
	)
	r.logger.Info(ctx, "request completed", attrs...)
}
