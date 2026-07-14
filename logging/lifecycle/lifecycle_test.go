package lifecycle

import (
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// mockCloser implements Closer for testing.
type mockCloser struct {
	flushed atomic.Bool
	closed  atomic.Bool
	flushErr error
	closeErr error
	mu       sync.Mutex
}

func (m *mockCloser) Flush() error {
	m.flushed.Store(true)
	if m.flushErr != nil {
		return m.flushErr
	}
	return nil
}

func (m *mockCloser) Close() error {
	m.closed.Store(true)
	if m.closeErr != nil {
		return m.closeErr
	}
	return nil
}

func TestManager_Shutdown(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	err := m.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if !c.flushed.Load() {
		t.Error("Flush should be called")
	}
	if !c.closed.Load() {
		t.Error("Close should be called")
	}
}

func TestManager_Shutdown_Idempotent(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	_ = m.Shutdown()
	err := m.Shutdown()
	if err != nil {
		t.Errorf("second Shutdown should be idempotent: %v", err)
	}
}

func TestManager_IsClosed(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	if m.IsClosed() {
		t.Error("should not be closed before Shutdown")
	}
	m.Shutdown()
	if !m.IsClosed() {
		t.Error("should be closed after Shutdown")
	}
}

func TestManager_WithTimeout(t *testing.T) {
	c := &slowCloser{delay: 200 * time.Millisecond}
	m := New(c, WithTimeout(50*time.Millisecond))
	err := m.Shutdown()
	if err == nil {
		t.Error("should timeout with slow closer")
	}
}

func TestManager_WithTimeout_EnoughTime(t *testing.T) {
	c := &slowCloser{delay: 10 * time.Millisecond}
	m := New(c, WithTimeout(100*time.Millisecond))
	err := m.Shutdown()
	if err != nil {
		t.Errorf("should not timeout: %v", err)
	}
}

func TestManager_FlushError(t *testing.T) {
	c := &mockCloser{flushErr: errors.New("flush error")}
	m := New(c)
	err := m.Shutdown()
	if err == nil {
		t.Error("should return flush error")
	}
}

func TestManager_CloseError(t *testing.T) {
	c := &mockCloser{closeErr: errors.New("close error")}
	m := New(c)
	err := m.Shutdown()
	if err == nil {
		t.Error("should return close error")
	}
}

func TestManager_WithResource(t *testing.T) {
	c := &mockCloser{}
	r1 := &mockCloser{}
	r2 := &mockCloser{}
	m := New(c, WithResource(r1), WithResource(r2))
	m.Shutdown()

	if !r1.closed.Load() {
		t.Error("resource 1 should be closed")
	}
	if !r2.closed.Load() {
		t.Error("resource 2 should be closed")
	}
	if !c.closed.Load() {
		t.Error("main closer should be closed")
	}
}

func TestManager_WithLogger(t *testing.T) {
	c := &mockCloser{}
	m := New(c, WithLogger(slog.Default()))
	err := m.Shutdown()
	if err != nil {
		t.Errorf("Shutdown with logger failed: %v", err)
	}
}

func TestManager_NilCloser(t *testing.T) {
	m := New(nil)
	err := m.Shutdown()
	if err != nil {
		t.Errorf("Shutdown with nil closer should not error: %v", err)
	}
}

func TestDefaultSignals(t *testing.T) {
	sigs := DefaultSignals()
	if len(sigs) != 2 {
		t.Errorf("expected 2 default signals, got %d", len(sigs))
	}
}

func TestManager_HandleSignalsContext(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	ctx, cancel := m.HandleSignalsContext(syscall.SIGUSR1)
	defer cancel()

	// Send signal to self.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	select {
	case <-ctx.Done():
		// Context was cancelled — good.
	case <-time.After(1 * time.Second):
		t.Error("context should be cancelled after signal")
	}
}

// slowCloser takes a long time to close.
type slowCloser struct {
	delay time.Duration
}

func (s *slowCloser) Flush() error {
	time.Sleep(s.delay)
	return nil
}

func (s *slowCloser) Close() error {
	time.Sleep(s.delay)
	return nil
}
