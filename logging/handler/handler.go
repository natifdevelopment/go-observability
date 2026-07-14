// Package handler implements log/slog.Handler with enterprise features:
//
//   - JSON and Console formatters (pluggable via Formatter interface)
//   - Automatic sensitive data masking (MaskEngine from core)
//   - Caller info (file:line:function) with PC cache
//   - Stacktrace for ERROR and above
//   - Hook chain (BeforeWrite / AfterWrite)
//   - Filter chain (level filter, sampling, dedup)
//   - Dynamic level threshold (runtime-changeable)
//   - Standard fields (service, version, environment, host)
//   - Carrier extraction from context (trace_id, request_id, user_id, etc.)
//   - Panic capture (PANIC level auto-logs stacktrace before re-panic)
//
// # Architecture
//
// EnterpriseHandler is the slog.Handler implementation. It:
//  1. Checks level filter (static or dynamic)
//  2. Runs filter chain (sampling, dedup, etc.)
//  3. Extracts carrier from context
//  4. Adds standard fields (service, host, etc.)
//  5. Adds caller info (if enabled)
//  6. Adds stacktrace (if ERROR+ and enabled)
//  7. Runs BeforeWrite hooks
//  8. Masks sensitive attrs via MaskEngine
//  9. Formats record via Formatter (JSON or Console)
//  10. Writes payload to Sink
//  11. Runs AfterWrite hooks
//
// # Thread Safety
//
// EnterpriseHandler is thread-safe. All internal state uses atomic ops
// or mutex. The same handler can be shared across goroutines via slog.Logger.
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// Formatter formats a slog.Record into a byte payload for the sink.
type Formatter interface {
	// Format converts a record to bytes (including trailing newline).
	Format(record slog.Record) ([]byte, error)
	// Name returns the formatter name (for debugging).
	Name() string
}

// HandlerOption configures an EnterpriseHandler.
type HandlerOption func(*EnterpriseHandler)

// EnterpriseHandler is a slog.Handler with enterprise features.
type EnterpriseHandler struct {
	sink       sink.Sink
	formatter  Formatter
	maskEngine *core.MaskEngine
	level      slog.Leveler
	dynLevel   *core.DynamicLevel // optional, for runtime level changes
	caller     bool
	stacktrace bool
	stdFields  []slog.Attr
	hooks      []core.Hook
	filters    []core.Filter
	metrics    *HandlerMetrics
	// internal
	attrs  []slog.Attr
	groups []string
}

// HandlerMetrics tracks handler-level metrics.
type HandlerMetrics struct {
	// Reuse sink metrics structure for consistency.
	records  uint64
	filtered uint64
	masked   uint64
	errors   uint64
}

// NewHandlerMetrics creates a new HandlerMetrics.
func NewHandlerMetrics() *HandlerMetrics {
	return &HandlerMetrics{}
}

// NewEnterpriseHandler creates a new EnterpriseHandler.
func NewEnterpriseHandler(s sink.Sink, opts ...HandlerOption) *EnterpriseHandler {
	h := &EnterpriseHandler{
		sink:       s,
		formatter:  NewJSONFormatter(),
		maskEngine: core.NewDefaultMaskEngine(),
		level:      slog.LevelInfo,
		caller:     true,
		stacktrace: true,
		metrics:    NewHandlerMetrics(),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WithFormatter sets the formatter.
func WithFormatter(f Formatter) HandlerOption {
	return func(h *EnterpriseHandler) { h.formatter = f }
}

// WithMaskEngine sets the mask engine.
func WithMaskEngine(m *core.MaskEngine) HandlerOption {
	return func(h *EnterpriseHandler) { h.maskEngine = m }
}

// WithLevel sets the static level threshold.
func WithLevel(l slog.Leveler) HandlerOption {
	return func(h *EnterpriseHandler) { h.level = l }
}

// WithDynamicLevel sets a dynamic level threshold.
func WithDynamicLevel(dl *core.DynamicLevel) HandlerOption {
	return func(h *EnterpriseHandler) {
		h.dynLevel = dl
		h.level = dl // DynamicLevel implements slog.Leveler
	}
}

// WithCaller enables/disables caller info.
func WithCaller(enabled bool) HandlerOption {
	return func(h *EnterpriseHandler) { h.caller = enabled }
}

// WithStacktrace enables/disables stacktrace for ERROR+.
func WithStacktrace(enabled bool) HandlerOption {
	return func(h *EnterpriseHandler) { h.stacktrace = enabled }
}

// WithStandardFields sets fields included in every record.
func WithStandardFields(attrs ...slog.Attr) HandlerOption {
	return func(h *EnterpriseHandler) { h.stdFields = attrs }
}

// WithHooks adds hooks to the handler.
func WithHooks(hooks ...core.Hook) HandlerOption {
	return func(h *EnterpriseHandler) { h.hooks = append(h.hooks, hooks...) }
}

// WithFilters adds filters to the handler.
func WithFilters(filters ...core.Filter) HandlerOption {
	return func(h *EnterpriseHandler) { h.filters = append(h.filters, filters...) }
}

// Enabled implements slog.Handler.
func (h *EnterpriseHandler) Enabled(_ context.Context, level slog.Level) bool {
	// Check dynamic level first (if set).
	if h.dynLevel != nil {
		return level >= h.dynLevel.Get()
	}
	return level >= h.level.Level()
}

// Handle implements slog.Handler.
func (h *EnterpriseHandler) Handle(ctx context.Context, record slog.Record) error {
	atomic.AddUint64(&h.metrics.records, 1)

	// Run filter chain.
	for _, f := range h.filters {
		if !f.Allow(ctx, core.Level(record.Level), record.Message) {
			atomic.AddUint64(&h.metrics.filtered, 1)
			return nil // filtered out
		}
	}

	// Extract carrier from context.
	carrier := core.CarrierFrom(ctx)
	if !carrier.IsZero() {
		carrier.AddAttrsToRecord(&record)
	}

	// Add standard fields.
	if len(h.stdFields) > 0 {
		for _, attr := range h.stdFields {
			record.AddAttrs(attr)
		}
	}

	// Add caller info (if enabled and not already present).
	if h.caller {
		if record.PC != 0 {
			record.AddAttrs(callerAttrFromPC(record.PC))
		} else {
			// Resolve from current call stack, skipping handler/logger internals.
			if attr, ok := callerAttrFromStack(); ok {
				record.AddAttrs(attr)
			}
		}
	}

	// Add stacktrace for ERROR and above.
	if h.stacktrace && record.Level >= slog.LevelError {
		if st := core.Stacktrace(); st != "" {
			record.AddAttrs(slog.String(string(core.FieldStacktrace), st))
		}
	}

	// Run BeforeWrite hooks.
	for _, hook := range h.hooks {
		var err error
		record, err = hook.BeforeWrite(ctx, record)
		if err != nil {
			// Log hook error to stderr but continue.
			atomic.AddUint64(&h.metrics.errors, 1)
			continue
		}
	}

	// Mask sensitive attrs.
	if h.maskEngine != nil {
		record = h.maskEngine.MaskRecord(record)
		atomic.AddUint64(&h.metrics.masked, 1)
	}

	// Apply groups: if groups are set, wrap all record attrs in nested groups.
	if len(h.groups) > 0 {
		// Collect all current record attrs.
		var recordAttrs []slog.Attr
		record.Attrs(func(attr slog.Attr) bool {
			recordAttrs = append(recordAttrs, attr)
			return true
		})
		// Also add handler-level attrs.
		recordAttrs = append(recordAttrs, h.attrs...)

		// Build nested group structure.
		var groupAttr slog.Attr
		for i := len(h.groups) - 1; i >= 0; i-- {
			if i == len(h.groups)-1 {
				groupAttr = slog.Group(h.groups[i], toAnyArgs(recordAttrs)...)
			} else {
				groupAttr = slog.Group(h.groups[i], groupAttr)
			}
		}

		// Rebuild record with only the grouped attr.
		newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
		newRecord.AddAttrs(groupAttr)
		record = newRecord
	} else {
		// No groups: just add handler-level attrs.
		for _, attr := range h.attrs {
			record.AddAttrs(attr)
		}
	}

	// Format the record.
	payload, err := h.formatter.Format(record)
	if err != nil {
		atomic.AddUint64(&h.metrics.errors, 1)
		return err
	}

	// Write to sink.
	if err := h.sink.Write(ctx, payload); err != nil {
		atomic.AddUint64(&h.metrics.errors, 1)
		return err
	}

	// Run AfterWrite hooks.
	for _, hook := range h.hooks {
		hook.AfterWrite(ctx, record, payload, nil)
	}

	return nil
}

// WithAttrs implements slog.Handler.
func (h *EnterpriseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := *h
	newH.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &newH
}

// WithGroup implements slog.Handler.
func (h *EnterpriseHandler) WithGroup(name string) slog.Handler {
	newH := *h
	newH.groups = append(append([]string{}, h.groups...), name)
	return &newH
}

// Close closes the underlying sink.
func (h *EnterpriseHandler) Close() error {
	return h.sink.Close()
}

// Metrics returns the handler metrics.
func (h *EnterpriseHandler) Metrics() *HandlerMetrics {
	return h.metrics
}

// SetLevel dynamically changes the log level (if dynamic level is set).
func (h *EnterpriseHandler) SetLevel(l core.Level) {
	if h.dynLevel != nil {
		h.dynLevel.Set(l)
	}
}

// Sink returns the underlying sink (for flushing, health checks, etc.).
func (h *EnterpriseHandler) Sink() sink.Sink {
	return h.sink
}

// Helper: add standard fields from config.
func ServiceFields(service, version, env, host string) []slog.Attr {
	var attrs []slog.Attr
	if service != "" {
		attrs = append(attrs, slog.String(string(core.FieldService), service))
	}
	if version != "" {
		attrs = append(attrs, slog.String(string(core.FieldServiceVersion), version))
	}
	if env != "" {
		attrs = append(attrs, slog.String(string(core.FieldEnvironment), env))
	}
	if host != "" {
		attrs = append(attrs, slog.String(string(core.FieldHostname), host))
	}
	return attrs
}

// toAnyArgs converts []slog.Attr to []any for slog.Group().
func toAnyArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, a := range attrs {
		args[i] = a
	}
	return args
}

// callerAttrFromPC builds a caller attr from a program counter.
func callerAttrFromPC(pc uintptr) slog.Attr {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return slog.String(string(core.FieldCaller), "unknown")
	}
	file, line := fn.FileLine(pc)
	return slog.String(string(core.FieldCaller), fmt.Sprintf("%s:%d:%s", file, line, fn.Name()))
}

// callerAttrFromStack resolves the caller attr from the current stack,
// skipping logging framework frames.
func callerAttrFromStack() (slog.Attr, bool) {
	const maxFrames = 32
	var pcs [maxFrames]uintptr
	n := runtime.Callers(0, pcs[:])
	if n == 0 {
		return slog.Attr{}, false
	}
	frames := runtime.CallersFrames(pcs[:n])
	skip := true
	for {
		frame, more := frames.Next()
		if skip && strings.Contains(frame.File, "/logger/logging/") {
			if !more {
				break
			}
			continue
		}
		skip = false
		// Also skip the runtime/slog machinery that may sit between the framework and user code.
		if strings.Contains(frame.File, "/log/slog/") || strings.HasPrefix(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}
		return slog.String(string(core.FieldCaller), fmt.Sprintf("%s:%d:%s", frame.File, frame.Line, frame.Function)), true
	}
	return slog.Attr{}, false
}

// Records returns the number of records handled.
func (m *HandlerMetrics) Records() uint64 { return atomic.LoadUint64(&m.records) }

// Filtered returns the number of records filtered out.
func (m *HandlerMetrics) Filtered() uint64 { return atomic.LoadUint64(&m.filtered) }

// Masked returns the number of records that were masked.
func (m *HandlerMetrics) Masked() uint64 { return atomic.LoadUint64(&m.masked) }

// Errors returns the number of errors encountered.
func (m *HandlerMetrics) Errors() uint64 { return atomic.LoadUint64(&m.errors) }
