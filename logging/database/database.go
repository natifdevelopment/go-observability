// Package database provides query logging helpers for database operations.
//
// It wraps the main logger.Logger facade with database-specific methods
// that emit structured log records using the standard core.Field* field
// names. This makes database logs queryable and consistent across
// services in Grafana Loki, Elasticsearch, and Datadog.
//
// # Quick Start
//
//	log, err := logger.New(logger.FromEnv())
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
//	db := database.New(log)
//	ctx := context.Background()
//
//	// Log a query.
//	db.LogQuery(ctx, "postgres", "SELECT * FROM users", 12*time.Millisecond)
//
//	// Time a query with QueryTimer.
//	timer := database.Start(ctx, db, "postgres", "SELECT * FROM users")
//	defer timer.Stop()
//
// # Field Usage
//
// All methods attach the standard database fields:
//   - core.FieldDBType  (db_type)
//   - core.FieldDBQuery (db_query)
//   - core.FieldDuration (duration)
//   - core.FieldTransactionID (transaction_id)
//   - core.FieldTxAction (tx_action)
//   - core.FieldDBError (db_error)
//   - core.FieldIsSlowQuery (is_slow_query)
//
// Additional caller-supplied slog.Attr values are appended after the
// standard fields, allowing per-call enrichment (e.g. rows affected).
package database

import (
	"context"
	"log/slog"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Facade wraps a *logger.Logger with database-specific logging helpers.
// It is safe for concurrent use: the underlying logger is immutable and
// goroutine-safe, and Facade holds no mutable state of its own.
type Facade struct {
	log *logger.Logger
}

// New creates a database logging Facade wrapping the provided logger.
// The logger must be non-nil; a nil logger will cause subsequent calls
// to panic when they delegate to the underlying logger.
func New(l *logger.Logger) *Facade {
	return &Facade{log: l}
}

// LogQuery logs a normal database query at DEBUG level.
//
// Parameters:
//   - dbType:   the database type/driver (e.g. "postgres", "mysql", "redis").
//   - query:    the query string (may be normalized/masked before logging).
//   - duration: the wall-clock duration the query took to execute.
//   - attrs:    additional structured attributes appended after the standard
//     fields (e.g. core.RowsAffected, host, etc.).
func (f *Facade) LogQuery(ctx context.Context, dbType, query string, duration time.Duration, attrs ...slog.Attr) {
	all := baseQueryAttrs(dbType, query, duration, attrs)
	f.log.Debug(ctx, "database query", all...)
}

// LogSlowQuery logs a slow database query at WARN level.
//
// A query is considered "slow" when its duration exceeds the configured
// threshold. The threshold is recorded so that operators can tune alerting
// without re-reading application code. The is_slow_query field is set to
// true to support simple log queries.
//
// Parameters:
//   - dbType:    the database type/driver.
//   - query:     the query string.
//   - duration:  the wall-clock duration the query took.
//   - threshold: the duration above which a query is considered slow.
//   - attrs:     additional structured attributes.
func (f *Facade) LogSlowQuery(ctx context.Context, dbType, query string, duration, threshold time.Duration, attrs ...slog.Attr) {
	all := baseQueryAttrs(dbType, query, duration, attrs)
	all = append(all,
		slog.String(string(core.FieldDuration), threshold.String()),
		slog.Bool(string(core.FieldIsSlowQuery), true),
	)
	f.log.Warn(ctx, "slow database query", all...)
}

// LogConnection logs a database connection event at INFO level.
//
// Use this to record connection lifecycle events such as "connected",
// "disconnected", "reconnected", or "pool_exhausted".
//
// Parameters:
//   - dbType: the database type/driver.
//   - host:   the host:port of the database server.
//   - status: a free-form status string (e.g. "connected", "failed").
//   - attrs:  additional structured attributes.
func (f *Facade) LogConnection(ctx context.Context, dbType, host, status string, attrs ...slog.Attr) {
	all := make([]slog.Attr, 0, len(attrs)+3)
	all = append(all,
		slog.String(string(core.FieldDBType), dbType),
		slog.String("host", host),
		slog.String("status", status),
	)
	all = append(all, attrs...)
	f.log.Info(ctx, "database connection", all...)
}

// LogTransaction logs a transaction lifecycle event at INFO level.
//
// Common actions include "begin", "commit", and "rollback". Recording
// the transaction id allows operators to correlate all queries that
// executed within the same transaction.
//
// Parameters:
//   - txID:     the transaction identifier.
//   - action:   the transaction action (begin/commit/rollback).
//   - duration: the duration of the action.
//   - attrs:    additional structured attributes.
func (f *Facade) LogTransaction(ctx context.Context, txID, action string, duration time.Duration, attrs ...slog.Attr) {
	all := make([]slog.Attr, 0, len(attrs)+3)
	all = append(all,
		slog.String(string(core.FieldTransactionID), txID),
		slog.String(string(core.FieldTxAction), action),
		slog.String(string(core.FieldDuration), duration.String()),
	)
	all = append(all, attrs...)
	f.log.Info(ctx, "database transaction", all...)
}

// LogError logs a database error at ERROR level.
//
// The error is recorded under the standard core.FieldDBError field so
// that database errors are distinguishable from generic application
// errors in log aggregation systems.
//
// Parameters:
//   - dbType: the database type/driver.
//   - query:  the query that triggered the error (may be empty).
//   - err:    the error returned by the database driver.
//   - attrs:  additional structured attributes.
func (f *Facade) LogError(ctx context.Context, dbType, query string, err error, attrs ...slog.Attr) {
	all := make([]slog.Attr, 0, len(attrs)+3)
	all = append(all,
		slog.String(string(core.FieldDBType), dbType),
		slog.String(string(core.FieldDBQuery), query),
	)
	if err != nil {
		all = append(all, slog.String(string(core.FieldDBError), err.Error()))
	}
	all = append(all, attrs...)
	f.log.Error(ctx, "database error", all...)
}

// baseQueryAttrs builds the standard attribute set shared by LogQuery
// and LogSlowQuery. It allocates a single slice sized to fit both the
// standard fields and any caller-supplied attributes.
func baseQueryAttrs(dbType, query string, duration time.Duration, attrs []slog.Attr) []slog.Attr {
	all := make([]slog.Attr, 0, len(attrs)+3)
	all = append(all,
		slog.String(string(core.FieldDBType), dbType),
		slog.String(string(core.FieldDBQuery), query),
		slog.String(string(core.FieldDuration), duration.String()),
	)
	all = append(all, attrs...)
	return all
}

// QueryTimer measures the duration of a database query and logs it
// automatically when Stop is called. It is intended for the common
// "time a query" pattern:
//
//	timer := database.Start(ctx, db, "postgres", "SELECT 1")
//	// ... execute query ...
//	timer.Stop()
//
// A zero-value QueryTimer (or one created with a nil Facade) is safe
// to call Stop on and is a no-op, returning a zero duration.
type QueryTimer struct {
	ctx      context.Context
	facade   *Facade
	dbType   string
	query    string
	start    time.Time
	duration time.Duration
	stopped  bool
}

// Start creates a QueryTimer that begins measuring immediately.
// The timer does not log until Stop is invoked. If facade is nil,
// the returned timer's Stop is a no-op.
func Start(ctx context.Context, facade *Facade, dbType, query string) *QueryTimer {
	return &QueryTimer{
		ctx:    ctx,
		facade: facade,
		dbType: dbType,
		query:  query,
		start:  time.Now(),
	}
}

// Stop stops the timer, logs the query at DEBUG level (via LogQuery),
// and returns the measured duration. Calling Stop more than once is
// safe: subsequent calls return the duration from the first Stop and
// do not log again.
func (t *QueryTimer) Stop() time.Duration {
	if t == nil || t.facade == nil {
		return 0
	}
	if t.stopped {
		// Return the originally measured duration without re-logging.
		return t.duration
	}
	t.stopped = true
	t.duration = time.Since(t.start)
	t.facade.LogQuery(t.ctx, t.dbType, t.query, t.duration)
	return t.duration
}
