package sink

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

func TestAuditSink_WriteAndVerify(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.log")

	s, err := NewAuditSink(path)
	if err != nil {
		t.Fatalf("NewAuditSink failed: %v", err)
	}
	defer s.Close()

	payloads := []core.AuditPayload{
		{Timestamp: "t1", User: "u1", Action: "create", IP: "ip1"},
		{Timestamp: "t2", User: "u2", Action: "update", IP: "ip2"},
		{Timestamp: "t3", User: "u3", Action: "delete", IP: "ip3"},
	}

	for _, p := range payloads {
		if err := s.WriteAudit(context.Background(), p); err != nil {
			t.Fatalf("WriteAudit failed: %v", err)
		}
	}

	// Verify chain.
	idx, err := s.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify failed: %v", err)
	}
	if idx != -1 {
		t.Errorf("chain should be valid, tampered at %d", idx)
	}
}

func TestAuditSink_SequenceIncrement(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "seq.log")

	s, _ := NewAuditSink(path)
	defer s.Close()

	if s.Seq() != 0 {
		t.Errorf("initial seq should be 0, got %d", s.Seq())
	}

	s.WriteAudit(context.Background(), core.AuditPayload{User: "u", Action: "a", IP: "ip", Timestamp: "t"})
	if s.Seq() != 1 {
		t.Errorf("seq should be 1 after one write, got %d", s.Seq())
	}

	s.WriteAudit(context.Background(), core.AuditPayload{User: "u", Action: "a", IP: "ip", Timestamp: "t"})
	if s.Seq() != 2 {
		t.Errorf("seq should be 2 after two writes, got %d", s.Seq())
	}
}

func TestAuditSink_PrevHash(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "hash.log")

	s, _ := NewAuditSink(path)
	defer s.Close()

	if s.PrevHash() != core.ZeroHash {
		t.Error("initial prevHash should be ZeroHash")
	}

	s.WriteAudit(context.Background(), core.AuditPayload{User: "u", Action: "a", IP: "ip", Timestamp: "t"})
	if s.PrevHash() == core.ZeroHash {
		t.Error("prevHash should be updated after write")
	}
}

func TestAuditSink_ResumeChain(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "resume.log")

	// Write some records.
	s1, _ := NewAuditSink(path)
	s1.WriteAudit(context.Background(), core.AuditPayload{User: "u1", Action: "a", IP: "ip", Timestamp: "t"})
	s1.WriteAudit(context.Background(), core.AuditPayload{User: "u2", Action: "a", IP: "ip", Timestamp: "t"})
	s1.Close()

	// Reopen and verify chain resumes.
	s2, _ := NewAuditSink(path)
	defer s2.Close()

	if s2.Seq() != 2 {
		t.Errorf("resumed seq should be 2, got %d", s2.Seq())
	}

	// Continue writing.
	s2.WriteAudit(context.Background(), core.AuditPayload{User: "u3", Action: "a", IP: "ip", Timestamp: "t"})
	if s2.Seq() != 3 {
		t.Errorf("seq after resume+write should be 3, got %d", s2.Seq())
	}

	// Verify entire chain.
	idx, err := s2.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify after resume failed: %v", err)
	}
	if idx != -1 {
		t.Errorf("chain should be valid after resume, tampered at %d", idx)
	}
}

func TestAuditSink_EmptyPath(t *testing.T) {
	_, err := NewAuditSink("")
	if err == nil {
		t.Error("empty path should error")
	}
}

func TestAuditSink_Name(t *testing.T) {
	s, _ := NewAuditSink(filepath.Join(t.TempDir(), "name.log"))
	defer s.Close()
	if s.Name() == "" {
		t.Error("Name should not be empty")
	}
}

func TestAuditSink_WriteAfterClose(t *testing.T) {
	s, _ := NewAuditSink(filepath.Join(t.TempDir(), "closed.log"))
	s.Close()
	err := s.WriteAudit(context.Background(), core.AuditPayload{User: "u", Action: "a", IP: "ip", Timestamp: "t"})
	if err == nil {
		t.Error("WriteAudit after Close should error")
	}
}

func TestAuditSink_Close_Idempotent(t *testing.T) {
	s, _ := NewAuditSink(filepath.Join(t.TempDir(), "idem.log"))
	s.Close()
	if err := s.Close(); err != nil {
		t.Errorf("second Close should be idempotent, got %v", err)
	}
}

func TestAuditSink_VerifyEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.log")
	// Create empty file.
	os.WriteFile(path, []byte{}, 0600)

	s, _ := NewAuditSink(path)
	defer s.Close()

	idx, err := s.Verify(context.Background())
	if err != nil {
		t.Errorf("Verify on empty file should not error, got %v", err)
	}
	if idx != -1 {
		t.Errorf("empty file should be valid, got tampered at %d", idx)
	}
}

func TestSinkMetrics_RecordAndSnapshot(t *testing.T) {
	m := NewSinkMetrics()
	m.RecordWrite(100, 5000)
	m.RecordWrite(200, 15000)
	m.RecordDrop()
	m.RecordError()

	snap := m.Snapshot()
	if snap.Written != 2 {
		t.Errorf("Written = %d, want 2", snap.Written)
	}
	if snap.Dropped != 1 {
		t.Errorf("Dropped = %d, want 1", snap.Dropped)
	}
	if snap.Errors != 1 {
		t.Errorf("Errors = %d, want 1", snap.Errors)
	}
	if snap.Bytes != 300 {
		t.Errorf("Bytes = %d, want 300", snap.Bytes)
	}
	if snap.AvgLatency != 10000 {
		t.Errorf("AvgLatency = %d, want 10000", snap.AvgLatency)
	}
	if snap.MaxLatency != 15000 {
		t.Errorf("MaxLatency = %d, want 15000", snap.MaxLatency)
	}
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := newCircuitBreaker(3, 100*time.Millisecond)

	// Should allow in closed state.
	if !cb.allow() {
		t.Error("should allow in closed state")
	}

	// Record failures to trigger open.
	cb.recordFailure()
	cb.recordFailure()
	cb.recordFailure()

	// Should be open now.
	if cb.allow() {
		t.Error("should not allow when open")
	}
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	cb := newCircuitBreaker(1, 50*time.Millisecond)
	cb.recordFailure() // triggers open

	// Wait for reset timeout.
	time.Sleep(60 * time.Millisecond)

	// Should transition to half-open and allow.
	if !cb.allow() {
		t.Error("should allow in half-open after reset timeout")
	}
}

func TestCircuitBreaker_HalfOpenSuccess(t *testing.T) {
	cb := newCircuitBreaker(1, 50*time.Millisecond)
	cb.recordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.allow()       // transitions to half-open
	cb.recordSuccess() // transitions back to closed

	if !cb.allow() {
		t.Error("should allow after half-open success")
	}
}

func TestCircuitBreaker_IsOpen(t *testing.T) {
	cb := newCircuitBreaker(1, 1*time.Second)
	if cb.isOpen() {
		t.Error("should not be open initially")
	}
	cb.recordFailure()
	if !cb.isOpen() {
		t.Error("should be open after threshold failures")
	}
}

func TestBufferedSink_Write(t *testing.T) {
	counting := &countingSink{}
	buf := NewBufferedSink(counting, 4096)
	defer buf.Close()

	err := buf.Write(context.Background(), []byte("buffered\n"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
}

func TestBufferedSink_Name(t *testing.T) {
	counting := &countingSink{}
	buf := NewBufferedSink(counting, 0) // default size
	defer buf.Close()
	if buf.Name() != "buffered:counting" {
		t.Errorf("Name = %q", buf.Name())
	}
}

func TestSimpleRand(t *testing.T) {
	for i := 0; i < 100; i++ {
		r := simpleRand()
		if r < 0 || r >= 1 {
			t.Errorf("simpleRand() = %f, should be in [0, 1)", r)
		}
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", p.MaxAttempts)
	}
	if p.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay = %v, want 100ms", p.InitialDelay)
	}
}

func TestSplitLines(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")
	lines := splitLines(data)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	if string(lines[0]) != "line1" {
		t.Errorf("line[0] = %q", lines[0])
	}
}

func TestSplitLines_NoTrailingNewline(t *testing.T) {
	data := []byte("line1\nline2")
	lines := splitLines(data)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
}

func TestSplitLines_Empty(t *testing.T) {
	lines := splitLines([]byte{})
	if len(lines) != 0 {
		t.Errorf("empty input should return 0 lines, got %d", len(lines))
	}
}

func TestSplitLines_CarriageReturn(t *testing.T) {
	data := []byte("line1\r\nline2\r\n")
	lines := splitLines(data)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if string(lines[0]) != "line1" {
		t.Errorf("line[0] = %q, want 'line1'", lines[0])
	}
}
