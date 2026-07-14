package security

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// newTestLogger creates a real logger writing to a temp file and returns
// the logger, a cleanup function, and a function to read the file contents.
func newTestLogger(t *testing.T) (*logger.Logger, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Level = core.LevelTrace
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "security.log")
	cfg.Format = "json"
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false
	cfg.MaskEnabled = false

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	cleanup := func() { _ = log.Close() }
	readFn := func() string {
		data, _ := os.ReadFile(cfg.FilePath)
		return string(data)
	}
	return log, cleanup, readFn
}

// parseRecords parses each newline-delimited JSON line into a map.
func parseRecords(t *testing.T, output string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("failed to parse log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one log record, got none")
	}
	return records
}

// lastRecord returns the last parsed record from the output.
func lastRecord(t *testing.T, output string) map[string]any {
	t.Helper()
	records := parseRecords(t, output)
	return records[len(records)-1]
}

// assertEventID checks that the record contains the expected event_id.
func assertEventID(t *testing.T, rec map[string]any, want string) {
	t.Helper()
	got, ok := rec["event_id"].(string)
	if !ok {
		t.Errorf("event_id field missing or not a string in record: %v", rec)
		return
	}
	if got != want {
		t.Errorf("event_id = %q, want %q", got, want)
	}
}

// assertField checks that the record contains the expected string field value.
func assertField(t *testing.T, rec map[string]any, key, want string) {
	t.Helper()
	got, ok := rec[key].(string)
	if !ok {
		t.Errorf("field %q missing or not a string in record: %v", key, rec)
		return
	}
	if got != want {
		t.Errorf("field %q = %q, want %q", key, got, want)
	}
}

// assertFieldInt checks that the record contains the expected int field value.
func assertFieldInt(t *testing.T, rec map[string]any, key string, want int) {
	t.Helper()
	got, ok := rec[key].(float64) // JSON numbers parse as float64
	if !ok {
		t.Errorf("field %q missing or not a number in record: %v", key, rec)
		return
	}
	if int(got) != want {
		t.Errorf("field %q = %v, want %d", key, got, want)
	}
}

func TestNew(t *testing.T) {
	log, cleanup, _ := newTestLogger(t)
	defer cleanup()

	f := New(log)
	if f == nil {
		t.Fatal("New returned nil")
	}
	if f.logger == nil {
		t.Error("Facade.logger is nil")
	}
}

func TestFacade_LoginFailed(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LoginFailed(context.Background(), "alice", "10.0.0.1",
		slog.String("user_agent", "curl/8.0"))

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityLoginFailed))
	assertField(t, rec, "username", "alice")
	assertField(t, rec, "ip", "10.0.0.1")
	assertField(t, rec, "user_agent", "curl/8.0")
	if level, _ := rec["level"].(string); level != "WARN" {
		t.Errorf("level = %q, want WARN", level)
	}
}

func TestFacade_LoginSuccess(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LoginSuccess(context.Background(), "bob", "192.168.1.1")

	rec := lastRecord(t, read())
	assertEventID(t, rec, "security.auth.login_success")
	assertField(t, rec, "username", "bob")
	assertField(t, rec, "ip", "192.168.1.1")
	if level, _ := rec["level"].(string); level != "INFO" {
		t.Errorf("level = %q, want INFO", level)
	}
}

func TestFacade_Logout(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.Logout(context.Background(), "carol", "172.16.0.1")

	rec := lastRecord(t, read())
	assertEventID(t, rec, "security.auth.logout")
	assertField(t, rec, "username", "carol")
	assertField(t, rec, "ip", "172.16.0.1")
	if level, _ := rec["level"].(string); level != "INFO" {
		t.Errorf("level = %q, want INFO", level)
	}
}

func TestFacade_BruteForce(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.BruteForce(context.Background(), "10.0.0.99", 7)

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityBruteForce))
	assertField(t, rec, "ip", "10.0.0.99")
	assertFieldInt(t, rec, "attempt_count", 7)
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_SQLInjection(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.SQLInjection(context.Background(), "' OR 1=1 --", "10.0.0.2")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecuritySQLInjection))
	assertField(t, rec, "query", "' OR 1=1 --")
	assertField(t, rec, "ip", "10.0.0.2")
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_XSS(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.XSS(context.Background(), "<script>alert(1)</script>", "10.0.0.3")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityXSS))
	assertField(t, rec, "detection_input", "<script>alert(1)</script>")
	assertField(t, rec, "ip", "10.0.0.3")
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_CSRF(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.CSRF(context.Background(), "10.0.0.4")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityCSRF))
	assertField(t, rec, "ip", "10.0.0.4")
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_Unauthorized(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.Unauthorized(context.Background(), "user-123", "/admin/users")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityUnauthorized))
	assertField(t, rec, "user_id", "user-123")
	assertField(t, rec, "path", "/admin/users")
	if level, _ := rec["level"].(string); level != "WARN" {
		t.Errorf("level = %q, want WARN", level)
	}
}

func TestFacade_Forbidden(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.Forbidden(context.Background(), "user-456", "/admin/settings")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityForbidden))
	assertField(t, rec, "user_id", "user-456")
	assertField(t, rec, "path", "/admin/settings")
	if level, _ := rec["level"].(string); level != "WARN" {
		t.Errorf("level = %q, want WARN", level)
	}
}

func TestFacade_JWTInvalid(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.JWTInvalid(context.Background(), "abc.def.ghi", "10.0.0.5")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityJWTInvalid))
	assertField(t, rec, "metadata", "abc.def.ghi")
	assertField(t, rec, "ip", "10.0.0.5")
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_JWTExpired(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.JWTExpired(context.Background(), "expired.token.value", "10.0.0.6")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityJWTExpired))
	assertField(t, rec, "metadata", "expired.token.value")
	assertField(t, rec, "ip", "10.0.0.6")
	if level, _ := rec["level"].(string); level != "WARN" {
		t.Errorf("level = %q, want WARN", level)
	}
}

func TestFacade_RateLimited(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.RateLimited(context.Background(), "10.0.0.7")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityRateLimited))
	assertField(t, rec, "ip", "10.0.0.7")
	if level, _ := rec["level"].(string); level != "WARN" {
		t.Errorf("level = %q, want WARN", level)
	}
}

func TestFacade_PrivilegeEscalation(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.PrivilegeEscalation(context.Background(), "user-789", "user", "admin")

	rec := lastRecord(t, read())
	assertEventID(t, rec, string(core.EventSecurityPrivilegeEscalation))
	assertField(t, rec, "user_id", "user-789")
	assertField(t, rec, "role", "user")
	assertField(t, rec, "metadata", "admin")
	if level, _ := rec["level"].(string); level != "ERROR" {
		t.Errorf("level = %q, want ERROR", level)
	}
}

func TestFacade_AllEventIDs(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	ctx := context.Background()

	f.LoginFailed(ctx, "u1", "1.1.1.1")
	f.LoginSuccess(ctx, "u2", "2.2.2.2")
	f.Logout(ctx, "u3", "3.3.3.3")
	f.BruteForce(ctx, "4.4.4.4", 3)
	f.SQLInjection(ctx, "drop", "5.5.5.5")
	f.XSS(ctx, "<x>", "6.6.6.6")
	f.CSRF(ctx, "7.7.7.7")
	f.Unauthorized(ctx, "u8", "/r8")
	f.Forbidden(ctx, "u9", "/r9")
	f.JWTInvalid(ctx, "tok", "8.8.8.8")
	f.JWTExpired(ctx, "tok", "9.9.9.9")
	f.RateLimited(ctx, "10.10.10.10")
	f.PrivilegeEscalation(ctx, "u11", "user", "admin")

	wantIDs := []string{
		string(core.EventSecurityLoginFailed),
		"security.auth.login_success",
		"security.auth.logout",
		string(core.EventSecurityBruteForce),
		string(core.EventSecuritySQLInjection),
		string(core.EventSecurityXSS),
		string(core.EventSecurityCSRF),
		string(core.EventSecurityUnauthorized),
		string(core.EventSecurityForbidden),
		string(core.EventSecurityJWTInvalid),
		string(core.EventSecurityJWTExpired),
		string(core.EventSecurityRateLimited),
		string(core.EventSecurityPrivilegeEscalation),
	}

	records := parseRecords(t, read())
	if len(records) != len(wantIDs) {
		t.Fatalf("got %d records, want %d", len(records), len(wantIDs))
	}
	for i, want := range wantIDs {
		got, _ := records[i]["event_id"].(string)
		if got != want {
			t.Errorf("record[%d].event_id = %q, want %q", i, got, want)
		}
	}
}

func TestFacade_ExtraAttrsPreserved(t *testing.T) {
	log, cleanup, read := newTestLogger(t)
	defer cleanup()

	f := New(log)
	f.LoginFailed(context.Background(), "alice", "10.0.0.1",
		slog.String("custom_field", "custom_value"),
		slog.Int("custom_int", 42),
	)

	rec := lastRecord(t, read())
	assertField(t, rec, "custom_field", "custom_value")
	if got, ok := rec["custom_int"].(float64); !ok || int(got) != 42 {
		t.Errorf("custom_int not preserved correctly: %v", rec["custom_int"])
	}
	// Standard fields should still be present.
	assertField(t, rec, "username", "alice")
	assertField(t, rec, "ip", "10.0.0.1")
	assertEventID(t, rec, string(core.EventSecurityLoginFailed))
}
