package metrics

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMetrics_New(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New should return non-nil")
	}
	if m.Uptime() <= 0 {
		t.Error("Uptime should be > 0")
	}
}

func TestMetrics_IncLog(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncLog("INFO")
	m.IncLog("ERROR")
	m.IncLog("warn")
	m.IncLog("WARNING")
	m.IncLog("debug")
	m.IncLog("trace")
	m.IncLog("fatal")
	m.IncLog("panic")

	if m.TotalLogs.Load() != 9 {
		t.Errorf("TotalLogs = %d, want 9", m.TotalLogs.Load())
	}
	if m.InfoLogs.Load() != 2 {
		t.Errorf("InfoLogs = %d, want 2", m.InfoLogs.Load())
	}
	if m.ErrorLogs.Load() != 1 {
		t.Errorf("ErrorLogs = %d, want 1", m.ErrorLogs.Load())
	}
	if m.WarnLogs.Load() != 2 {
		t.Errorf("WarnLogs = %d, want 2", m.WarnLogs.Load())
	}
}

func TestMetrics_IncBytes(t *testing.T) {
	m := New()
	m.IncBytes(100)
	m.IncBytes(200)
	if m.TotalBytes.Load() != 300 {
		t.Errorf("TotalBytes = %d, want 300", m.TotalBytes.Load())
	}
}

func TestMetrics_IncError(t *testing.T) {
	m := New()
	m.IncError(errors.New("write failed"))
	if m.WriteErrors.Load() != 1 {
		t.Errorf("WriteErrors = %d, want 1", m.WriteErrors.Load())
	}
	lastErr, _ := m.LastErrorMessage.Load().(string)
	if lastErr != "write failed" {
		t.Errorf("LastErrorMessage = %q", lastErr)
	}
}

func TestMetrics_IncError_Nil(t *testing.T) {
	m := New()
	m.IncError(nil)
	if m.WriteErrors.Load() != 1 {
		t.Errorf("WriteErrors = %d, want 1", m.WriteErrors.Load())
	}
}

func TestMetrics_RecordLatency(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncLog("INFO") // 2 logs to match 2 latency recordings
	m.RecordLatency(10 * time.Millisecond)
	m.RecordLatency(20 * time.Millisecond)

	avg := m.AvgLatency()
	// avg = (10+20)/2 = 15ms
	if avg != 15*time.Millisecond {
		t.Errorf("AvgLatency = %v, want 15ms", avg)
	}
	maxLat := m.MaxLatency()
	if maxLat < 20*time.Millisecond {
		t.Errorf("MaxLatency = %v, should be >= 20ms", maxLat)
	}
}

func TestMetrics_AvgLatency_NoLogs(t *testing.T) {
	m := New()
	if m.AvgLatency() != 0 {
		t.Error("AvgLatency with no logs should be 0")
	}
}

func TestMetrics_SetAsyncQueueDepth(t *testing.T) {
	m := New()
	m.SetAsyncQueueDepth(42)
	if m.AsyncQueueDepth.Load() != 42 {
		t.Errorf("AsyncQueueDepth = %d, want 42", m.AsyncQueueDepth.Load())
	}
}

func TestMetrics_RecordSinkWrite(t *testing.T) {
	m := New()
	m.RecordSinkWrite("file", 1024)
	m.RecordSinkWrite("file", 2048)

	h := m.SinkHealth("file")
	if h == nil {
		t.Fatal("SinkHealth should not be nil")
	}
	if h.TotalWrites != 2 {
		t.Errorf("TotalWrites = %d, want 2", h.TotalWrites)
	}
	if h.TotalBytes != 3072 {
		t.Errorf("TotalBytes = %d, want 3072", h.TotalBytes)
	}
	if !h.Healthy {
		t.Error("sink should be healthy after successful write")
	}
}

func TestMetrics_RecordSinkError(t *testing.T) {
	m := New()
	m.RecordSinkWrite("console", 100)
	m.RecordSinkError("console", errors.New("disk full"))

	h := m.SinkHealth("console")
	if h == nil {
		t.Fatal("SinkHealth should not be nil")
	}
	if h.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d, want 1", h.TotalErrors)
	}
	if h.Healthy {
		t.Error("sink should be unhealthy after error")
	}
	if h.LastError != "disk full" {
		t.Errorf("LastError = %q", h.LastError)
	}
}

func TestMetrics_SinkHealth_NotFound(t *testing.T) {
	m := New()
	if m.SinkHealth("nonexistent") != nil {
		t.Error("nonexistent sink should return nil")
	}
}

func TestMetrics_AllSinkHealth(t *testing.T) {
	m := New()
	m.RecordSinkWrite("sink1", 100)
	m.RecordSinkWrite("sink2", 200)

	all := m.AllSinkHealth()
	if len(all) != 2 {
		t.Errorf("expected 2 sinks, got %d", len(all))
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncLog("ERROR")
	m.IncBytes(500)
	m.IncError(errors.New("test error"))
	m.RecordSinkWrite("file", 500)

	snap := m.Snapshot()
	if snap.TotalLogs != 2 {
		t.Errorf("TotalLogs = %d, want 2", snap.TotalLogs)
	}
	if snap.ByLevel.Info != 1 {
		t.Errorf("ByLevel.Info = %d", snap.ByLevel.Info)
	}
	if snap.ByLevel.Error != 1 {
		t.Errorf("ByLevel.Error = %d", snap.ByLevel.Error)
	}
	if snap.TotalBytes != 500 {
		t.Errorf("TotalBytes = %d", snap.TotalBytes)
	}
	if snap.WriteErrors != 1 {
		t.Errorf("WriteErrors = %d", snap.WriteErrors)
	}
	if snap.LastError != "test error" {
		t.Errorf("LastError = %q", snap.LastError)
	}
	if len(snap.Sinks) != 1 {
		t.Errorf("Sinks = %d, want 1", len(snap.Sinks))
	}
}

func TestMetrics_JSON(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncBytes(100)

	data, err := m.JSON()
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if snap.TotalLogs != 1 {
		t.Errorf("TotalLogs = %d", snap.TotalLogs)
	}
}

func TestMetrics_PrometheusString(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncLog("ERROR")
	m.IncBytes(1024)
	m.RecordSinkWrite("file", 1024)

	s := m.PrometheusString()
	if !strings.Contains(s, "logger_total_logs") {
		t.Error("Prometheus output should contain logger_total_logs")
	}
	if !strings.Contains(s, "logger_logs_by_level") {
		t.Error("Prometheus output should contain logger_logs_by_level")
	}
	if !strings.Contains(s, "logger_total_bytes") {
		t.Error("Prometheus output should contain logger_total_bytes")
	}
	if !strings.Contains(s, "logger_sink_healthy") {
		t.Error("Prometheus output should contain logger_sink_healthy")
	}
}

func TestMetrics_PrometheusString_WithErrors(t *testing.T) {
	m := New()
	m.IncError(errors.New("test"))
	s := m.PrometheusString()
	if !strings.Contains(s, "logger_write_errors") {
		t.Error("Prometheus output should contain logger_write_errors")
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := New()
	m.IncLog("INFO")
	m.IncBytes(100)
	m.IncError(errors.New("err"))
	m.RecordSinkWrite("file", 100)

	m.Reset()

	if m.TotalLogs.Load() != 0 {
		t.Error("TotalLogs should be 0 after reset")
	}
	if m.TotalBytes.Load() != 0 {
		t.Error("TotalBytes should be 0 after reset")
	}
	if m.WriteErrors.Load() != 0 {
		t.Error("WriteErrors should be 0 after reset")
	}
	if len(m.AllSinkHealth()) != 0 {
		t.Error("sink health should be empty after reset")
	}
}

func TestMetrics_Concurrent(t *testing.T) {
	m := New()
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.IncLog("INFO")
				m.IncBytes(10)
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	if m.TotalLogs.Load() != 1000 {
		t.Errorf("TotalLogs = %d, want 1000", m.TotalLogs.Load())
	}
	if m.TotalBytes.Load() != 10000 {
		t.Errorf("TotalBytes = %d, want 10000", m.TotalBytes.Load())
	}
}
