package core

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestLevelFilter(t *testing.T) {
	f := LevelFilter{Min: LevelInfo}
	if !f.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("LevelFilter should allow INFO when min=INFO")
	}
	if !f.Allow(context.Background(), LevelError, "msg") {
		t.Error("LevelFilter should allow ERROR when min=INFO")
	}
	if f.Allow(context.Background(), LevelDebug, "msg") {
		t.Error("LevelFilter should not allow DEBUG when min=INFO")
	}
}

func TestFilterChain(t *testing.T) {
	chain := NewFilterChain(
		LevelFilter{Min: LevelInfo},
		&SamplingFilter{Rate: 1.0},
	)
	if !chain.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("chain should allow INFO with all-pass filters")
	}
	if chain.Allow(context.Background(), LevelDebug, "msg") {
		t.Error("chain should not allow DEBUG with min=INFO")
	}
}

func TestSamplingFilter(t *testing.T) {
	// Rate 1.0 = all pass.
	f := NewSamplingFilter(1.0)
	for i := 0; i < 10; i++ {
		if !f.Allow(context.Background(), LevelInfo, "msg") {
			t.Error("SamplingFilter with rate 1.0 should allow all")
		}
	}

	// Rate 0.0 = none pass.
	f0 := NewSamplingFilter(0.0)
	if f0.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("SamplingFilter with rate 0.0 should allow none")
	}

	// Rate 0.5 = ~50% pass (every 2nd).
	f50 := NewSamplingFilter(0.5)
	passed := 0
	for i := 0; i < 100; i++ {
		if f50.Allow(context.Background(), LevelInfo, "msg") {
			passed++
		}
	}
	if passed != 50 {
		t.Errorf("SamplingFilter 0.5 passed %d/100, want 50", passed)
	}
}

func TestDedupFilter(t *testing.T) {
	f := NewDedupFilter(5*time.Minute, 3, 1000)

	// First 3 occurrences pass.
	for i := 0; i < 3; i++ {
		if !f.Allow(context.Background(), LevelError, "same error") {
			t.Errorf("occurrence %d should pass", i+1)
		}
	}
	// 4th occurrence should be dropped.
	if f.Allow(context.Background(), LevelError, "same error") {
		t.Error("4th occurrence should be dropped")
	}
	// Different message should pass.
	if !f.Allow(context.Background(), LevelError, "different error") {
		t.Error("different message should pass")
	}
}

func TestDedupFilter_WindowExpiry(t *testing.T) {
	f := NewDedupFilter(50*time.Millisecond, 2, 1000)

	f.Allow(context.Background(), LevelInfo, "msg")
	f.Allow(context.Background(), LevelInfo, "msg")
	// 3rd should be dropped.
	if f.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("3rd occurrence should be dropped")
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)
	// After window expiry, should pass again.
	if !f.Allow(context.Background(), LevelInfo, "msg") {
		t.Error("after window expiry, message should pass")
	}
}

func TestHookChain(t *testing.T) {
	called := map[string]bool{}
	hook1 := &testHook{name: "hook1", called: called}
	hook2 := &testHook{name: "hook2", called: called}

	chain := NewHookChain(hook1, hook2)

	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	modified, err := chain.BeforeWrite(context.Background(), record)
	if err != nil {
		t.Fatalf("BeforeWrite failed: %v", err)
	}
	if !called["hook1_before"] || !called["hook2_before"] {
		t.Error("both hooks' BeforeWrite should be called")
	}

	chain.AfterWrite(context.Background(), modified, []byte("payload"), nil)
	if !called["hook1_after"] || !called["hook2_after"] {
		t.Error("both hooks' AfterWrite should be called")
	}
}

type testHook struct {
	name   string
	called map[string]bool
}

func (h *testHook) BeforeWrite(_ context.Context, r slog.Record) (slog.Record, error) {
	h.called[h.name+"_before"] = true
	return r, nil
}

func (h *testHook) AfterWrite(_ context.Context, _ slog.Record, _ []byte, _ error) {
	h.called[h.name+"_after"] = true
}

func (h *testHook) Name() string { return h.name }

func TestNoopHook(t *testing.T) {
	h := NoopHook{}
	record := slog.NewRecord(time.Now(), LevelInfo, "test", 0)
	r, err := h.BeforeWrite(context.Background(), record)
	if err != nil {
		t.Errorf("NoopHook.BeforeWrite should not error: %v", err)
	}
	if r.Message != "test" {
		t.Error("NoopHook should not modify record")
	}
	h.AfterWrite(context.Background(), record, nil, nil)
	if h.Name() != "noop" {
		t.Error("NoopHook name should be 'noop'")
	}
}

func TestDynamicLevel(t *testing.T) {
	dl := NewDynamicLevel(LevelInfo)
	if dl.Get() != LevelInfo {
		t.Error("initial level should be INFO")
	}
	if !dl.Enabled(context.Background(), LevelError) {
		t.Error("ERROR should be enabled when level is INFO")
	}
	if dl.Enabled(context.Background(), LevelDebug) {
		t.Error("DEBUG should not be enabled when level is INFO")
	}

	dl.Set(LevelDebug)
	if dl.Get() != LevelDebug {
		t.Error("level should be DEBUG after Set")
	}
	if !dl.Enabled(context.Background(), LevelDebug) {
		t.Error("DEBUG should be enabled after Set(DEBUG)")
	}
	if dl.String() != "DEBUG" {
		t.Errorf("String() = %q, want 'DEBUG'", dl.String())
	}
}

func TestCapturePanic_NoPanic(t *testing.T) {
	info, panicked := CapturePanic(func() {
		// normal execution
	})
	if panicked {
		t.Error("panicked should be false when no panic")
	}
	if info.Message != "" {
		t.Error("Message should be empty when no panic")
	}
}

func TestCapturePanic_WithPanic(t *testing.T) {
	info, panicked := CapturePanic(func() {
		panic("test panic")
	})
	if !panicked {
		t.Error("panicked should be true")
	}
	if info.Message != "test panic" {
		t.Errorf("Message = %q, want 'test panic'", info.Message)
	}
	if info.Stack == "" {
		t.Error("Stack should not be empty")
	}
}

func TestCapturePanic_WithError(t *testing.T) {
	info, panicked := CapturePanic(func() {
		panic(ErrSinkClosed)
	})
	if !panicked {
		t.Error("panicked should be true")
	}
	if info.Message != ErrSinkClosed.Error() {
		t.Errorf("Message = %q, want %q", info.Message, ErrSinkClosed.Error())
	}
}
