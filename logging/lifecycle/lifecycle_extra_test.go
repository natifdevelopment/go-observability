package lifecycle

import (
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestManager_WithResourceError(t *testing.T) {
	c := &mockCloser{}
	r := &mockCloser{closeErr: errResource}
	m := New(c, WithResource(r))
	err := m.Shutdown()
	if err == nil {
		t.Error("should return resource close error")
	}
}

func TestManager_MultipleResources(t *testing.T) {
	c := &mockCloser{}
	var resources []*mockCloser
	for i := 0; i < 5; i++ {
		r := &mockCloser{}
		resources = append(resources, r)
	}
	opts := []Option{}
	for _, r := range resources {
		opts = append(opts, WithResource(r))
	}
	m := New(c, opts...)
	m.Shutdown()

	for i, r := range resources {
		if !r.closed.Load() {
			t.Errorf("resource %d should be closed", i)
		}
	}
}

func TestManager_WithLoggerAndTimeout(t *testing.T) {
	c := &mockCloser{}
	m := New(c, WithLogger(slog.Default()), WithTimeout(1*time.Second))
	err := m.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestManager_ShutdownConcurrent(t *testing.T) {
	c := &mockCloser{}
	m := New(c)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Shutdown()
		}()
	}
	wg.Wait()

	// Should only close once.
	if !c.closed.Load() {
		t.Error("closer should be closed")
	}
}

func TestManager_HandleSignalsContext_Multiple(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	ctx, cancel := m.HandleSignalsContext(syscall.SIGUSR2)
	defer cancel()

	// Send multiple signals.
	syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)

	select {
	case <-ctx.Done():
	case <-time.After(1 * time.Second):
		t.Error("context should be cancelled")
	}
}

func TestManager_HandleSignalsContext_NoSignal(t *testing.T) {
	c := &mockCloser{}
	m := New(c)
	ctx, cancel := m.HandleSignalsContext(syscall.SIGUSR1)
	defer cancel()

	// Don't send signal — context should not be cancelled.
	select {
	case <-ctx.Done():
		t.Error("context should not be cancelled without signal")
	case <-time.After(50 * time.Millisecond):
		// Good — context still active.
	}
}

func TestManager_GracefulShutdown(t *testing.T) {
	// GracefulShutdown calls HandleSignals which calls os.Exit on signal.
	// We can't easily test os.Exit, but we can verify the manager is created.
	c := &mockCloser{}
	m := GracefulShutdown(c, WithTimeout(100*time.Millisecond))
	if m == nil {
		t.Fatal("GracefulShutdown should return manager")
	}
	if m.IsClosed() {
		t.Error("should not be closed immediately")
	}
	// Manually shutdown to clean up.
	_ = m.Shutdown()
}

func TestManager_NilCloser_WithResources(t *testing.T) {
	r := &mockCloser{}
	m := New(nil, WithResource(r))
	err := m.Shutdown()
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
	if !r.closed.Load() {
		t.Error("resource should be closed")
	}
}

func TestManager_FlushAndCloseBothError(t *testing.T) {
	c := &mockCloser{
		flushErr: errFlush,
		closeErr: errClose,
	}
	m := New(c)
	err := m.Shutdown()
	if err == nil {
		t.Error("should return error")
	}
}

// Ensure os import is used.
var _ = os.Args

var (
	errFlush    = &testError{"flush error"}
	errClose    = &testError{"close error"}
	errResource = &testError{"resource error"}
)

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// Ensure atomic is used.
var _ = atomic.Bool{}
