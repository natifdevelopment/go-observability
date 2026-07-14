// Package builder provides a fluent builder for constructing Logger instances.
//
// The builder pattern allows flexible construction with functional options:
//
//	log, err := builder.New(
//	    builder.WithConfig(config.FromEnv()),
//	    builder.WithConsoleSink(),
//	    builder.WithMasking(true),
//	    builder.WithAsync(4096, 2),
//	)
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
// # Builder vs Direct Construction
//
// The builder is the RECOMMENDED way to create a Logger. It handles:
//   - Sink creation (console, file, rotate, multi)
//   - Handler configuration (formatter, masking, hooks, filters)
//   - Async wrapping (if enabled)
//   - Audit sink setup (if enabled)
//   - Validation
//
// Direct construction (manual sink + handler) is for advanced use cases
// where the builder doesn't provide enough flexibility.
package builder

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/handler"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// Logger is the facade that the builder returns.
// It wraps slog.Logger with additional methods (Close, Flush, etc.).
// The actual implementation is in the logging/ facade package, but
// the builder returns a concrete type to avoid import cycles.
type Logger struct {
	slogger  *slog.Logger
	handler  *handler.EnterpriseHandler
	sinks    []sink.Sink
	audit    *sink.AuditSink // optional
	dynLevel *core.DynamicLevel
	closed   atomic.Bool
}

// SLogger returns the underlying *slog.Logger for direct use.
func (l *Logger) SLogger() *slog.Logger {
	return l.slogger
}

// Handler returns the underlying EnterpriseHandler (for dynamic level changes, etc.).
func (l *Logger) Handler() *handler.EnterpriseHandler {
	return l.handler
}

// SetLevel dynamically changes the log level.
func (l *Logger) SetLevel(level core.Level) {
	if l.handler != nil {
		l.handler.SetLevel(level)
	}
}

// Close closes all sinks and the handler.
// Idempotent: safe to call multiple times.
func (l *Logger) Close() error {
	if l.closed.Swap(true) {
		return nil
	}
	var lastErr error
	if l.handler != nil {
		if err := l.handler.Close(); err != nil {
			lastErr = err
		}
	}
	// Close any additional sinks not owned by the handler.
	for _, s := range l.sinks {
		if err := s.Close(); err != nil {
			lastErr = err
		}
	}
	// Close audit sink if present.
	if l.audit != nil {
		if err := l.audit.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// AuditSink returns the audit sink if audit logging is enabled, or nil.
func (l *Logger) AuditSink() *sink.AuditSink {
	return l.audit
}

// Flush flushes any async sinks.
func (l *Logger) Flush() error {
	ctx := context.Background()
	// Flush the primary sink owned by the handler.
	if l.handler != nil && l.handler.Sink() != nil {
		if async, ok := l.handler.Sink().(sink.AsyncSink); ok {
			_ = async.Flush(ctx)
		}
	}
	// Find async extra sinks and flush them.
	for _, s := range l.sinks {
		if async, ok := s.(sink.AsyncSink); ok {
			_ = async.Flush(ctx)
		}
	}
	return nil
}

// --- Builder ---

// Builder constructs a Logger from configuration and options.
type Builder struct {
	cfg          *config.Config
	customSinks  []sink.Sink
	formatter    handler.Formatter
	maskEngine   *core.MaskEngine
	hooks        []core.Hook
	filters      []core.Filter
	dynLevel     *core.DynamicLevel
	extraSinks   []sink.Sink
	skipDefaults bool
}

// Option is a functional option for the Builder.
type Option func(*Builder)

// New creates a new Builder with the given options.
func New(opts ...Option) *Builder {
	b := &Builder{
		cfg: config.DefaultConfig(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// WithConfig sets the configuration.
func WithConfig(cfg *config.Config) Option {
	return func(b *Builder) { b.cfg = cfg }
}

// WithSink adds a custom sink to the builder.
func WithSink(s sink.Sink) Option {
	return func(b *Builder) { b.customSinks = append(b.customSinks, s) }
}

// WithFormatter sets a custom formatter.
func WithFormatter(f handler.Formatter) Option {
	return func(b *Builder) { b.formatter = f }
}

// WithMaskEngine sets a custom mask engine.
func WithMaskEngine(m *core.MaskEngine) Option {
	return func(b *Builder) { b.maskEngine = m }
}

// WithHooks adds hooks to the handler.
func WithHooks(hooks ...core.Hook) Option {
	return func(b *Builder) { b.hooks = append(b.hooks, hooks...) }
}

// WithFilters adds filters to the handler.
func WithFilters(filters ...core.Filter) Option {
	return func(b *Builder) { b.filters = append(b.filters, filters...) }
}

// WithDynamicLevel enables dynamic level threshold.
func WithDynamicLevel() Option {
	return func(b *Builder) { b.dynLevel = core.NewDynamicLevel(core.Level(b.cfg.Level)) }
}

// WithConsoleSink adds a console sink (stdout).
func WithConsoleSink() Option {
	return func(b *Builder) {
		b.customSinks = append(b.customSinks, sink.NewConsoleSink())
	}
}

// WithFileSink adds a file sink.
func WithFileSink(path string) Option {
	return func(b *Builder) {
		if s, err := sink.NewFileSink(path); err == nil {
			b.customSinks = append(b.customSinks, s)
		}
	}
}

// WithRotateSink adds a rotating file sink.
func WithRotateSink(cfg sink.RotateConfig) Option {
	return func(b *Builder) {
		if s, err := sink.NewRotateSink(cfg); err == nil {
			b.customSinks = append(b.customSinks, s)
		}
	}
}

// WithAuditSink adds an audit sink at the given path.
func WithAuditSink(path string) Option {
	return func(b *Builder) {
		b.cfg.AuditEnabled = true
		b.cfg.AuditFile = path
	}
}

// SkipDefaultSinks disables automatic sink creation from config.
// Use this when you want full control over sinks.
func SkipDefaultSinks() Option {
	return func(b *Builder) { b.skipDefaults = true }
}

// Build constructs the Logger from the builder configuration.
func (b *Builder) Build() (*Logger, error) {
	// Validate config.
	if err := b.cfg.Validate(); err != nil {
		return nil, fmt.Errorf("builder: config validation failed: %w", err)
	}

	// Create sinks.
	var sinks []sink.Sink

	if !b.skipDefaults {
		defaultSinks, err := b.createDefaultSinks()
		if err != nil {
			return nil, err
		}
		sinks = append(sinks, defaultSinks...)
	}

	// Add custom sinks.
	sinks = append(sinks, b.customSinks...)

	if len(sinks) == 0 {
		// Fallback to console if no sinks configured.
		sinks = append(sinks, sink.NewConsoleSink())
	}

	// Wrap with multi-sink if more than one.
	var primarySink sink.Sink
	if len(sinks) == 1 {
		primarySink = sinks[0]
	} else {
		primarySink = sink.NewMultiSink(sinks)
	}

	// Wrap with async if enabled.
	if b.cfg.EnableAsync {
		primarySink = sink.NewAsyncSink(primarySink,
			sink.WithAsyncBufferSize(b.cfg.AsyncBufferSize),
			sink.WithAsyncWorkers(b.cfg.AsyncWorkers),
		)
	}

	// Create formatter.
	formatter := b.formatter
	if formatter == nil {
		switch b.cfg.Format {
		case "console":
			if b.cfg.EnableColor {
				formatter = handler.NewConsoleFormatterColor()
			} else {
				formatter = handler.NewConsoleFormatter()
			}
		default:
			formatter = handler.NewJSONFormatter()
		}
	}

	// Create mask engine.
	maskEngine := b.maskEngine
	if maskEngine == nil {
		cfg := core.NewDefaultMaskConfig()
		cfg.Enabled = b.cfg.MaskEnabled
		maskEngine = core.NewMaskEngine(cfg)
	}

	// Create dynamic level if requested.
	dynLevel := b.dynLevel
	if dynLevel == nil {
		dynLevel = core.NewDynamicLevel(core.Level(b.cfg.Level))
	}

	// Build handler options.
	handlerOpts := []handler.HandlerOption{
		handler.WithFormatter(formatter),
		handler.WithMaskEngine(maskEngine),
		handler.WithDynamicLevel(dynLevel),
		handler.WithCaller(b.cfg.EnableCaller),
		handler.WithStacktrace(b.cfg.EnableStacktrace),
		handler.WithStandardFields(
			handler.ServiceFields(
				b.cfg.ServiceName,
				b.cfg.ServiceVersion,
				b.cfg.Environment,
				getHostname(),
			)...,
		),
	}
	if len(b.hooks) > 0 {
		handlerOpts = append(handlerOpts, handler.WithHooks(b.hooks...))
	}
	if len(b.filters) > 0 {
		handlerOpts = append(handlerOpts, handler.WithFilters(b.filters...))
	}

	// Create handler.
	h := handler.NewEnterpriseHandler(primarySink, handlerOpts...)

	// Create slog.Logger.
	slogger := slog.New(h)

	// Create audit sink if configured.
	var auditSink *sink.AuditSink
	if b.cfg.AuditEnabled && b.cfg.AuditFile != "" {
		as, err := sink.NewAuditSink(b.cfg.AuditFile)
		if err != nil {
			return nil, fmt.Errorf("builder: failed to create audit sink: %w", err)
		}
		auditSink = as
	}

	return &Logger{
		slogger:  slogger,
		handler:  h,
		sinks:    b.extraSinks, // extra sinks not owned by handler
		audit:    auditSink,
		dynLevel: dynLevel,
	}, nil
}

// createDefaultSinks creates sinks based on config.Output setting.
func (b *Builder) createDefaultSinks() ([]sink.Sink, error) {
	var sinks []sink.Sink

	switch b.cfg.Output {
	case "console":
		sinks = append(sinks, sink.NewConsoleSink())

	case "file":
		s, err := sink.NewFileSink(b.cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("builder: failed to create file sink: %w", err)
		}
		sinks = append(sinks, s)

	case "both":
		sinks = append(sinks, sink.NewConsoleSink())
		s, err := sink.NewFileSink(b.cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("builder: failed to create file sink: %w", err)
		}
		sinks = append(sinks, s)
	}

	return sinks, nil
}

// getHostname returns the hostname or "unknown" if it can't be determined.
func getHostname() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "unknown"
	}
	return host
}
