package sink

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimitSink wraps a Sink with token-bucket rate limiting.
//
// HA: protects downstream sinks (Kafka, Loki, Datadog) from log storms.
// If rate is exceeded, payloads are dropped (not queued) to prevent
// memory growth.
//
// Rate semantics:
//   - ratePerSec = 0: unlimited (no rate limiting)
//   - ratePerSec > 0: max N payloads per second
type RateLimitSink struct {
	inner      Sink
	ratePerSec int
	tokens     atomic.Int64
	lastRefill atomic.Int64 // unix nano
	mu         sync.Mutex
	closed     atomic.Bool
	metrics    *SinkMetricsImpl
}

// NewRateLimitSink wraps inner Sink with rate limiting.
// ratePerSec=0 means unlimited.
func NewRateLimitSink(inner Sink, ratePerSec int) *RateLimitSink {
	s := &RateLimitSink{
		inner:      inner,
		ratePerSec: ratePerSec,
		metrics:    NewSinkMetrics(),
	}
	s.tokens.Store(int64(ratePerSec))
	s.lastRefill.Store(time.Now().UnixNano())
	return s
}

func (s *RateLimitSink) refill() {
	if s.ratePerSec <= 0 {
		return // unlimited
	}
	now := time.Now().UnixNano()
	last := s.lastRefill.Load()
	elapsed := now - last
	if elapsed <= 0 {
		return
	}
	// Calculate tokens to add based on elapsed time.
	tokensToAdd := int64(float64(s.ratePerSec) * float64(elapsed) / float64(time.Second))
	if tokensToAdd > 0 {
		s.tokens.Add(tokensToAdd)
		// Cap at ratePerSec (burst = 1 second of tokens).
		if t := s.tokens.Load(); t > int64(s.ratePerSec) {
			s.tokens.Store(int64(s.ratePerSec))
		}
		s.lastRefill.Store(now)
	}
}

func (s *RateLimitSink) Write(ctx context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}
	if s.ratePerSec > 0 {
		s.mu.Lock()
		s.refill()
		if s.tokens.Load() <= 0 {
			s.mu.Unlock()
			s.metrics.RecordDrop()
			return nil // silently drop (rate limited)
		}
		s.tokens.Add(-1)
		s.mu.Unlock()
	}
	err := s.inner.Write(ctx, payload)
	if err != nil {
		s.metrics.RecordError()
	} else {
		s.metrics.RecordWrite(uint64(len(payload)), 0)
	}
	return err
}

func (s *RateLimitSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	return s.inner.Close()
}

func (s *RateLimitSink) Name() string {
	return "ratelimit:" + s.inner.Name()
}

// Metrics returns the rate limit sink's metrics.
func (s *RateLimitSink) Metrics() SinkMetricsSnapshot {
	return s.metrics.Snapshot()
}
