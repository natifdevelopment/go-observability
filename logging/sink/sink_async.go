package sink

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// AsyncSinkImpl wraps a Sink with asynchronous write via a bounded channel
// and worker goroutine(s).
//
// HA features:
//   - Bounded channel prevents unbounded memory growth.
//   - Backpressure policy: Block / DropOldest / DropNewest.
//   - Flush() drains the channel for graceful shutdown.
//   - onDrop callback for metrics/alerting when payloads are dropped.
//
// Performance:
//   - Write() returns immediately after pushing to channel (non-blocking
//     except for BackpressureBlock policy).
//   - Worker goroutine(s) handle the actual I/O.
type AsyncSinkImpl struct {
	inner      Sink
	queue      chan []byte
	policy     BackpressurePolicy
	workers    int
	onDrop     func(payload []byte)
	metrics    *SinkMetricsImpl
	wg         sync.WaitGroup
	closed     atomic.Bool
	flushMu    sync.Mutex
}

// AsyncSinkOption configures an AsyncSinkImpl.
type AsyncSinkOption func(*AsyncSinkImpl)

// WithAsyncBufferSize sets the channel buffer size.
func WithAsyncBufferSize(size int) AsyncSinkOption {
	return func(s *AsyncSinkImpl) { s.queue = make(chan []byte, size) }
}

// WithAsyncWorkers sets the number of worker goroutines.
func WithAsyncWorkers(n int) AsyncSinkOption {
	return func(s *AsyncSinkImpl) { s.workers = n }
}

// WithBackpressure sets the backpressure policy.
func WithBackpressure(p BackpressurePolicy) AsyncSinkOption {
	return func(s *AsyncSinkImpl) { s.policy = p }
}

// WithOnDrop sets a callback for dropped payloads.
func WithOnDrop(fn func(payload []byte)) AsyncSinkOption {
	return func(s *AsyncSinkImpl) { s.onDrop = fn }
}

// NewAsyncSink wraps inner Sink with async write.
func NewAsyncSink(inner Sink, opts ...AsyncSinkOption) *AsyncSinkImpl {
	s := &AsyncSinkImpl{
		inner:   inner,
		queue:   make(chan []byte, 4096),
		policy:  BackpressureDropNewest,
		workers: 1,
		metrics: NewSinkMetrics(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.startWorkers()
	return s
}

func (s *AsyncSinkImpl) startWorkers() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

func (s *AsyncSinkImpl) worker() {
	defer s.wg.Done()
	ctx := context.Background()
	for payload := range s.queue {
		if err := s.inner.Write(ctx, payload); err != nil {
			s.metrics.RecordError()
		} else {
			s.metrics.RecordWrite(uint64(len(payload)), 0)
		}
	}
}

func (s *AsyncSinkImpl) Write(_ context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}

	// Clone payload to avoid race with caller reusing the buffer.
	p := make([]byte, len(payload))
	copy(p, payload)

	switch s.policy {
	case BackpressureBlock:
		select {
		case s.queue <- p:
			return nil
		default:
			// Would block; fall through to drop logic.
		}
		// Actually block.
		s.queue <- p
		return nil

	case BackpressureDropOldest:
		select {
		case s.queue <- p:
			return nil
		default:
			// Queue full; drop oldest.
			select {
			case <-s.queue: // discard oldest
			default:
			}
			s.metrics.RecordDrop()
			if s.onDrop != nil {
				s.onDrop(p)
			}
			// Try again to push the new one.
			select {
			case s.queue <- p:
				return nil
			default:
				return ErrSinkFull
			}
		}

	case BackpressureDropNewest:
		fallthrough

	default:
		select {
		case s.queue <- p:
			return nil
		default:
			s.metrics.RecordDrop()
			if s.onDrop != nil {
				s.onDrop(p)
			}
			return ErrSinkFull
		}
	}
}

// Flush waits for all queued payloads to be written or ctx cancel.
func (s *AsyncSinkImpl) Flush(ctx context.Context) error {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()

	// Close the queue to signal workers to drain and exit.
	// But we can't close if not closed yet and we want to keep the sink usable.
	// Instead, drain the queue by waiting.
	for {
		if s.QueueDepth() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// QueueDepth returns the current number of payloads in the queue.
func (s *AsyncSinkImpl) QueueDepth() int {
	return len(s.queue)
}

func (s *AsyncSinkImpl) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	close(s.queue)
	s.wg.Wait()
	return s.inner.Close()
}

func (s *AsyncSinkImpl) Name() string {
	return "async:" + s.inner.Name()
}

// Metrics returns the async sink's metrics snapshot.
func (s *AsyncSinkImpl) Metrics() SinkMetricsSnapshot {
	return s.metrics.Snapshot()
}
