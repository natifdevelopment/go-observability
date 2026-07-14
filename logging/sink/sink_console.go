package sink

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
)

// ConsoleSink writes log payloads to stdout or stderr.
//
// This is the Twelve-Factor App compliant sink (logs to stdout, not files).
// Console writes never fail (kernel-guaranteed), making this the ideal
// HA fallback sink.
type ConsoleSink struct {
	out       *os.File
	name      string
	closed    atomic.Bool
	mu        sync.Mutex
}

// NewConsoleSink creates a ConsoleSink that writes to stdout.
func NewConsoleSink() *ConsoleSink {
	return &ConsoleSink{
		out:  os.Stdout,
		name: "console:stdout",
	}
}

// NewConsoleSinkStderr creates a ConsoleSink that writes to stderr.
func NewConsoleSinkStderr() *ConsoleSink {
	return &ConsoleSink{
		out:  os.Stderr,
		name: "console:stderr",
	}
}

// NewConsoleSinkFile creates a ConsoleSink writing to a specific *os.File.
func NewConsoleSinkFile(f *os.File, name string) *ConsoleSink {
	return &ConsoleSink{
		out:  f,
		name: "console:" + name,
	}
}

func (s *ConsoleSink) Write(_ context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.out.Write(payload)
	return err
}

func (s *ConsoleSink) Close() error {
	if s.closed.Swap(true) {
		return nil // already closed (idempotent)
	}
	return nil // don't close os.Stdout/Stderr
}

func (s *ConsoleSink) Name() string {
	return s.name
}
