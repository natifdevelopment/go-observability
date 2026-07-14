package sink

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// BatchSink collects payloads and flushes them in batches to the inner sink.
//
// HA: reduces syscall count by batching multiple log lines into a single write.
// Flush triggers when:
//   - Batch size is reached (batchSize payloads)
//   - Flush interval elapses (flushEvery)
//   - Flush() is called explicitly (graceful shutdown)
//
// Thread-safe via mutex + condition variable.
type BatchSink struct {
	inner      Sink
	batchSize  int
	flushEvery time.Duration
	mu         sync.Mutex
	buffer     [][]byte
	timer      *time.Timer
	closed     atomic.Bool
	flushCh    chan struct{}
	metrics    *SinkMetricsImpl
}

// BatchSinkOption configures a BatchSink.
type BatchSinkOption func(*BatchSink)

// WithBatchSize sets the batch size.
func WithBatchSize(n int) BatchSinkOption {
	return func(s *BatchSink) { s.batchSize = n }
}

// WithBatchFlushInterval sets the flush interval.
func WithBatchFlushInterval(d time.Duration) BatchSinkOption {
	return func(s *BatchSink) { s.flushEvery = d }
}

// NewBatchSink wraps inner Sink with batching.
func NewBatchSink(inner Sink, opts ...BatchSinkOption) *BatchSink {
	s := &BatchSink{
		inner:      inner,
		batchSize:  100,
		flushEvery: 1 * time.Second,
		flushCh:    make(chan struct{}, 1),
		metrics:    NewSinkMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.timer = time.AfterFunc(s.flushEvery, s.triggerFlush)
	go s.flushLoop()
	return s
}

// flushLoop listens for timer-triggered flushes in a background goroutine.
func (s *BatchSink) flushLoop() {
	for range s.flushCh {
		if s.closed.Load() {
			return
		}
		s.flush(context.Background())
	}
}

func (s *BatchSink) triggerFlush() {
	if s.closed.Load() {
		return
	}
	select {
	case s.flushCh <- struct{}{}:
	default:
	}
}

func (s *BatchSink) Write(ctx context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}

	// Clone payload.
	p := make([]byte, len(payload))
	copy(p, payload)

	s.mu.Lock()
	s.buffer = append(s.buffer, p)
	shouldFlush := len(s.buffer) >= s.batchSize
	s.mu.Unlock()

	if shouldFlush {
		return s.flush(ctx)
	}

	return nil
}

func (s *BatchSink) flush(ctx context.Context) error {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		return nil
	}
	batch := s.buffer
	s.buffer = nil
	s.mu.Unlock()

	// Combine all payloads into one write.
	var buf bytes.Buffer
	for _, p := range batch {
		buf.Write(p)
	}

	err := s.inner.Write(ctx, buf.Bytes())
	if err != nil {
		s.metrics.RecordError()
	} else {
		s.metrics.RecordWrite(uint64(buf.Len()), 0)
	}

	// Reset timer.
	if !s.closed.Load() {
		s.timer.Reset(s.flushEvery)
	}

	return err
}

// Flush forces a flush of the batch.
func (s *BatchSink) Flush(ctx context.Context) error {
	return s.flush(ctx)
}

func (s *BatchSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	close(s.flushCh) // stop flushLoop goroutine
	// Final flush.
	_ = s.flush(context.Background())
	return s.inner.Close()
}

func (s *BatchSink) Name() string {
	return "batch:" + s.inner.Name()
}

// Metrics returns the batch sink's metrics.
func (s *BatchSink) Metrics() SinkMetricsSnapshot {
	return s.metrics.Snapshot()
}
