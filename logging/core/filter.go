package core

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Filter determines whether a log record should be written.
// Called early in the handler pipeline (before hooks and masking).
// Enables: sampling, deduplication, dynamic level, conditional dropping.
//
// Thread-safety: implementations MUST be thread-safe.
type Filter interface {
	// Allow returns true if the record should be logged.
	// Called with the context, level, and message (attrs not yet resolved).
	Allow(ctx context.Context, level Level, msg string) bool
}

// FilterChain runs multiple filters with AND logic.
// A record is allowed only if ALL filters return true.
type FilterChain struct {
	filters []Filter
}

// NewFilterChain creates a FilterChain from the given filters.
func NewFilterChain(filters ...Filter) *FilterChain {
	return &FilterChain{filters: filters}
}

// Allow returns true only if all filters allow the record.
func (c *FilterChain) Allow(ctx context.Context, level Level, msg string) bool {
	for _, f := range c.filters {
		if !f.Allow(ctx, level, msg) {
			return false
		}
	}
	return true
}

// LevelFilter allows records at or above the configured minimum level.
// This is the primary filter used by the handler.
type LevelFilter struct {
	Min Level
}

// Allow returns true if level >= Min.
func (f LevelFilter) Allow(_ context.Context, level Level, _ string) bool {
	return level >= f.Min
}

// SamplingFilter allows a fraction of records through (SRE best practice
// for high-volume services). Uses a counter-based approach for determinism.
//
// Rate semantics:
//   - 0.0: no records allowed (effectively disables logging)
//   - 1.0: all records allowed (no sampling)
//   - 0.1: ~10% of records allowed
//
// Note: sampling is applied AFTER level filtering. ERROR/FATAL/PANIC
// should typically NOT be sampled (use a separate filter chain for those).
type SamplingFilter struct {
	Rate    float64
	counter atomic.Uint64
}

// NewSamplingFilter creates a SamplingFilter with the given rate.
// Rate must be between 0.0 and 1.0.
func NewSamplingFilter(rate float64) *SamplingFilter {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &SamplingFilter{Rate: rate}
}

// Allow returns true based on the sampling rate.
func (f *SamplingFilter) Allow(_ context.Context, _ Level, _ string) bool {
	if f.Rate >= 1.0 {
		return true
	}
	if f.Rate <= 0.0 {
		return false
	}
	n := f.counter.Add(1)
	// Deterministic sampling: every Nth record passes.
	threshold := uint64(1.0 / f.Rate)
	return n%threshold == 0
}

// DedupFilter drops records with the same message if they appear more than
// MaxCount times within the Window duration. Prevents log storms from
// runaway error loops (HA: protect downstream sinks).
//
// Uses an LRU-like map with TTL-based eviction to bound memory.
// Thread-safe via sync.Mutex.
type DedupFilter struct {
	Window    time.Duration
	MaxCount  int
	maxSize   int
	mu        sync.Mutex
	entries   map[string]*dedupEntry
	lastClean time.Time
}

type dedupEntry struct {
	count    int
	firstSeen time.Time
	lastSeen  time.Time
}

// NewDedupFilter creates a DedupFilter.
// window: time window for counting (e.g., 5*time.Minute).
// maxCount: max allowed occurrences within the window before dropping.
// maxSize: max number of unique messages tracked (LRU eviction, default 10000).
func NewDedupFilter(window time.Duration, maxCount, maxSize int) *DedupFilter {
	if maxSize <= 0 {
		maxSize = DefaultDedupWindowSize
	}
	return &DedupFilter{
		Window:   window,
		MaxCount: maxCount,
		maxSize:  maxSize,
		entries:  make(map[string]*dedupEntry, maxSize),
	}
}

// Allow returns true if the message has not exceeded MaxCount within Window.
func (f *DedupFilter) Allow(_ context.Context, _ Level, msg string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()
	f.maybeClean(now)

	entry, exists := f.entries[msg]
	if !exists {
		// Evict if at capacity.
		if len(f.entries) >= f.maxSize {
			f.evictOldest()
		}
		f.entries[msg] = &dedupEntry{
			count:     1,
			firstSeen: now,
			lastSeen:  now,
		}
		return true
	}

	// Reset if window expired.
	if now.Sub(entry.firstSeen) > f.Window {
		entry.count = 1
		entry.firstSeen = now
		entry.lastSeen = now
		return true
	}

	entry.count++
	entry.lastSeen = now
	return entry.count <= f.MaxCount
}

// maybeClean removes expired entries periodically.
func (f *DedupFilter) maybeClean(now time.Time) {
	// Clean at most once per minute to avoid overhead.
	if now.Sub(f.lastClean) < time.Minute {
		return
	}
	f.lastClean = now
	for msg, entry := range f.entries {
		if now.Sub(entry.lastSeen) > f.Window {
			delete(f.entries, msg)
		}
	}
}

// evictOldest removes the entry with the oldest lastSeen time.
func (f *DedupFilter) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	first := true
	for key, entry := range f.entries {
		if first || entry.lastSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastSeen
			first = false
		}
	}
	if oldestKey != "" {
		delete(f.entries, oldestKey)
	}
}

// RandomSamplingFilter uses math/rand for probabilistic sampling.
// Less deterministic than SamplingFilter but more "random" distribution.
// Useful for distributed tracing where you don't want every Nth pattern.
type RandomSamplingFilter struct {
	Rate float64
	rng  *rand.Rand
	mu   sync.Mutex
}

// NewRandomSamplingFilter creates a RandomSamplingFilter.
func NewRandomSamplingFilter(rate float64) *RandomSamplingFilter {
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	return &RandomSamplingFilter{
		Rate: rate,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Allow returns true with probability Rate.
func (f *RandomSamplingFilter) Allow(_ context.Context, _ Level, _ string) bool {
	if f.Rate >= 1.0 {
		return true
	}
	if f.Rate <= 0.0 {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rng.Float64() < f.Rate
}
