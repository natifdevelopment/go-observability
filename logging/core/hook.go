package core

import (
	"context"
	"log/slog"
)

// Hook is called before and after a log record is written to a sink.
// Hooks enable extensibility without modifying the handler:
//   - Metrics increment (count logs by level)
//   - Alert triggering (send alert on ERROR/FATAL)
//   - Log shipping to external systems
//   - Deduplication
//   - OpenTelemetry span log export
//
// Thread-safety: implementations MUST be thread-safe (called concurrently).
//
// Ordering in the handler pipeline:
//  1. Filter.Allow() — decide whether to log
//  2. Hook.BeforeWrite() — modify/augment record
//  3. MaskEngine.MaskAttrs() — mask sensitive data
//  4. Formatter.Format() — serialize to bytes
//  5. Sink.Write() — write to destination
//  6. Hook.AfterWrite() — post-write side effects
//
// Security note: hooks run BEFORE masking, so they can see raw values.
// Hook implementations must not leak raw sensitive data. If a hook
// exports data externally, it should apply its own masking or only
// export non-sensitive fields.
type Hook interface {
	// BeforeWrite is called before formatting and writing.
	// May mutate the record (add/remove/modify attrs).
	// Return an error to abort writing this record.
	BeforeWrite(ctx context.Context, record slog.Record) (slog.Record, error)

	// AfterWrite is called after Sink.Write completes.
	// Must not mutate the record. Used for side effects (metrics, alerts).
	// writeErr is nil if the write succeeded.
	AfterWrite(ctx context.Context, record slog.Record, payload []byte, writeErr error)

	// Name returns the hook's identifier (for metrics/debugging).
	Name() string
}

// HookChain runs multiple hooks in sequence.
// BeforeWrite: hooks run in order; if any returns an error, the chain stops
// and that error is returned (record is not written).
// AfterWrite: hooks run in order; errors are collected but not returned
// (AfterWrite is best-effort).
type HookChain struct {
	hooks []Hook
}

// NewHookChain creates a HookChain from the given hooks.
func NewHookChain(hooks ...Hook) *HookChain {
	return &HookChain{hooks: hooks}
}

// BeforeWrite runs all hooks' BeforeWrite in order.
func (c *HookChain) BeforeWrite(ctx context.Context, record slog.Record) (slog.Record, error) {
	for _, h := range c.hooks {
		var err error
		record, err = h.BeforeWrite(ctx, record)
		if err != nil {
			return record, err
		}
	}
	return record, nil
}

// AfterWrite runs all hooks' AfterWrite in order (best-effort, errors ignored).
func (c *HookChain) AfterWrite(ctx context.Context, record slog.Record, payload []byte, writeErr error) {
	for _, h := range c.hooks {
		h.AfterWrite(ctx, record, payload, writeErr)
	}
}

// Name returns the chain name.
func (c *HookChain) Name() string {
	return "hook_chain"
}

// Hooks returns the list of hooks in the chain (read-only snapshot).
func (c *HookChain) Hooks() []Hook {
	return c.hooks
}

// NoopHook is a hook that does nothing. Useful as a default or for testing.
type NoopHook struct{}

func (NoopHook) BeforeWrite(_ context.Context, r slog.Record) (slog.Record, error) { return r, nil }
func (NoopHook) AfterWrite(_ context.Context, _ slog.Record, _ []byte, _ error)    {}
func (NoopHook) Name() string                                                       { return "noop" }
