package core

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
)

// Stacktrace returns the current goroutine's stack trace as a string.
// Uses runtime/debug.Stack which includes all frames.
//
// This is expensive (~µs) and should only be called for ERROR/FATAL/PANIC
// levels, not in the hot path of INFO/DEBUG logs.
func Stacktrace() string {
	return string(debug.Stack())
}

// PanicInfo holds information captured during a panic.
type PanicInfo struct {
	Message   string // the panic message (from recover())
	Stack     string // full stack trace
	Function  string // function where panic occurred (best-effort)
	File      string // file where panic occurred (best-effort)
	Line      int    // line where panic occurred (best-effort)
	Recovered any    // the raw recovered value
}

// CapturePanic runs fn and captures any panic that occurs.
// Returns PanicInfo (zero-value if no panic) and a boolean indicating
// whether a panic was caught.
//
// This function does NOT log the panic — the caller is responsible for
// logging (separation of concerns). This makes it reusable across
// all adapters (gin, echo, fiber, grpc, net/http) without duplication.
//
// Example:
//
//	info, panicked := core.CapturePanic(func() {
//	    doRiskyWork()
//	})
//	if panicked {
//	    logger.Error(ctx, "panic recovered",
//	        slog.String("panic_message", info.Message),
//	        slog.String("stacktrace", info.Stack),
//	    )
//	}
func CapturePanic(fn func()) (info PanicInfo, panicked bool) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				info.Recovered = r
				info.Message = panicMessage(r)
				info.Stack = Stacktrace()
				info.Function, info.File, info.Line = extractFirstUserFrame(info.Stack)
			}
		}()
		fn()
	}()
	return info, panicked
}

// CapturePanicCtx is like CapturePanic but accepts a context.
// The context is not used for panic capture itself, but is provided
// so the caller can pass it through to logging after capture.
func CapturePanicCtx(ctx context.Context, fn func()) (PanicInfo, bool) {
	_ = ctx
	return CapturePanic(fn)
}

// panicMessage converts a recovered value to a string message.
func panicMessage(r any) string {
	if r == nil {
		return "<nil>"
	}
	switch v := r.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return sprintAny(r)
	}
}

// extractFirstUserFrame parses a stack trace string and returns
// the function, file, and line of the first user-code frame.
func extractFirstUserFrame(stack string) (fn, file string, line int) {
	lines := strings.Split(stack, "\n")
	// Skip goroutine header (first 2-3 lines until blank/tab-only line).
	start := 0
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			start = i + 1
			break
		}
	}
	// Frames: each frame is 2 lines (function + file:line).
	// Skip runtime/panic/CapturePanic frames.
	for i := start; i+1 < len(lines); i += 2 {
		fname := strings.TrimSpace(lines[i])
		if isUserFrame(fname) {
			fileLine := strings.TrimSpace(lines[i+1])
			fn = trimFuncName(fname)
			file, line = parseFileLine(fileLine)
			return
		}
	}
	return
}

func isUserFrame(fn string) bool {
	return !strings.Contains(fn, "runtime.") &&
		!strings.Contains(fn, "panic") &&
		!strings.Contains(fn, "core.CapturePanic")
}

func trimFuncName(fn string) string {
	// Keep last 2 path segments for readability: "bbo/logger/core.foo".
	parts := strings.Split(fn, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return fn
}

func parseFileLine(s string) (string, int) {
	idx := strings.LastIndex(s, ":")
	if idx < 0 {
		return s, 0
	}
	file := s[:idx]
	line, _ := strconv.Atoi(s[idx+1:])
	return file, line
}

// sprintAny is a minimal any→string converter for panic values.
func sprintAny(v any) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprint(v)
}
