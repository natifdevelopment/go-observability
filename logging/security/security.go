// Package security provides a high-level facade for security event logging.
//
// It wraps the logger.Logger with security-specific methods that emit
// structured log records carrying the standardized event_id field, enabling
// SIEM/Loki/ELK queries such as:
//
//	{event_id=~"security.auth.*"}
//	{event_id="security.injection.sql"}
//
// The facade covers OWASP-recommended security events and common attack
// patterns: authentication failures, brute-force detection, injection
// attacks (SQL/XSS/CSRF), authorization failures, JWT issues, rate
// limiting, and privilege escalation.
//
// # Quick Start
//
//	log, err := logger.New(logger.FromEnv())
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
//	sec := security.New(log)
//	sec.LoginFailed(ctx, "alice", "10.0.0.1")
//	sec.BruteForce(ctx, "10.0.0.1", 5)
//	sec.SQLInjection(ctx, "' OR 1=1 --", "10.0.0.1")
//
// Each method accepts variadic slog.Attr arguments so callers can attach
// additional context (e.g. user_agent, path, metadata) without losing the
// standardized fields emitted by the facade.
package security

import (
	"context"
	"log/slog"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Facade is the high-level facade for security event logging.
// It wraps a *logger.Logger and emits structured security events with
// the standardized event_id field.
type Facade struct {
	logger *logger.Logger
}

// New creates a new security logging Facade wrapping the given logger.
// The logger must be non-nil; a nil logger will cause subsequent method
// calls to panic.
func New(l *logger.Logger) *Facade {
	return &Facade{logger: l}
}

// LoginFailed logs a failed login attempt at WARN level.
// The emitted record carries event_id="security.auth.login_failed".
func (f *Facade) LoginFailed(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityLoginFailed)),
	)
	f.logger.Warn(ctx, "security: login failed", attrs...)
}

// LoginSuccess logs a successful login at INFO level.
// The emitted record carries event_id="security.auth.login_success".
func (f *Facade) LoginSuccess(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), "security.auth.login_success"),
	)
	f.logger.Info(ctx, "security: login success", attrs...)
}

// Logout logs a logout event at INFO level.
// The emitted record carries event_id="security.auth.logout".
func (f *Facade) Logout(ctx context.Context, username, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUsername), username),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), "security.auth.logout"),
	)
	f.logger.Info(ctx, "security: logout", attrs...)
}

// BruteForce logs a brute-force detection event at ERROR level.
// The emitted record carries event_id="security.ratelimit.brute_force".
func (f *Facade) BruteForce(ctx context.Context, ip string, attemptCount int, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldIP), ip),
		slog.Int(string(core.FieldAttemptCount), attemptCount),
		slog.String(string(core.FieldEventID), string(core.EventSecurityBruteForce)),
	)
	f.logger.Error(ctx, "security: brute force detected", attrs...)
}

// SQLInjection logs a detected SQL injection attempt at ERROR level.
// The emitted record carries event_id="security.injection.sql".
func (f *Facade) SQLInjection(ctx context.Context, query, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldQuery), query),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecuritySQLInjection)),
	)
	f.logger.Error(ctx, "security: SQL injection detected", attrs...)
}

// XSS logs a detected Cross-Site Scripting attempt at ERROR level.
// The emitted record carries event_id="security.injection.xss".
func (f *Facade) XSS(ctx context.Context, payload, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldDetectionInput), payload),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityXSS)),
	)
	f.logger.Error(ctx, "security: XSS detected", attrs...)
}

// CSRF logs a detected Cross-Site Request Forgery attempt at ERROR level.
// The emitted record carries event_id="security.injection.csrf".
func (f *Facade) CSRF(ctx context.Context, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityCSRF)),
	)
	f.logger.Error(ctx, "security: CSRF detected", attrs...)
}

// Unauthorized logs an unauthorized access attempt (no/failed authentication)
// at WARN level. The emitted record carries event_id="security.auth.unauthorized".
func (f *Facade) Unauthorized(ctx context.Context, userID, resource string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUserID), userID),
		slog.String(string(core.FieldPath), resource),
		slog.String(string(core.FieldEventID), string(core.EventSecurityUnauthorized)),
	)
	f.logger.Warn(ctx, "security: unauthorized access", attrs...)
}

// Forbidden logs a forbidden access attempt (authenticated but denied) at
// WARN level. The emitted record carries event_id="security.auth.forbidden".
func (f *Facade) Forbidden(ctx context.Context, userID, resource string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUserID), userID),
		slog.String(string(core.FieldPath), resource),
		slog.String(string(core.FieldEventID), string(core.EventSecurityForbidden)),
	)
	f.logger.Warn(ctx, "security: forbidden access", attrs...)
}

// JWTInvalid logs an invalid JWT token (bad signature/format) at ERROR level.
// The emitted record carries event_id="security.jwt.invalid".
func (f *Facade) JWTInvalid(ctx context.Context, token string, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMetadata), token),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityJWTInvalid)),
	)
	f.logger.Error(ctx, "security: invalid JWT", attrs...)
}

// JWTExpired logs an expired JWT token at WARN level.
// The emitted record carries event_id="security.jwt.expired".
func (f *Facade) JWTExpired(ctx context.Context, token string, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldMetadata), token),
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityJWTExpired)),
	)
	f.logger.Warn(ctx, "security: expired JWT", attrs...)
}

// RateLimited logs a rate-limit exceeded event at WARN level.
// The emitted record carries event_id="security.ratelimit.exceeded".
func (f *Facade) RateLimited(ctx context.Context, ip string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldIP), ip),
		slog.String(string(core.FieldEventID), string(core.EventSecurityRateLimited)),
	)
	f.logger.Warn(ctx, "security: rate limited", attrs...)
}

// PrivilegeEscalation logs a privilege escalation attempt at ERROR level.
// The emitted record carries event_id="security.rbac.privilege_escalation".
func (f *Facade) PrivilegeEscalation(ctx context.Context, userID, from, to string, attrs ...slog.Attr) {
	attrs = append(attrs,
		slog.String(string(core.FieldUserID), userID),
		slog.String(string(core.FieldRole), from),
		slog.String(string(core.FieldMetadata), to),
		slog.String(string(core.FieldEventID), string(core.EventSecurityPrivilegeEscalation)),
	)
	f.logger.Error(ctx, "security: privilege escalation", attrs...)
}
