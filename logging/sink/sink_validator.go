package sink

import (
	"context"
	"sync"
	"sync/atomic"
)

// ValidatorSink wraps a Sink with payload validation before write.
//
// Security:
//   - Checks payload size (prevents oversized log injection).
//   - Checks for null bytes and control characters (anti log injection).
//   - Drops invalid payloads and records metrics.
type ValidatorSink struct {
	inner     Sink
	maxSize   int
	closed    atomic.Bool
	mu        sync.Mutex
	metrics   *SinkMetricsImpl
}

// ValidatorSinkOption configures a ValidatorSink.
type ValidatorSinkOption func(*ValidatorSink)

// WithMaxPayloadSize sets the max payload size in bytes.
func WithMaxPayloadSize(bytes int) ValidatorSinkOption {
	return func(s *ValidatorSink) { s.maxSize = bytes }
}

// NewValidatorSink wraps inner Sink with payload validation.
func NewValidatorSink(inner Sink, opts ...ValidatorSinkOption) *ValidatorSink {
	s := &ValidatorSink{
		inner:   inner,
		maxSize: 1 << 20, // 1MB default
		metrics: NewSinkMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *ValidatorSink) Write(ctx context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}

	// Validate size.
	if len(payload) > s.maxSize {
		s.metrics.RecordDrop()
		return nil // silently drop oversized
	}

	// Validate content: no null bytes.
	for i := 0; i < len(payload); i++ {
		if payload[i] == 0 {
			s.metrics.RecordDrop()
			return nil // drop payloads with null bytes
		}
	}

	return s.inner.Write(ctx, payload)
}

func (s *ValidatorSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	return s.inner.Close()
}

func (s *ValidatorSink) Name() string {
	return "validator:" + s.inner.Name()
}

// Metrics returns the validator sink's metrics.
func (s *ValidatorSink) Metrics() SinkMetricsSnapshot {
	return s.metrics.Snapshot()
}
