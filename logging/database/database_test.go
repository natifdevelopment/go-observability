package database

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/natifdevelopment/go-observability/logging/config"
	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// newTestLogger creates a file-based logger for testing and returns the
// facade plus a cleanup function and a reader that returns the file
// contents. The logger is configured at DEBUG level so that all
// database methods emit records.
func newTestLogger(t *testing.T) (*Facade, func(), func() string) {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Level = core.LevelDebug
	cfg.Output = "file"
	cfg.FilePath = filepath.Join(tmpDir, "db.log")
	cfg.EnableCaller = false
	cfg.EnableStacktrace = false
	cfg.MaskEnabled = false

	log, err := logger.New(cfg)
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	facade := New(log)
	cleanup := func() { _ = log.Close() }
	readFn := func() string {
		data, _ := os.ReadFile(cfg.FilePath)
		return string(data)
	}
	return facade, cleanup, readFn
}

func TestNew(t *testing.T) {
	facade, cleanup, _ := newTestLogger(t)
	defer cleanup()

	if facade == nil {
		t.Fatal("New returned nil facade")
	}
	if facade.log == nil {
		t.Fatal("facade.log is nil")
	}
}

func TestNew_WrapsLogger(t *testing.T) {
	// New should accept any logger and store it without modification.
	log, err := logger.New(config.DefaultConfig())
	if err != nil {
		t.Fatalf("logger.New failed: %v", err)
	}
	defer log.Close()

	f := New(log)
	if f == nil || f.log != log {
		t.Errorf("New did not wrap the provided logger")
	}
}

func TestFacade_LogQuery(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	ctx := context.Background()
	facade.LogQuery(ctx, "postgres", "SELECT * FROM users", 12*time.Millisecond,
		slog.Int64(string(core.FieldAffectedRows), 42),
	)

	out := read()
	for _, want := range []string{
		"database query",
		`"db_type":"postgres"`,
		`"db_query":"SELECT * FROM users"`,
		`"duration":"12ms"`,
		`"affected_rows":42`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestFacade_LogQuery_NoAttrs(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogQuery(context.Background(), "mysql", "SELECT 1", 1*time.Microsecond)

	out := read()
	if !strings.Contains(out, "database query") {
		t.Errorf("output should contain message: %s", out)
	}
	if !strings.Contains(out, `"db_type":"mysql"`) {
		t.Errorf("output should contain db_type: %s", out)
	}
}

func TestFacade_LogSlowQuery(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	duration := 500 * time.Millisecond
	threshold := 100 * time.Millisecond
	facade.LogSlowQuery(context.Background(), "postgres", "SELECT * FROM large_table", duration, threshold)

	out := read()
	for _, want := range []string{
		"slow database query",
		`"level":"WARN"`,
		`"db_type":"postgres"`,
		`"db_query":"SELECT * FROM large_table"`,
		`"is_slow_query":true`,
		`"duration":"100ms"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestFacade_LogSlowQuery_WithExtraAttrs(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogSlowQuery(context.Background(), "redis", "KEYS *", 250*time.Millisecond, 50*time.Millisecond,
		slog.String("host", "redis-0:6379"),
	)

	out := read()
	if !strings.Contains(out, `"host":"redis-0:6379"`) {
		t.Errorf("output should contain extra attr: %s", out)
	}
	if !strings.Contains(out, `"is_slow_query":true`) {
		t.Errorf("output should mark slow query: %s", out)
	}
}

func TestFacade_LogConnection(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogConnection(context.Background(), "postgres", "db.example.com:5432", "connected",
		slog.Int("pool_size", 10),
	)

	out := read()
	for _, want := range []string{
		"database connection",
		`"db_type":"postgres"`,
		`"host":"db.example.com:5432"`,
		`"status":"connected"`,
		`"pool_size":10`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestFacade_LogConnection_Failed(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogConnection(context.Background(), "mysql", "db.example.com:3306", "failed")

	out := read()
	if !strings.Contains(out, `"status":"failed"`) {
		t.Errorf("output should contain failed status: %s", out)
	}
}

func TestFacade_LogTransaction(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogTransaction(context.Background(), "tx-123", "commit", 5*time.Millisecond,
		slog.Int("statements", 3),
	)

	out := read()
	for _, want := range []string{
		"database transaction",
		`"transaction_id":"tx-123"`,
		`"tx_action":"commit"`,
		`"duration":"5ms"`,
		`"statements":3`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestFacade_LogTransaction_Rollback(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogTransaction(context.Background(), "tx-456", "rollback", 1*time.Millisecond)

	out := read()
	if !strings.Contains(out, `"tx_action":"rollback"`) {
		t.Errorf("output should contain rollback action: %s", out)
	}
}

func TestFacade_LogError(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	dbErr := errors.New("connection refused")
	facade.LogError(context.Background(), "postgres", "SELECT 1", dbErr,
		slog.String("host", "db.example.com:5432"),
	)

	out := read()
	for _, want := range []string{
		"database error",
		`"level":"ERROR"`,
		`"db_type":"postgres"`,
		`"db_query":"SELECT 1"`,
		`"db_error":"connection refused"`,
		`"host":"db.example.com:5432"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput: %s", want, out)
		}
	}
}

func TestFacade_LogError_NilError(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	facade.LogError(context.Background(), "mysql", "SELECT 1", nil)

	out := read()
	if !strings.Contains(out, "database error") {
		t.Errorf("output should contain message: %s", out)
	}
	// When err is nil, the db_error field should be omitted.
	if strings.Contains(out, "db_error") {
		t.Errorf("output should not contain db_error when err is nil: %s", out)
	}
}

func TestQueryTimer_StartAndStop(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	timer := Start(context.Background(), facade, "postgres", "SELECT pg_sleep(0.001)")
	// Simulate a small amount of work.
	time.Sleep(2 * time.Millisecond)
	d := timer.Stop()

	if d <= 0 {
		t.Errorf("Stop returned non-positive duration: %v", d)
	}
	out := read()
	if !strings.Contains(out, "database query") {
		t.Errorf("output should contain query log: %s", out)
	}
	if !strings.Contains(out, `"db_type":"postgres"`) {
		t.Errorf("output should contain db_type: %s", out)
	}
}

func TestQueryTimer_StopReturnsDuration(t *testing.T) {
	facade, cleanup, _ := newTestLogger(t)
	defer cleanup()

	timer := Start(context.Background(), facade, "postgres", "SELECT 1")
	time.Sleep(1 * time.Millisecond)
	d := timer.Stop()
	if d <= 0 {
		t.Errorf("Stop returned non-positive duration: %v", d)
	}
}

func TestQueryTimer_StopIdempotent(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	timer := Start(context.Background(), facade, "postgres", "SELECT 1")
	first := timer.Stop()
	// Sleep so that a second time.Since call would differ.
	time.Sleep(2 * time.Millisecond)
	second := timer.Stop()

	if first != second {
		t.Errorf("Stop not idempotent: first=%v second=%v", first, second)
	}

	// Ensure the query was logged only once.
	out := read()
	if cnt := strings.Count(out, "database query"); cnt != 1 {
		t.Errorf("expected query logged once, got %d times\noutput: %s", cnt, out)
	}
}

func TestQueryTimer_NilFacade(t *testing.T) {
	timer := Start(context.Background(), nil, "postgres", "SELECT 1")
	d := timer.Stop()
	if d != 0 {
		t.Errorf("Stop on nil facade should return 0, got %v", d)
	}
}

func TestQueryTimer_NilReceiver(t *testing.T) {
	// Calling Stop on a nil *QueryTimer must not panic.
	var timer *QueryTimer
	d := timer.Stop()
	if d != 0 {
		t.Errorf("Stop on nil receiver should return 0, got %v", d)
	}
}

func TestQueryTimer_StopTwiceNoExtraLog(t *testing.T) {
	facade, cleanup, read := newTestLogger(t)
	defer cleanup()

	timer := Start(context.Background(), facade, "mysql", "SELECT 1")
	_ = timer.Stop()
	_ = timer.Stop()
	_ = timer.Stop()

	out := read()
	if cnt := strings.Count(out, "database query"); cnt != 1 {
		t.Errorf("expected exactly one query log, got %d\noutput: %s", cnt, out)
	}
}

func TestFacade_AllMethodsConcurrentSafe(t *testing.T) {
	// Smoke test that concurrent use does not race. Run with -race.
	facade, cleanup, _ := newTestLogger(t)
	defer cleanup()

	ctx := context.Background()
	done := make(chan struct{})
	work := func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			facade.LogQuery(ctx, "postgres", "SELECT 1", 1*time.Millisecond)
			facade.LogSlowQuery(ctx, "postgres", "SELECT 2", 200*time.Millisecond, 100*time.Millisecond)
			facade.LogConnection(ctx, "postgres", "h:5432", "connected")
			facade.LogTransaction(ctx, "tx", "commit", 1*time.Millisecond)
			facade.LogError(ctx, "postgres", "SELECT 3", errors.New("boom"))
		}
	}
	go work()
	<-done
}
