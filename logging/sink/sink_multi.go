package sink

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// circuitBreaker implements a simple circuit breaker for sink health.
//
// States:
//   - closed: writes pass through; errors increment failure count.
//   - open: writes fail immediately with ErrCircuitBreakerOpen;
//     after resetTimeout, transitions to half-open.
//   - half-open: one trial write; success → closed, failure → open.
//
// This prevents a failing sink from slowing down the MultiSink pipeline
// (HA: fast-fail instead of waiting for timeout on every write).
type circuitBreaker struct {
	mu            sync.Mutex
	failures      atomic.Uint64
	failThreshold uint64
	resetTimeout  time.Duration
	state         atomic.Int32 // 0=closed, 1=open, 2=half-open
	openedAt      time.Time
}

const (
	cbClosed   int32 = 0
	cbOpen     int32 = 1
	cbHalfOpen int32 = 2
)

func newCircuitBreaker(threshold uint64, reset time.Duration) *circuitBreaker {
	return &circuitBreaker{
		failThreshold: threshold,
		resetTimeout:  reset,
	}
}

func (cb *circuitBreaker) allow() bool {
	state := cb.state.Load()
	switch state {
	case cbClosed:
		return true
	case cbOpen:
		cb.mu.Lock()
		defer cb.mu.Unlock()
		// Check if reset timeout has elapsed.
		if time.Since(cb.openedAt) >= cb.resetTimeout {
			cb.state.Store(cbHalfOpen)
			return true // trial write
		}
		return false
	case cbHalfOpen:
		return true // allow trial
	default:
		return true
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)
	cb.state.Store(cbClosed)
}

func (cb *circuitBreaker) recordFailure() {
	count := cb.failures.Add(1)
	if count >= cb.failThreshold {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		cb.state.Store(cbOpen)
		cb.openedAt = time.Now()
	}
}

func (cb *circuitBreaker) isOpen() bool {
	return cb.state.Load() == cbOpen
}

// MultiSink writes to multiple sinks with failover and circuit breaker.
//
// HA features:
//   - FailoverContinue (default): if one sink fails, others continue.
//   - Circuit breaker per sink: failing sink is fast-failed after threshold.
//   - Implicit stderr fallback: if ALL sinks fail, payload goes to stderr.
//   - Per-sink metrics: track written/dropped/errors.
type MultiSink struct {
	sinks    []Sink
	breakers []*circuitBreaker
	metrics  []*SinkMetricsImpl
	policy   FailoverPolicy
	quorum   int
	stderr   *ConsoleSink // implicit fallback
	closed   atomic.Bool
}

// MultiSinkOption configures a MultiSink.
type MultiSinkOption func(*MultiSink)

// WithFailoverPolicy sets the failover policy.
func WithFailoverPolicy(p FailoverPolicy) MultiSinkOption {
	return func(m *MultiSink) { m.policy = p }
}

// WithQuorum sets the minimum number of sinks that must succeed (for FailoverQuorum).
func WithQuorum(n int) MultiSinkOption {
	return func(m *MultiSink) { m.quorum = n }
}

// NewMultiSink creates a MultiSink from the given sinks.
func NewMultiSink(sinks []Sink, opts ...MultiSinkOption) *MultiSink {
	m := &MultiSink{
		sinks:    sinks,
		breakers: make([]*circuitBreaker, len(sinks)),
		metrics:  make([]*SinkMetricsImpl, len(sinks)),
		policy:   FailoverContinue,
		stderr:   NewConsoleSinkStderr(),
	}
	for i := range sinks {
		m.breakers[i] = newCircuitBreaker(5, 30*time.Second)
		m.metrics[i] = NewSinkMetrics()
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *MultiSink) Write(ctx context.Context, payload []byte) error {
	if m.closed.Load() {
		return ErrSinkClosed
	}

	successCount := 0
	var firstErr error

	for i, s := range m.sinks {
		// Check circuit breaker.
		if !m.breakers[i].allow() {
			m.metrics[i].RecordError()
			if firstErr == nil {
				firstErr = ErrCircuitBreakerOpen
			}
			continue
		}

		start := time.Now()
		err := s.Write(ctx, payload)
		latency := time.Since(start).Nanoseconds()

		if err != nil {
			m.breakers[i].recordFailure()
			m.metrics[i].RecordError()
			if firstErr == nil {
				firstErr = err
			}
			switch m.policy {
			case FailoverStop:
				return err
			}
			continue
		}

		m.breakers[i].recordSuccess()
		m.metrics[i].RecordWrite(uint64(len(payload)), uint64(latency))
		successCount++
	}

	// Check quorum.
	if m.policy == FailoverQuorum && successCount < m.quorum {
		// Fallback to stderr.
		_ = m.stderr.Write(ctx, payload)
		return fmt.Errorf("quorum not met: %d/%d sinks succeeded (need %d)", successCount, len(m.sinks), m.quorum)
	}

	// If ALL sinks failed, fallback to stderr (HA: log never disappears).
	if successCount == 0 && len(m.sinks) > 0 {
		_ = m.stderr.Write(ctx, payload)
		return firstErr
	}

	return nil
}

func (m *MultiSink) Close() error {
	if m.closed.Swap(true) {
		return nil
	}
	var lastErr error
	for _, s := range m.sinks {
		if err := s.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (m *MultiSink) Name() string {
	return "multi"
}

// Metrics returns per-sink metrics snapshots.
func (m *MultiSink) Metrics() []SinkMetricsSnapshot {
	result := make([]SinkMetricsSnapshot, len(m.metrics))
	for i, met := range m.metrics {
		result[i] = met.Snapshot()
	}
	return result
}

// Health returns per-sink health status.
func (m *MultiSink) Health() []HealthStatus {
	result := make([]HealthStatus, len(m.sinks))
	for i := range m.sinks {
		status := HealthStatus{Name: m.sinks[i].Name()}
		if m.breakers[i].isOpen() {
			status.Healthy = false
			status.Error = "circuit breaker open"
		} else {
			status.Healthy = true
		}
		result[i] = status
	}
	return result
}

// SinkMetricsImpl is a thread-safe metrics counter for a sink.
type SinkMetricsImpl struct {
	written    atomic.Uint64
	dropped    atomic.Uint64
	errors     atomic.Uint64
	bytes      atomic.Uint64
	latencySum atomic.Uint64
	latencyMax atomic.Uint64
}

// NewSinkMetrics creates a new SinkMetricsImpl.
func NewSinkMetrics() *SinkMetricsImpl {
	return &SinkMetricsImpl{}
}

// RecordWrite records a successful write.
func (m *SinkMetricsImpl) RecordWrite(bytes, latencyNs uint64) {
	m.written.Add(1)
	m.bytes.Add(bytes)
	m.latencySum.Add(latencyNs)
	for {
		current := m.latencyMax.Load()
		if latencyNs <= current {
			break
		}
		if m.latencyMax.CompareAndSwap(current, latencyNs) {
			break
		}
	}
}

// RecordDrop records a dropped payload (async overflow).
func (m *SinkMetricsImpl) RecordDrop() {
	m.dropped.Add(1)
}

// RecordError records a write error.
func (m *SinkMetricsImpl) RecordError() {
	m.errors.Add(1)
}

// Snapshot returns a point-in-time snapshot of the metrics.
func (m *SinkMetricsImpl) Snapshot() SinkMetricsSnapshot {
	written := m.written.Load()
	sum := m.latencySum.Load()
	var avg uint64
	if written > 0 {
		avg = sum / written
	}
	return SinkMetricsSnapshot{
		Written:    written,
		Dropped:    m.dropped.Load(),
		Errors:     m.errors.Load(),
		Bytes:      m.bytes.Load(),
		AvgLatency: avg,
		MaxLatency: m.latencyMax.Load(),
	}
}

// stderrFallback is used internally for HA fallback.
var _ = os.Stderr // ensure os import is used
