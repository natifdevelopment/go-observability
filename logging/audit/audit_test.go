package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// newTestFacade creates a Facade backed by a temp-file AuditSink and returns
// the facade, the file path, and a cleanup function.
func newTestFacade(t *testing.T) (*Facade, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	s, err := sink.NewAuditSink(path)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	f := New(s, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	return f, path
}

func TestNew_NilLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	s, err := sink.NewAuditSink(path)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	f := New(s, nil)
	if f == nil {
		t.Fatal("expected non-nil facade")
	}
	if f.logger == nil {
		t.Fatal("expected non-nil logger after nil handling")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestNewPayload(t *testing.T) {
	p := NewPayload("alice", "login", "10.0.0.1")
	if p.User != "alice" {
		t.Errorf("User = %q, want alice", p.User)
	}
	if p.Action != "login" {
		t.Errorf("Action = %q, want login", p.Action)
	}
	if p.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want 10.0.0.1", p.IP)
	}
	if p.Timestamp == "" {
		t.Error("Timestamp should be set")
	}
}

func TestFacade_Log(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	p := NewPayload("alice", "create_record", "10.0.0.1")
	p.Resource = "orders/123"
	if err := f.Log(ctx, p); err != nil {
		t.Fatalf("Log: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}

	// Write a second record to exercise chain progression.
	p2 := NewPayload("bob", "delete_record", "10.0.0.2")
	if err := f.Log(ctx, p2); err != nil {
		t.Fatalf("Log second: %v", err)
	}
	if f.sink.Seq() != 2 {
		t.Errorf("Seq = %d, want 2", f.sink.Seq())
	}
}

func TestFacade_Log_NilSink(t *testing.T) {
	f := &Facade{logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	if err := f.Log(context.Background(), NewPayload("u", "a", "ip")); err == nil {
		t.Fatal("expected error when sink is nil")
	}
}

func TestFacade_Login_Success(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.Login(ctx, "alice", "login", "10.0.0.1", true,
		slog.String("device", "laptop"),
	); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

func TestFacade_Login_Failure(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.Login(ctx, "eve", "login", "10.0.0.9", false); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

func TestFacade_DataAccess(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.DataAccess(ctx, "bob", "customers/42", "read",
		slog.String("query", "select"),
	); err != nil {
		t.Fatalf("DataAccess: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

func TestFacade_ConfigChange(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.ConfigChange(ctx, "admin", "max_connections", "100", "200"); err != nil {
		t.Fatalf("ConfigChange: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

func TestFacade_Verify_ValidChain(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := f.Log(ctx, NewPayload("alice", "action", "10.0.0.1")); err != nil {
			t.Fatalf("Log %d: %v", i, err)
		}
	}
	idx, err := f.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if idx != -1 {
		t.Errorf("tampered index = %d, want -1", idx)
	}
}

func TestFacade_Verify_EmptyFile(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	idx, err := f.Verify(context.Background())
	if err != nil {
		t.Fatalf("Verify on empty file: %v", err)
	}
	if idx != -1 {
		t.Errorf("tampered index = %d, want -1 for empty file", idx)
	}
}

func TestFacade_Verify_TamperedChain(t *testing.T) {
	f, path := newTestFacade(t)

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := f.Log(ctx, NewPayload("alice", "action", "10.0.0.1")); err != nil {
			t.Fatalf("Log %d: %v", i, err)
		}
	}
	// Must close before tampering so buffers flush.
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Tamper: rewrite the file with a modified payload on the second line.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Append garbage to corrupt the last entry's hash linkage.
	if err := os.WriteFile(path, append(data, []byte(`{"seq":99,"prev_hash":"deadbeef","payload":{"user":"evil"},"curr_hash":"deadbeef"}`+"\n")...), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Reopen for verification.
	s2, err := sink.NewAuditSink(path)
	if err != nil {
		t.Fatalf("NewAuditSink reopen: %v", err)
	}
	f2 := New(s2, nil)
	defer f2.Close()

	idx, err := f2.Verify(ctx)
	if err == nil {
		t.Fatal("expected verification error for tampered chain")
	}
	if idx < 0 {
		t.Errorf("expected tampered index >= 0, got %d", idx)
	}
}

func TestFacade_Close(t *testing.T) {
	f, _ := newTestFacade(t)

	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Second close should be a no-op (not error / not panic).
	if err := f.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// Writes after close should fail.
	if err := f.Log(context.Background(), NewPayload("u", "a", "ip")); err == nil {
		t.Fatal("expected error logging after close")
	}
}

func TestFacade_Close_NilSink(t *testing.T) {
	f := &Facade{logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	if err := f.Close(); err != nil {
		t.Fatalf("Close with nil sink: %v", err)
	}
}

func TestFacade_Verify_NilSink(t *testing.T) {
	f := &Facade{logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	if _, err := f.Verify(context.Background()); err == nil {
		t.Fatal("expected error verifying with nil sink")
	}
}

func TestFacade_Login_MultipleAttrs(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.Login(ctx, "alice", "login", "10.0.0.1", true,
		slog.String("device", "laptop"),
		slog.String("browser", "firefox"),
		slog.Int("attempts", 1),
	); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

func TestFacade_DataAccess_NoAttrs(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.DataAccess(ctx, "bob", "customers/42", "delete"); err != nil {
		t.Fatalf("DataAccess: %v", err)
	}
	if f.sink.Seq() != 1 {
		t.Errorf("Seq = %d, want 1", f.sink.Seq())
	}
}

// Ensure the payload produced by helpers round-trips through the chain.
func TestFacade_Integration(t *testing.T) {
	f, _ := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.Login(ctx, "alice", "login", "10.0.0.1", true); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := f.DataAccess(ctx, "alice", "orders/1", "read"); err != nil {
		t.Fatalf("DataAccess: %v", err)
	}
	if err := f.ConfigChange(ctx, "alice", "timeout", "30s", "60s"); err != nil {
		t.Fatalf("ConfigChange: %v", err)
	}
	if f.sink.Seq() != 3 {
		t.Errorf("Seq = %d, want 3", f.sink.Seq())
	}
	idx, err := f.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if idx != -1 {
		t.Errorf("tampered index = %d, want -1", idx)
	}
}

// Ensure Result field is set correctly for login events.
func TestFacade_Login_ResultSet(t *testing.T) {
	f, path := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.Login(ctx, "alice", "login", "10.0.0.1", true); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := f.Login(ctx, "eve", "login", "10.0.0.9", false); err != nil {
		t.Fatalf("Login fail: %v", err)
	}

	// Read back and verify metadata.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	entries := parseEntries(t, data)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Check Metadata for result.
	if got := entries[0].Payload.Metadata["result"]; got != "success" {
		t.Errorf("entry 0 result = %v, want success", got)
	}
	if got := entries[1].Payload.Metadata["result"]; got != "failure" {
		t.Errorf("entry 1 result = %v, want failure", got)
	}
}

// Ensure ConfigChange captures before/after values.
func TestFacade_ConfigChange_BeforeAfter(t *testing.T) {
	f, path := newTestFacade(t)
	defer f.Close()

	ctx := context.Background()
	if err := f.ConfigChange(ctx, "admin", "max_conn", "100", "200"); err != nil {
		t.Fatalf("ConfigChange: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	entries := parseEntries(t, data)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Payload.Resource != "max_conn" {
		t.Errorf("Resource = %q, want max_conn", entries[0].Payload.Resource)
	}
	if entries[0].Payload.Before != "100" {
		t.Errorf("Before = %v, want 100", entries[0].Payload.Before)
	}
	if entries[0].Payload.After != "200" {
		t.Errorf("After = %v, want 200", entries[0].Payload.After)
	}
}

// parseEntries reads the audit log file bytes into chain entries.
func parseEntries(t *testing.T, data []byte) []core.AuditChainEntry {
	t.Helper()
	var entries []core.AuditChainEntry
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			if len(line) == 0 {
				continue
			}
			var e core.AuditChainEntry
			if err := json.Unmarshal(line, &e); err != nil {
				t.Fatalf("unmarshal entry: %v", err)
			}
			entries = append(entries, e)
		}
	}
	if start < len(data) {
		var e core.AuditChainEntry
		if err := json.Unmarshal(data[start:], &e); err != nil {
			t.Fatalf("unmarshal trailing entry: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}
