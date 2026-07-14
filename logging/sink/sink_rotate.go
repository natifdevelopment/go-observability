package sink

import (
	"context"
	"sync"
	"sync/atomic"

	"gopkg.in/natefinch/lumberjack.v2"
)

// RotateSink writes to a file with automatic rotation.
// Uses lumberjack for rotation: size-based, with max backups, max age, and compression.
//
// HA: prevents disk from filling up by auto-rotating log files.
// Rotation parameters:
//   - MaxSizeMB: max file size before rotation (default 100MB)
//   - MaxBackups: max number of old log files to keep (default 10)
//   - MaxAgeDays: max days to retain old log files (default 30)
//   - Compress: gzip old log files (default false)
type RotateSink struct {
	writer *lumberjack.Logger
	name   string
	closed atomic.Bool
	mu     sync.Mutex
}

// RotateConfig configures log file rotation.
type RotateConfig struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// NewRotateSink creates a RotateSink with the given rotation config.
func NewRotateSink(cfg RotateConfig) (*RotateSink, error) {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 100
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 10
	}
	if cfg.MaxAgeDays < 0 {
		cfg.MaxAgeDays = 30
	}

	lj := &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	return &RotateSink{
		writer: lj,
		name:   "rotate:" + cfg.Path,
	}, nil
}

func (s *RotateSink) Write(_ context.Context, payload []byte) error {
	if s.closed.Load() {
		return ErrSinkClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.writer.Write(payload)
	return err
}

func (s *RotateSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Close()
}

func (s *RotateSink) Name() string {
	return s.name
}
