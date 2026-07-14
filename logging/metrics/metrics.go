// Package metrics provides logger health monitoring and metrics collection.
//
// It tracks:
//   - Total logs written by level
//   - Total bytes written
//   - Write errors
//   - Write latency (histogram)
//   - Sink health (last error, error count)
//   - Async queue depth (if async sink is used)
//   - Uptime
//
// Metrics can be exposed via:
//   - Prometheus exposition format (for scraping)
//   - JSON snapshot (for health endpoints)
//   - Log-based metrics (periodic dump to log)
package metrics

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds logger health metrics.
// All fields are atomic for lock-free concurrent access.
type Metrics struct {
	// Counters
	TotalLogs    atomic.Int64
	TraceLogs    atomic.Int64
	DebugLogs    atomic.Int64
	InfoLogs     atomic.Int64
	WarnLogs     atomic.Int64
	ErrorLogs    atomic.Int64
	FatalLogs    atomic.Int64
	PanicLogs    atomic.Int64

	// Byte counter
	TotalBytes atomic.Int64

	// Error tracking
	WriteErrors atomic.Int64
	LastErrorMessage atomic.Value // string

	// Latency tracking (nanoseconds)
	totalLatencyNs atomic.Int64
	maxLatencyNs   atomic.Int64

	// Async queue depth (if applicable)
	AsyncQueueDepth atomic.Int64

	// Uptime
	startTime time.Time

	// Sink health
	mu          sync.RWMutex
	sinkHealth  map[string]*SinkHealth
}

// SinkHealth tracks health of a single sink.
type SinkHealth struct {
	Name         string    `json:"name"`
	TotalWrites  int64     `json:"total_writes"`
	TotalErrors  int64     `json:"total_errors"`
	LastError    string    `json:"last_error,omitempty"`
	LastErrorAt  time.Time `json:"last_error_at,omitempty"`
	Healthy      bool      `json:"healthy"`
	LastWriteAt  time.Time `json:"last_write_at,omitempty"`
	TotalBytes   int64     `json:"total_bytes"`
}

// New creates a new Metrics instance.
func New() *Metrics {
	return &Metrics{
		startTime: time.Now(),
		sinkHealth: make(map[string]*SinkHealth),
	}
}

// IncLog increments the log counter for the given level.
func (m *Metrics) IncLog(level string) {
	m.TotalLogs.Add(1)
	switch strings.ToUpper(level) {
	case "TRACE":
		m.TraceLogs.Add(1)
	case "DEBUG":
		m.DebugLogs.Add(1)
	case "INFO":
		m.InfoLogs.Add(1)
	case "WARN", "WARNING":
		m.WarnLogs.Add(1)
	case "ERROR":
		m.ErrorLogs.Add(1)
	case "FATAL":
		m.FatalLogs.Add(1)
	case "PANIC":
		m.PanicLogs.Add(1)
	}
}

// IncBytes adds to the total bytes written counter.
func (m *Metrics) IncBytes(n int64) {
	m.TotalBytes.Add(n)
}

// IncError increments the write error counter and records the error message.
func (m *Metrics) IncError(err error) {
	m.WriteErrors.Add(1)
	if err != nil {
		m.LastErrorMessage.Store(err.Error())
	}
}

// RecordLatency records a write latency.
func (m *Metrics) RecordLatency(d time.Duration) {
	ns := d.Nanoseconds()
	m.totalLatencyNs.Add(ns)
	for {
		current := m.maxLatencyNs.Load()
		if ns <= current || m.maxLatencyNs.CompareAndSwap(current, ns) {
			break
		}
	}
}

// AvgLatency returns the average write latency.
func (m *Metrics) AvgLatency() time.Duration {
	total := m.TotalLogs.Load()
	if total == 0 {
		return 0
	}
	return time.Duration(m.totalLatencyNs.Load() / total)
}

// MaxLatency returns the maximum write latency.
func (m *Metrics) MaxLatency() time.Duration {
	return time.Duration(m.maxLatencyNs.Load())
}

// Uptime returns the time since metrics collection started.
func (m *Metrics) Uptime() time.Duration {
	return time.Since(m.startTime)
}

// SetAsyncQueueDepth sets the current async queue depth.
func (m *Metrics) SetAsyncQueueDepth(depth int64) {
	m.AsyncQueueDepth.Store(depth)
}

// --- Sink Health ---

// RecordSinkWrite records a successful write to a sink.
func (m *Metrics) RecordSinkWrite(name string, bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.sinkHealth[name]
	if !ok {
		h = &SinkHealth{Name: name, Healthy: true}
		m.sinkHealth[name] = h
	}
	h.TotalWrites++
	h.TotalBytes += bytes
	h.LastWriteAt = time.Now()
	h.Healthy = true
	h.LastError = ""
}

// RecordSinkError records a write error for a sink.
func (m *Metrics) RecordSinkError(name string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.sinkHealth[name]
	if !ok {
		h = &SinkHealth{Name: name}
		m.sinkHealth[name] = h
	}
	h.TotalErrors++
	h.LastError = err.Error()
	h.LastErrorAt = time.Now()
	h.Healthy = false
}

// SinkHealth returns the health status of a specific sink.
func (m *Metrics) SinkHealth(name string) *SinkHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if h, ok := m.sinkHealth[name]; ok {
		// Return a copy.
		copy := *h
		return &copy
	}
	return nil
}

// AllSinkHealth returns health status of all sinks.
func (m *Metrics) AllSinkHealth() []SinkHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]SinkHealth, 0, len(m.sinkHealth))
	for _, h := range m.sinkHealth {
		result = append(result, *h)
	}
	return result
}

// --- Snapshots ---

// Snapshot returns a point-in-time snapshot of all metrics.
type Snapshot struct {
	Timestamp     time.Time     `json:"timestamp"`
	Uptime        string        `json:"uptime"`
	TotalLogs     int64         `json:"total_logs"`
	ByLevel       LevelCounts   `json:"by_level"`
	TotalBytes    int64         `json:"total_bytes"`
	WriteErrors   int64         `json:"write_errors"`
	LastError     string        `json:"last_error,omitempty"`
	AvgLatency    string        `json:"avg_latency"`
	MaxLatency    string        `json:"max_latency"`
	AsyncQueueDepth int64       `json:"async_queue_depth"`
	Sinks         []SinkHealth  `json:"sinks"`
}

// LevelCounts holds log counts by level.
type LevelCounts struct {
	Trace  int64 `json:"trace"`
	Debug  int64 `json:"debug"`
	Info   int64 `json:"info"`
	Warn   int64 `json:"warn"`
	Error  int64 `json:"error"`
	Fatal  int64 `json:"fatal"`
	Panic  int64 `json:"panic"`
}

// Snapshot returns a point-in-time snapshot of all metrics.
func (m *Metrics) Snapshot() Snapshot {
	lastErr, _ := m.LastErrorMessage.Load().(string)
	return Snapshot{
		Timestamp:  time.Now(),
		Uptime:     m.Uptime().String(),
		TotalLogs:  m.TotalLogs.Load(),
		ByLevel: LevelCounts{
			Trace: m.TraceLogs.Load(),
			Debug: m.DebugLogs.Load(),
			Info:  m.InfoLogs.Load(),
			Warn:  m.WarnLogs.Load(),
			Error: m.ErrorLogs.Load(),
			Fatal: m.FatalLogs.Load(),
			Panic: m.PanicLogs.Load(),
		},
		TotalBytes:      m.TotalBytes.Load(),
		WriteErrors:     m.WriteErrors.Load(),
		LastError:       lastErr,
		AvgLatency:      m.AvgLatency().String(),
		MaxLatency:      m.MaxLatency().String(),
		AsyncQueueDepth: m.AsyncQueueDepth.Load(),
		Sinks:           m.AllSinkHealth(),
	}
}

// JSON returns a JSON representation of the metrics snapshot.
func (m *Metrics) JSON() ([]byte, error) {
	return json.Marshal(m.Snapshot())
}

// --- Prometheus Exposition ---

// WritePrometheus writes metrics in Prometheus exposition format.
func (m *Metrics) WritePrometheus(w io.Writer) error {
	snap := m.Snapshot()
	var buf strings.Builder

	// Total logs.
	buf.WriteString("# HELP logger_total_logs Total number of log entries written.\n")
	buf.WriteString("# TYPE logger_total_logs counter\n")
	fmt.Fprintf(&buf, "logger_total_logs %d\n", snap.TotalLogs)

	// By level.
	buf.WriteString("# HELP logger_logs_by_level Number of log entries by level.\n")
	buf.WriteString("# TYPE logger_logs_by_level counter\n")
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"trace\"} %d\n", snap.ByLevel.Trace)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"debug\"} %d\n", snap.ByLevel.Debug)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"info\"} %d\n", snap.ByLevel.Info)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"warn\"} %d\n", snap.ByLevel.Warn)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"error\"} %d\n", snap.ByLevel.Error)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"fatal\"} %d\n", snap.ByLevel.Fatal)
	fmt.Fprintf(&buf, "logger_logs_by_level{level=\"panic\"} %d\n", snap.ByLevel.Panic)

	// Total bytes.
	buf.WriteString("# HELP logger_total_bytes Total bytes written to sinks.\n")
	buf.WriteString("# TYPE logger_total_bytes counter\n")
	fmt.Fprintf(&buf, "logger_total_bytes %d\n", snap.TotalBytes)

	// Write errors.
	buf.WriteString("# HELP logger_write_errors Total write errors.\n")
	buf.WriteString("# TYPE logger_write_errors counter\n")
	fmt.Fprintf(&buf, "logger_write_errors %d\n", snap.WriteErrors)

	// Async queue depth.
	buf.WriteString("# HELP logger_async_queue_depth Current async queue depth.\n")
	buf.WriteString("# TYPE logger_async_queue_depth gauge\n")
	fmt.Fprintf(&buf, "logger_async_queue_depth %d\n", snap.AsyncQueueDepth)

	// Sink health.
	buf.WriteString("# HELP logger_sink_healthy Whether a sink is healthy (1=yes, 0=no).\n")
	buf.WriteString("# TYPE logger_sink_healthy gauge\n")
	for _, sink := range snap.Sinks {
		healthy := 0
		if sink.Healthy {
			healthy = 1
		}
		fmt.Fprintf(&buf, "logger_sink_healthy{sink=%q} %d\n", sink.Name, healthy)
		fmt.Fprintf(&buf, "logger_sink_writes_total{sink=%q} %d\n", sink.Name, sink.TotalWrites)
		fmt.Fprintf(&buf, "logger_sink_errors_total{sink=%q} %d\n", sink.Name, sink.TotalErrors)
	}

	_, err := w.Write([]byte(buf.String()))
	return err
}

// PrometheusString returns the Prometheus exposition as a string.
func (m *Metrics) PrometheusString() string {
	var buf strings.Builder
	_ = m.WritePrometheus(&buf)
	return buf.String()
}

// Reset resets all metrics to zero.
func (m *Metrics) Reset() {
	m.TotalLogs.Store(0)
	m.TraceLogs.Store(0)
	m.DebugLogs.Store(0)
	m.InfoLogs.Store(0)
	m.WarnLogs.Store(0)
	m.ErrorLogs.Store(0)
	m.FatalLogs.Store(0)
	m.PanicLogs.Store(0)
	m.TotalBytes.Store(0)
	m.WriteErrors.Store(0)
	m.totalLatencyNs.Store(0)
	m.maxLatencyNs.Store(0)
	m.AsyncQueueDepth.Store(0)
	m.startTime = time.Now()
	m.mu.Lock()
	m.sinkHealth = make(map[string]*SinkHealth)
	m.mu.Unlock()
}
