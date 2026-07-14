package core

import (
	"errors"
	"fmt"
)

// Sentinel errors used across the framework.
// Callers can use errors.Is(err, core.ErrSinkFull) for HA decision-making.

var (
	// ErrSinkClosed is returned when Write is called after Close.
	ErrSinkClosed = errors.New("logger: sink is closed")

	// ErrSinkFull is returned when async sink buffer is full and policy is Block.
	ErrSinkFull = errors.New("logger: sink buffer is full")

	// ErrSinkTimeout is returned when sink write exceeds configured timeout.
	ErrSinkTimeout = errors.New("logger: sink write timed out")

	// ErrSinkWrite is returned when sink write fails after retries.
	ErrSinkWrite = errors.New("logger: sink write failed")

	// ErrMaskFailed is returned when masking encounters an unrecoverable error.
	// Masking should never fail (fail-secure: drop field instead), so this is rare.
	ErrMaskFailed = errors.New("logger: masking failed")

	// ErrInvalidConfig is returned when configuration validation fails.
	ErrInvalidConfig = errors.New("logger: invalid configuration")

	// ErrInvalidLevel is returned when level string cannot be parsed.
	ErrInvalidLevel = errors.New("logger: invalid log level")

	// ErrEventNotFound is returned when an event ID is not in the registry.
	ErrEventNotFound = errors.New("logger: event not found")

	// ErrEventAlreadyRegistered is returned when registering a duplicate event ID.
	ErrEventAlreadyRegistered = errors.New("logger: event already registered")

	// ErrAuditHashMismatch is returned when audit chain verification detects tampering.
	ErrAuditHashMismatch = errors.New("logger: audit hash chain mismatch (tampered)")

	// ErrAuditFileOpen is returned when audit file cannot be opened.
	ErrAuditFileOpen = errors.New("logger: cannot open audit file")

	// ErrAuditWrite is returned when audit record write fails.
	ErrAuditWrite = errors.New("logger: audit write failed")

	// ErrPathTraversal is returned when LOG_FILE path contains traversal sequences.
	ErrPathTraversal = errors.New("logger: path traversal detected in log file path")

	// ErrBuilderUsed is returned when Builder is used after Build().
	ErrBuilderUsed = errors.New("logger: builder already used (call Build only once)")

	// ErrLoggerClosed is returned when logging after Logger.Close().
	ErrLoggerClosed = errors.New("logger: logger is closed")

	// ErrFlushTimeout is returned when async flush exceeds timeout.
	ErrFlushTimeout = errors.New("logger: flush timed out")

	// ErrCircuitBreakerOpen is returned when sink circuit breaker is open.
	ErrCircuitBreakerOpen = errors.New("logger: sink circuit breaker is open")
)

// WrapError wraps an error with context message.
// Uses fmt.Errorf with %w for errors.Is/As compatibility.
func WrapError(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
