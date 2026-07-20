// Package external provides structured logging for external service calls.
//
// It wraps the main logger.Logger with convenience methods for logging
// outbound calls to external services such as HTTP APIs, gRPC services,
// message brokers, or any third-party dependency.
//
// # Quick Start
//
//	log, _ := logger.New(logger.FromEnv())
//	defer log.Close()
//
//	ext := external.New(log)
//	ext.LogCall(ctx, "payment-api", "POST /charge", "POST", 200, 150*time.Millisecond)
//	ext.LogError(ctx, "payment-api", "POST /charge", err)
//
// # Call Timer
//
//	timer := external.Start(ctx, ext, "payment-api", "POST /charge", "POST")
//	// ... perform the call ...
//	timer.Stop(resp.StatusCode)
package external

import (
	"context"
	"log/slog"
	"time"

	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Facade wraps a *logger.Logger with external-service-call logging methods.
//
// All methods are context-first and accept variadic slog.Attr for
// additional structured fields. A nil Facade is safe to call: every
// method guards against a nil receiver or nil underlying logger.
type Facade struct {
	logger *logger.Logger
}

// New creates a new external-service logging Facade wrapping the given logger.
// Returns nil if the provided logger is nil.
func New(l *logger.Logger) *Facade {
	if l == nil {
		return nil
	}
	return &Facade{logger: l}
}

// LogCall logs a completed external service call at INFO level.
//
// Typical fields recorded: service, endpoint, method, status, duration.
// Additional attributes (e.g. request_id, retry_count) can be appended.
func (f *Facade) LogCall(ctx context.Context, service, endpoint, method string, status int, duration time.Duration, attrs ...slog.Attr) {
	if f == nil || f.logger == nil {
		return
	}
	attrs = append(attrs,
		slog.String("service", service),
		slog.String("endpoint", endpoint),
		slog.String("method", method),
		slog.Int("status", status),
		slog.Duration("duration", duration),
	)
	f.logger.Info(ctx, "external: service call", attrs...)
}

// LogError logs an external service call error at ERROR level.
//
// The error is recorded as a string field. If err is nil, no error
// field is appended but the call is still logged.
func (f *Facade) LogError(ctx context.Context, service, endpoint string, err error, attrs ...slog.Attr) {
	if f == nil || f.logger == nil {
		return
	}
	attrs = append(attrs,
		slog.String("service", service),
		slog.String("endpoint", endpoint),
	)
	if err != nil {
	}
	f.logger.Error(ctx, "external: service error", attrs...)
}

// LogTimeout logs an external service call timeout at WARN level.
//
// The configured/elapsed timeout duration is recorded so operators can
// correlate slow dependencies.
func (f *Facade) LogTimeout(ctx context.Context, service, endpoint string, timeout time.Duration, attrs ...slog.Attr) {
	if f == nil || f.logger == nil {
		return
	}
	attrs = append(attrs,
		slog.String("service", service),
		slog.String("endpoint", endpoint),
		slog.Duration("timeout", timeout),
	)
	f.logger.Warn(ctx, "external: service timeout", attrs...)
}

// LogRetry logs a retry attempt for an external service call at WARN level.
//
// attempt is the current attempt number (1-based) and maxAttempts is the
// configured maximum number of retries.
func (f *Facade) LogRetry(ctx context.Context, service, endpoint string, attempt, maxAttempts int, attrs ...slog.Attr) {
	if f == nil || f.logger == nil {
		return
	}
	attrs = append(attrs,
		slog.String("service", service),
		slog.String("endpoint", endpoint),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", maxAttempts),
	)
	f.logger.Warn(ctx, "external: service retry", attrs...)
}

// LogCircuitBreaker logs a circuit breaker state change at WARN level.
//
// state is the new circuit breaker state (e.g. "open", "closed", "half-open").
func (f *Facade) LogCircuitBreaker(ctx context.Context, service, state string, attrs ...slog.Attr) {
	if f == nil || f.logger == nil {
		return
	}
	attrs = append(attrs,
		slog.String("service", service),
		slog.String("state", state),
	)
	f.logger.Warn(ctx, "external: circuit breaker state change", attrs...)
}

// CallTimer measures the duration of an external service call.
//
// Create one with Start before performing the call, then call Stop with
// the resulting HTTP/gRPC status code when finished. Stop logs the call
// via the owning Facade and returns the elapsed duration.
//
//	timer := external.Start(ctx, ext, "payment-api", "POST /charge", "POST")
//	resp, err := client.Do(req)
//	if err != nil {
//	    ext.LogError(ctx, "payment-api", "POST /charge", err)
//	    timer.Stop(0)
//	    return
//	}
//	timer.Stop(resp.StatusCode)
type CallTimer struct {
	facade   *Facade
	ctx      context.Context
	service  string
	endpoint string
	method   string
	start    time.Time
}

// Start creates a CallTimer that begins measuring immediately.
// The returned timer is safe to use even if facade is nil; Stop will
// simply return a zero duration in that case.
func Start(ctx context.Context, facade *Facade, service, endpoint, method string) *CallTimer {
	return &CallTimer{
		facade:   facade,
		ctx:      ctx,
		service:  service,
		endpoint: endpoint,
		method:   method,
		start:    time.Now(),
	}
}

// Stop ends timing and logs the external service call with the given status.
// It returns the elapsed duration since Start was called.
// If the timer's facade is nil, no logging occurs and the elapsed duration
// is still returned.
func (t *CallTimer) Stop(status int) time.Duration {
	if t == nil {
		return 0
	}
	duration := time.Since(t.start)
	if t.facade == nil {
		return duration
	}
	t.facade.LogCall(t.ctx, t.service, t.endpoint, t.method, status, duration)
	return duration
}
