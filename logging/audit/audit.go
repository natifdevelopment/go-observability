// Package audit provides a high-level facade for audit logging with hash
// chain integrity. It wraps the sink.AuditSink with a more ergonomic API
// for common audit event categories (login, data access, config changes).
//
// The Facade is a thin layer: all persistence and hash-chain logic lives in
// sink.AuditSink and core (ComputeAuditHash / VerifyAuditChain). This package
// only adds convenience constructors and typed helpers.
package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/sink"
)

// Facade is a high-level wrapper around sink.AuditSink that provides
// ergonomic helpers for the most common audit event categories.
//
// A Facade is safe for concurrent use because the underlying AuditSink is.
// The embedded *slog.Logger is used to record operational events (e.g. write
// failures) alongside the tamper-evident audit trail.
type Facade struct {
	sink   *sink.AuditSink
	logger *slog.Logger
}

// New creates a new audit Facade wrapping the given AuditSink.
//
// If logger is nil, a no-op logger (slog.Default() with level disabled) is
// used so that callers never need to nil-check before logging.
func New(sink *sink.AuditSink, logger *slog.Logger) *Facade {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.Level(100)}))
	}
	return &Facade{
		sink:   sink,
		logger: logger,
	}
}

// Log writes a single audit record via the underlying AuditSink.
// The payload is hashed and appended to the hash chain.
func (f *Facade) Log(ctx context.Context, payload core.AuditPayload) error {
	if f.sink == nil {
		return fmt.Errorf("audit: sink is not initialized")
	}
	if err := f.sink.WriteAudit(ctx, payload); err != nil {
		f.logger.Error("audit write failed",
			slog.String("user", payload.User),
			slog.String("action", payload.Action),
			slog.Any("err", err),
		)
		return err
	}
	return nil
}

// Login records an authentication event (login success or failure).
//
// action is typically "login" or "logout"; success is encoded in the
// Result metadata field as "success" or "failure". Additional structured
// attributes are merged into the payload Metadata.
func (f *Facade) Login(ctx context.Context, user, action, ip string, success bool, attrs ...slog.Attr) error {
	result := "failure"
	if success {
		result = "success"
	}
	md := map[string]any{
		"result": result,
	}
	for _, a := range attrs {
		md[a.Key] = a.Value.Any()
	}
	payload := NewPayload(user, action, ip)
	payload.Metadata = md
	return f.Log(ctx, payload)
}

// DataAccess records a data-access audit event (read/write/delete on a
// resource). Additional structured attributes are merged into Metadata.
func (f *Facade) DataAccess(ctx context.Context, user, resource, action string, attrs ...slog.Attr) error {
	md := map[string]any{}
	for _, a := range attrs {
		md[a.Key] = a.Value.Any()
	}
	payload := NewPayload(user, action, "")
	payload.Resource = resource
	payload.Metadata = md
	return f.Log(ctx, payload)
}

// ConfigChange records a configuration change audit event, capturing the
// before/after values for the given key.
func (f *Facade) ConfigChange(ctx context.Context, user, key, oldValue, newValue string) error {
	payload := NewPayload(user, "config_change", "")
	payload.Resource = key
	payload.Before = oldValue
	payload.After = newValue
	payload.Metadata = map[string]any{
		"key": key,
	}
	return f.Log(ctx, payload)
}

// Verify verifies the integrity of the audit hash chain by replaying it.
// Returns the index of the first tampered entry (-1 if the chain is valid)
// and an error if verification fails.
func (f *Facade) Verify(ctx context.Context) (int, error) {
	if f.sink == nil {
		return -1, fmt.Errorf("audit: sink is not initialized")
	}
	idx, err := f.sink.Verify(ctx)
	if err != nil {
		f.logger.Error("audit chain verification failed",
			slog.Int("tampered_index", idx),
			slog.Any("err", err),
		)
		return idx, err
	}
	if idx >= 0 {
		f.logger.Warn("audit chain tamper detected",
			slog.Int("tampered_index", idx),
		)
	}
	return idx, nil
}

// Close closes the underlying AuditSink. After Close, further Log calls
// will fail. Calling Close more than once is a no-op.
func (f *Facade) Close() error {
	if f.sink == nil {
		return nil
	}
	return f.sink.Close()
}

// NewPayload creates a basic AuditPayload with the given user, action and IP,
// and a current RFC3339 timestamp. It is the recommended starting point for
// building payloads before passing them to Facade.Log.
func NewPayload(user, action, ip string) core.AuditPayload {
	return core.AuditPayload{
		User:      user,
		Action:    action,
		IP:        ip,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
}
