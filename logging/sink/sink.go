// Package sink provides output destinations for log records.
//
// The Sink interface is the port (in Clean Architecture terms) that
// decouples the logging handler from the actual I/O destination.
// This allows swapping output destinations (console, file, Kafka, etc.)
// without modifying the handler or core logic.
//
// # Available Sinks
//
//   - ConsoleSink: writes to stdout/stderr (Twelve-Factor App compliant)
//   - FileSink: writes to a JSON file with append mode
//   - RotateSink: writes to a rotating file (lumberjack-backed)
//   - MultiSink: fan-out to multiple sinks with failover + circuit breaker
//   - AsyncSink: wraps any sink with async write (bounded channel + worker)
//   - AuditSink: append-only file with SHA256 hash chain (immutable audit log)
//   - BufferedSink: wraps any sink with bufio.Writer for batched syscall
//   - RateLimitSink: wraps any sink with token-bucket rate limiting
//   - RetrySink: wraps any sink with retry + exponential backoff
//   - BatchSink: collects payloads and flushes in batches
//
// # HA Features
//
//   - MultiSink failover: if one sink fails, others continue
//   - Circuit breaker per sink: prevents slow sink from blocking others
//   - Async backpressure: Block/DropOldest/DropNewest policies
//   - Implicit stderr fallback: if ALL sinks fail, log goes to stderr
//   - SinkMetrics: per-sink count of written/dropped/errors
//
// # Thread Safety
//
// All Sink implementations MUST be thread-safe (concurrent Write calls).
// Close MUST be idempotent (safe to call multiple times).
package sink

import (
	"context"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// Sink is the port for writing log payloads to a destination.
//
// Contract:
//   - Thread-safe: Write may be called concurrently from multiple goroutines.
//   - Write MUST NOT panic; return error on failure.
//   - Write SHOULD respect ctx.Done() for long I/O operations (HA: no hang).
//   - Close MUST be idempotent; after Close, Write returns ErrSinkClosed.
//   - payload includes a trailing newline; sink writes it as-is.
type Sink interface {
	// Write writes a payload to the destination.
	// payload includes trailing newline.
	// Returns error if write fails (MUST NOT panic).
	Write(ctx context.Context, payload []byte) error

	// Close closes the sink and flushes any buffers.
	// Idempotent: subsequent calls return nil.
	// After Close, Write returns ErrSinkClosed.
	Close() error

	// Name returns the sink's identifier (for metrics/debugging).
	Name() string
}

// AsyncSink is a Sink with asynchronous write capability.
type AsyncSink interface {
	Sink
	// Flush waits for all queued payloads to be written or ctx cancel.
	// Called during graceful shutdown.
	Flush(ctx context.Context) error
	// QueueDepth returns the number of payloads currently in the queue.
	// For HA observability (alert when queue is near capacity).
	QueueDepth() int
}

// SinkWithErrorHandler adds an error callback to a Sink.
type SinkWithErrorHandler interface {
	Sink
	// OnError registers a callback invoked when Write fails.
	// The callback receives the error and the payload that failed.
	OnError(fn func(err error, payload []byte))
}

// BackpressurePolicy determines behavior when the async queue is full.
type BackpressurePolicy int

const (
	// BackpressureBlock blocks the caller until a slot is available.
	// NOT recommended for HTTP request paths (can block request goroutine).
	BackpressureBlock BackpressurePolicy = iota
	// BackpressureDropOldest drops the oldest queued payload and pushes the new one.
	// Recommended for high-throughput services (keeps recent logs).
	BackpressureDropOldest
	// BackpressureDropNewest drops the new payload when queue is full.
	// Recommended for priority logs like audit (preserves older entries).
	BackpressureDropNewest
)

// FailoverPolicy determines MultiSink behavior when a sink fails.
type FailoverPolicy int

const (
	// FailoverContinue continues writing to other sinks even if one fails.
	// Errors are counted in metrics but not returned. Default for HA.
	FailoverContinue FailoverPolicy = iota
	// FailoverStop stops writing to subsequent sinks on first failure.
	// Returns the first error. Not recommended for production.
	FailoverStop
	// FailoverQuorum requires at least N sinks to succeed.
	// Returns error if fewer than N sinks succeed.
	FailoverQuorum
)

// SinkMetricsSnapshot is a point-in-time snapshot of sink metrics.
type SinkMetricsSnapshot struct {
	Written    uint64
	Dropped    uint64
	Errors     uint64
	Bytes      uint64
	AvgLatency uint64 // nanoseconds
	MaxLatency uint64 // nanoseconds
}

// HealthStatus reports the health of a sink.
type HealthStatus struct {
	Name    string
	Healthy bool
	Error   string
}

// sentinel re-export for convenience
var (
	ErrSinkClosed       = core.ErrSinkClosed
	ErrSinkFull         = core.ErrSinkFull
	ErrSinkTimeout      = core.ErrSinkTimeout
	ErrSinkWrite        = core.ErrSinkWrite
	ErrCircuitBreakerOpen = core.ErrCircuitBreakerOpen
)
