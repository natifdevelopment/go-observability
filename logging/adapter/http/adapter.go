// Package http provides standard library net/http middleware that integrates
// with the enterprise logging framework.
//
// The middleware logs each HTTP request's start and completion with structured
// fields (method, path, status, duration, bytes written, client IP, user-agent)
// using the standard core.Field* field names so that HTTP logs are queryable
// and consistent across services in Grafana Loki, Elasticsearch, and Datadog.
//
// # Quick Start
//
//	log, err := logger.New(logger.FromEnv())
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/api", handler)
//
//	// Wrap the mux with logging middleware.
//	server := &http.Server{
//	    Addr:    ":8080",
//	    Handler: http.Middleware(log, mux),
//	}
//	server.ListenAndServe()
//
// # Options
//
//	middleware := http.Middleware(log,
//	    http.WithSkipPaths("/healthz", "/metrics"),
//	    http.WithLogBody(4096),               // log bodies up to 4KB
//	    http.WithSlowThreshold(500*time.Millisecond),
//	)
//
// # Carrier Extraction
//
// The middleware extracts the context carrier from the incoming request's
// context (trace_id, request_id, user_id, etc.) so that all log records
// emitted for a request are correlated. W3C traceparent headers are also
// extracted via core.ExtractTraceContext when present.
package http

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/helper"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// Config holds configuration for the HTTP logging middleware.
type Config struct {
	// SkipPaths is a list of request paths for which logging is skipped
	// entirely (both start and completion logs). Useful for noisy
	// endpoints such as /healthz or /metrics.
	SkipPaths []string

	// LogBody enables request/response body logging. When true, the
	// request body is captured (up to MaxBodyBytes) and logged under the
	// core.FieldRequestBody field. The response body is likewise captured
	// and logged under core.FieldResponseBody.
	LogBody bool

	// MaxBodyBytes is the maximum number of body bytes to capture and log
	// when LogBody is enabled. Bodies larger than this are truncated and
	// a "body_truncated" field is set to true. Defaults to
	// core.DefaultBodyMaxBytes (1024) when zero.
	MaxBodyBytes int

	// SlowRequestThreshold is the duration above which a request is
	// considered slow and an additional WARN-level "slow request" log
	// record is emitted. A zero value disables slow request detection.
	SlowRequestThreshold time.Duration
}

// Option configures the middleware Config. Apply with the functional-options
// pattern via Middleware/Handler.
type Option func(*Config)

// WithSkipPaths adds request paths to the SkipPaths list. Requests whose
// URL.Path exactly matches one of these paths are not logged.
func WithSkipPaths(paths ...string) Option {
	return func(c *Config) {
		c.SkipPaths = append(c.SkipPaths, paths...)
	}
}

// WithLogBody enables request/response body logging, capturing up to
// maxBytes. If maxBytes is <= 0, the default (core.DefaultBodyMaxBytes)
// is used.
func WithLogBody(maxBytes int) Option {
	return func(c *Config) {
		c.LogBody = true
		if maxBytes > 0 {
			c.MaxBodyBytes = maxBytes
		} else {
			c.MaxBodyBytes = core.DefaultBodyMaxBytes
		}
	}
}

// WithSlowThreshold sets the duration above which a request is considered
// slow. A zero or negative value disables slow request detection.
func WithSlowThreshold(d time.Duration) Option {
	return func(c *Config) {
		if d > 0 {
			c.SlowRequestThreshold = d
		}
	}
}

// ResponseRecorder wraps http.ResponseWriter to capture the status code
// and the number of bytes written to the body. It also buffers the response
// body when body logging is enabled so that it can be included in the
// completion log.
type ResponseRecorder struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int
	bodyBuf       *bytes.Buffer
	logBody       bool
	maxBodyBytes  int
	headerWritten bool
}

// NewResponseRecorder creates a ResponseRecorder wrapping the given
// ResponseWriter. When logBody is true, the response body is captured into
// an internal buffer (up to maxBodyBytes) and replayed to the client.
func NewResponseRecorder(w http.ResponseWriter, logBody bool, maxBodyBytes int) *ResponseRecorder {
	r := &ResponseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		logBody:        logBody,
		maxBodyBytes:   maxBodyBytes,
	}
	if logBody {
		r.bodyBuf = &bytes.Buffer{}
	}
	return r
}

// WriteHeader captures the status code and forwards it to the underlying
// ResponseWriter. It is safe to call multiple times; only the first call
// records the status code and forwards it.
func (r *ResponseRecorder) WriteHeader(code int) {
	if r.headerWritten {
		return
	}
	r.headerWritten = true
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written and, when body logging is
// enabled, buffers the response body (up to maxBodyBytes) before forwarding
// the bytes to the client.
func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n
	if r.logBody && r.bodyBuf != nil && err == nil {
		remaining := r.maxBodyBytes - r.bodyBuf.Len()
		if remaining > 0 {
			if len(b) <= remaining {
				r.bodyBuf.Write(b)
			} else {
				r.bodyBuf.Write(b[:remaining])
			}
		}
	}
	return n, err
}

// StatusCode returns the captured HTTP status code.
func (r *ResponseRecorder) StatusCode() int {
	return r.statusCode
}

// BytesWritten returns the total number of bytes written to the body.
func (r *ResponseRecorder) BytesWritten() int {
	return r.bytesWritten
}

// Body returns the captured response body (truncated to MaxBodyBytes).
// Returns nil when body logging is disabled.
func (r *ResponseRecorder) Body() []byte {
	if r.bodyBuf == nil {
		return nil
	}
	return r.bodyBuf.Bytes()
}

// Flush delegates to the underlying ResponseWriter when it implements
// http.Flusher, enabling streaming responses.
func (r *ResponseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying ResponseWriter so that http.ResponseController
// can access additional interfaces (Hijacker, Flusher, Pusher, etc.).
func (r *ResponseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Middleware returns an HTTP middleware that logs each request handled by
// the next handler using the provided logger. The middleware:
//
//   - Logs request start (method, path, client IP, user-agent) at DEBUG level.
//   - Logs request completion (method, path, status, duration, bytes written)
//     at a level derived from the status code via helper.StatusLevel.
//   - Logs slow requests at WARN level when the duration exceeds the
//     configured SlowRequestThreshold.
//   - Skips logging entirely for paths listed in SkipPaths.
//   - Extracts the context carrier from the request context so that
//     trace_id/request_id/user_id are correlated across log records.
//   - Optionally captures and logs request/response bodies when LogBody
//     is enabled.
//
// A nil logger causes the middleware to return the handler unchanged.
func Middleware(log *logger.Logger, opts ...Option) func(http.Handler) http.Handler {
	cfg := &Config{
		MaxBodyBytes: core.DefaultBodyMaxBytes,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		if log == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for configured paths.
			if _, ok := skip[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			// Extract W3C trace context from headers and merge into the
			// request context so downstream handlers and log records
			// carry the trace_id.
			if carrier := core.ExtractTraceContext(r.Header); !carrier.IsZero() {
				ctx = core.WithCarrier(ctx, core.MergeCarrier(core.CarrierFrom(ctx), carrier))
			}

			// Capture the request body when body logging is enabled.
			var reqBody []byte
			if cfg.LogBody && r.Body != nil {
				limit := cfg.MaxBodyBytes
				if limit <= 0 {
					limit = core.DefaultBodyMaxBytes
				}
				bodyBytes, err := io.ReadAll(r.Body)
				if err == nil {
					if len(bodyBytes) > limit {
						reqBody = bodyBytes[:limit]
					} else {
						reqBody = bodyBytes
					}
					// Restore the full body for downstream handlers.
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// Log request start.
			startAttrs := []slog.Attr{
				slog.String(string(core.FieldMethod), r.Method),
				slog.String(string(core.FieldPath), r.URL.Path),
				slog.String(string(core.FieldIP), helper.ClientIP(r)),
				slog.String(string(core.FieldUserAgent), r.UserAgent()),
			}
			if cfg.LogBody && len(reqBody) > 0 {
				startAttrs = append(startAttrs, slog.String(string(core.FieldRequestBody), string(reqBody)))
			}
			log.Debug(ctx, "http request started", startAttrs...)

			// Wrap the ResponseWriter to capture status and bytes.
			rec := NewResponseRecorder(w, cfg.LogBody, cfg.MaxBodyBytes)

			start := time.Now()
			next.ServeHTTP(rec, r.WithContext(ctx))
			duration := time.Since(start)

			// Build completion attributes.
			completeAttrs := []slog.Attr{
				slog.String(string(core.FieldMethod), r.Method),
				slog.String(string(core.FieldPath), r.URL.Path),
				slog.Int(string(core.FieldStatusCode), rec.StatusCode()),
				slog.Int(string(core.FieldResponseSize), rec.BytesWritten()),
				slog.Duration(string(core.FieldDuration), duration),
				slog.String(string(core.FieldIP), helper.ClientIP(r)),
			}
			if cfg.LogBody && len(rec.Body()) > 0 {
				completeAttrs = append(completeAttrs, slog.String(string(core.FieldResponseBody), string(rec.Body())))
			}

			// Log completion at a level appropriate for the status code.
			level := helper.StatusLevel(rec.StatusCode())
			switch level {
			case core.LevelError:
				log.Error(ctx, "http request completed", completeAttrs...)
			case core.LevelWarn:
				log.Warn(ctx, "http request completed", completeAttrs...)
			default:
				log.Info(ctx, "http request completed", completeAttrs...)
			}

			// Log slow requests.
			if cfg.SlowRequestThreshold > 0 && duration > cfg.SlowRequestThreshold {
				slowAttrs := append(completeAttrs,
					slog.String("threshold", cfg.SlowRequestThreshold.String()),
					slog.Bool("is_slow_request", true),
				)
				log.Warn(ctx, "slow http request", slowAttrs...)
			}
		})
	}
}

// Handler is a convenience wrapper that applies Middleware to the given
// handler and returns the wrapped handler. It is equivalent to:
//
//	http.Middleware(log, opts...)(handler)
//
// A nil logger causes the handler to be returned unchanged.
func Handler(log *logger.Logger, handler http.Handler, opts ...Option) http.Handler {
	return Middleware(log, opts...)(handler)
}
