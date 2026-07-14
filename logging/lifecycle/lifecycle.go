// Package lifecycle provides graceful shutdown management for the logging framework.
//
// It ensures that:
//   - All buffered/async logs are flushed before exit
//   - Sinks are closed in the correct order
//   - Audit hash chains are finalized
//   - No log entries are lost during shutdown
//
// # Usage
//
//	lm := lifecycle.New(log, lifecycle.WithTimeout(10*time.Second))
//	defer lm.Shutdown()
//
//	// Or integrate with signal handling:
//	lm.HandleSignals(syscall.SIGINT, syscall.SIGTERM)
package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Closer is the interface for resources that need to be closed during shutdown.
// The logging framework's Logger implements this.
type Closer interface {
	Close() error
	Flush() error
}

// Manager coordinates graceful shutdown of the logging framework.
type Manager struct {
	closer    Closer
	timeout   time.Duration
	logger    *slog.Logger
	closed    atomic.Bool
	mu        sync.Mutex
	resources []Closer // additional resources to close
}

// Option is a functional option for the Manager.
type Option func(*Manager)

// New creates a new lifecycle Manager.
func New(closer Closer, opts ...Option) *Manager {
	m := &Manager{
		closer:  closer,
		timeout: 5 * time.Second,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// WithTimeout sets the shutdown timeout.
func WithTimeout(d time.Duration) Option {
	return func(m *Manager) { m.timeout = d }
}

// WithLogger sets a logger for shutdown events.
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) { m.logger = l }
}

// WithResource adds an additional resource to close during shutdown.
func WithResource(c Closer) Option {
	return func(m *Manager) { m.resources = append(m.resources, c) }
}

// Shutdown gracefully shuts down the logging framework.
// It flushes buffers, closes sinks, and waits for completion.
// Returns an error if shutdown times out.
func (m *Manager) Shutdown() error {
	if m.closed.Swap(true) {
		return nil // already closed
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		var lastErr error

		// Flush first.
		if m.closer != nil {
			if err := m.closer.Flush(); err != nil {
				m.log("shutdown: flush error: %v", err)
				lastErr = err
			}
		}

		// Close additional resources in reverse order.
		for i := len(m.resources) - 1; i >= 0; i-- {
			if err := m.resources[i].Close(); err != nil {
				m.log("shutdown: resource close error: %v", err)
				lastErr = err
			}
		}

		// Close the main closer.
		if m.closer != nil {
			if err := m.closer.Close(); err != nil {
				m.log("shutdown: close error: %v", err)
				lastErr = err
			}
		}

		done <- lastErr
	}()

	select {
	case err := <-done:
		m.log("shutdown: completed successfully")
		return err
	case <-ctx.Done():
		return fmt.Errorf("lifecycle: shutdown timed out after %v", m.timeout)
	}
}

// HandleSignals sets up signal handlers for graceful shutdown.
// When any of the given signals is received, Shutdown() is called.
// Returns a channel that receives the signal that triggered shutdown.
func (m *Manager) HandleSignals(sig ...os.Signal) chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sig...)

	go func() {
		s := <-sigCh
		m.log("shutdown: received signal %v", s)
		_ = m.Shutdown()
		// Propagate the signal to default handler for os.Exit.
		signal.Stop(sigCh)
		os.Exit(0)
	}()

	return sigCh
}

// HandleSignalsContext sets up signal handlers that cancel a context.
// Returns the context and a cancel function.
func (m *Manager) HandleSignalsContext(sig ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, sig...)

	go func() {
		s := <-sigCh
		m.log("shutdown: received signal %v", s)
		cancel()
		_ = m.Shutdown()
		signal.Stop(sigCh)
	}()

	return ctx, cancel
}

// IsClosed returns true if shutdown has been initiated.
func (m *Manager) IsClosed() bool {
	return m.closed.Load()
}

// log logs a message if a logger is set.
func (m *Manager) log(format string, args ...any) {
	if m.logger != nil {
		m.logger.Info(fmt.Sprintf(format, args...))
	}
}

// DefaultSignals returns the default set of signals to handle (SIGINT, SIGTERM).
func DefaultSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

// GracefulShutdown is a convenience function that creates a Manager,
// handles default signals, and returns it for manual shutdown if needed.
func GracefulShutdown(closer Closer, opts ...Option) *Manager {
	m := New(closer, opts...)
	m.HandleSignals(DefaultSignals()...)
	return m
}
