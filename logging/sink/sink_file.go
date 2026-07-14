package sink

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// FileSink writes log payloads to a file in append mode.
//
// Security:
//   - File opened with O_APPEND|O_CREATE|O_WRONLY, mode 0640 (not world-readable).
//   - No path traversal check here (done in config validator).
//
// HA:
//   - Write has a configurable timeout (default 2s) to prevent hangs.
//   - If write times out, returns ErrSinkTimeout.
type FileSink struct {
	file        *os.File
	name        string
	writeTimeout time.Duration
	closed      atomic.Bool
	mu          sync.Mutex
}

// FileSinkOption configures a FileSink.
type FileSinkOption func(*FileSink)

// WithFileWriteTimeout sets the write timeout.
func WithFileWriteTimeout(d time.Duration) FileSinkOption {
	return func(s *FileSink) { s.writeTimeout = d }
}

// NewFileSink opens a file for append-only logging.
// The file is created if it doesn't exist, with mode 0640.
// Returns error if the file cannot be opened.
func NewFileSink(path string, opts ...FileSinkOption) (*FileSink, error) {
	if path == "" {
		return nil, errors.New("sink: file path is empty")
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return nil, err
	}

	s := &FileSink{
		file:         f,
		name:         "file:" + path,
		writeTimeout: 2 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// NewFileSinkFromFile wraps an existing *os.File.
// The caller is responsible for file permissions; FileSink will close it on Close().
func NewFileSinkFromFile(f *os.File, name string, opts ...FileSinkOption) *FileSink {
	s := &FileSink{
		file:         f,
		name:         "file:" + name,
		writeTimeout: 2 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *FileSink) Write(ctx context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}

	// Use a timeout context if the caller's ctx has no deadline.
	timeout := s.writeTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	done := make(chan error, 1)
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		_, err := s.file.Write(payload)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrSinkTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *FileSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

func (s *FileSink) Name() string {
	return s.name
}
