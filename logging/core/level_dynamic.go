package core

import (
	"context"
	"sync/atomic"
)

// DynamicLevel is a level threshold that can be changed at runtime
// without restarting the service. This enables HA operations:
//   - SRE can lower level to DEBUG during incident investigation
//   - Admin API can raise level to WARN during high-load events
//   - SIGHUP handler can reload level from config
//
// Thread-safe via atomic.Pointer[Level]. Reads are lock-free.
// Implements slog.Leveler interface for direct use with slog.HandlerOptions.
type DynamicLevel struct {
	current atomic.Pointer[Level]
}

// NewDynamicLevel creates a DynamicLevel with the given initial level.
func NewDynamicLevel(initial Level) *DynamicLevel {
	l := initial
	dl := &DynamicLevel{}
	dl.current.Store(&l)
	return dl
}

// Get returns the current level threshold.
func (d *DynamicLevel) Get() Level {
	if l := d.current.Load(); l != nil {
		return *l
	}
	return LevelInfo // fallback
}

// Set changes the level threshold. Takes effect immediately for
// subsequent log calls. Safe to call from any goroutine.
func (d *DynamicLevel) Set(l Level) {
	d.current.Store(&l)
}

// Level returns the current level as a slog.Level.
// This implements the slog.Leveler interface, allowing DynamicLevel
// to be used directly in slog.HandlerOptions:
//
//	opts := &slog.HandlerOptions{
//	    Level: dynamicLevel,
//	}
func (d *DynamicLevel) Level() Level {
	return d.Get()
}

// Enabled reports whether the given level meets the current threshold.
// This can be used by custom handlers for level checking.
func (d *DynamicLevel) Enabled(_ context.Context, l Level) bool {
	return l >= d.Get()
}

// String returns the label of the current level (e.g., "INFO").
func (d *DynamicLevel) String() string {
	return LevelLabel(d.Get())
}
