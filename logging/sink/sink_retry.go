package sink

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// RetrySink wraps a Sink with retry and exponential backoff.
//
// HA: transient errors (network blips to Kafka/Loki) don't cause
// immediate log loss. Retries with jitter prevent thundering herd.
//
// Retry behavior:
//   - MaxAttempts: max retry attempts (default 3)
//   - InitialDelay: first retry delay (default 100ms)
//   - MaxDelay: cap on delay (default 1s)
//   - Multiplier: exponential factor (default 2.0)
//   - Jitter: add randomness to prevent thundering herd (default true)
type RetrySink struct {
	inner     Sink
	policy    RetryPolicy
	closed    atomic.Bool
	metrics   *SinkMetricsImpl
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool
}

// DefaultRetryPolicy returns a sane default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

// NewRetrySink wraps inner Sink with retry.
func NewRetrySink(inner Sink, policy RetryPolicy) *RetrySink {
	if policy.MaxAttempts <= 0 {
		policy = DefaultRetryPolicy()
	}
	return &RetrySink{
		inner:   inner,
		policy:  policy,
		metrics: NewSinkMetrics(),
	}
}

func (s *RetrySink) Write(ctx context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}

	var lastErr error
	delay := s.policy.InitialDelay

	for attempt := 0; attempt < s.policy.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := s.inner.Write(ctx, payload)
		if err == nil {
			s.metrics.RecordWrite(uint64(len(payload)), 0)
			return nil
		}
		lastErr = err
		s.metrics.RecordError()

		// Don't sleep after the last attempt.
		if attempt == s.policy.MaxAttempts-1 {
			break
		}

		// Calculate delay with jitter.
		sleepDelay := delay
		if s.policy.Jitter {
			// Add up to 50% jitter.
			sleepDelay = time.Duration(float64(delay) * (0.5 + 0.5*simpleRand()))
		}

		select {
		case <-time.After(sleepDelay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Exponential backoff.
		delay = time.Duration(float64(delay) * s.policy.Multiplier)
		if delay > s.policy.MaxDelay {
			delay = s.policy.MaxDelay
		}
	}

	return lastErr
}

func (s *RetrySink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	return s.inner.Close()
}

func (s *RetrySink) Name() string {
	return "retry:" + s.inner.Name()
}

// Metrics returns the retry sink's metrics.
func (s *RetrySink) Metrics() SinkMetricsSnapshot {
	return s.metrics.Snapshot()
}

// simpleRand returns a pseudo-random float64 in [0, 1).
// Uses a simple LCG to avoid importing math/rand in the hot path.
var (
	randMu   sync.Mutex
	randSeed uint64 = 1234567890
)

func simpleRand() float64 {
	randMu.Lock()
	randSeed = randSeed*1103515245 + 12345
	// Use the high bits for better randomness.
	r := float64(randSeed>>33) / float64(1<<31)
	randMu.Unlock()
	return r
}

// Ensure math import is used (for MaxDelay comparison).
var _ = math.MaxFloat64
