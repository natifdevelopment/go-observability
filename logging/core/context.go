package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// Context helper functions for setting/getting individual Carrier fields
// in a context.Context. These are convenience wrappers around WithCarrier
// and CarrierFrom that preserve existing carrier fields (merge semantics).
//
// Usage:
//
//	ctx = core.WithTraceID(ctx, "abc123")
//	ctx = core.WithUser(ctx, "user123", "john", "admin")
//	traceID := core.TraceID(ctx) // "abc123"

// WithTraceID sets the trace_id in the context carrier.
func WithTraceID(ctx context.Context, id string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{TraceID: id}))
}

// WithRequestID sets the request_id in the context carrier.
func WithRequestID(ctx context.Context, id string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{RequestID: id}))
}

// WithCorrelationID sets the correlation_id in the context carrier.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{CorrelationID: id}))
}

// WithSessionID sets the session_id in the context carrier.
func WithSessionID(ctx context.Context, id string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{SessionID: id}))
}

// WithUser sets user_id, username, and role in the context carrier.
func WithUser(ctx context.Context, userID, username, role string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{
		UserID:   userID,
		Username: username,
		Role:     role,
	}))
}

// WithIP sets the client IP in the context carrier.
func WithIP(ctx context.Context, ip string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{IP: ip}))
}

// WithHTTP sets method and path in the context carrier.
func WithHTTP(ctx context.Context, method, path string) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{
		Method: method,
		Path:   path,
	}))
}

// WithStatusCode sets the status_code in the context carrier.
func WithStatusCode(ctx context.Context, code int) context.Context {
	return WithCarrier(ctx, MergeCarrier(CarrierFrom(ctx), Carrier{StatusCode: code}))
}

// WithExtra adds an extra field to the context carrier.
func WithExtra(ctx context.Context, key, value string) context.Context {
	c := CarrierFrom(ctx)
	extra := make(map[string]string, len(c.Extra)+1)
	for k, v := range c.Extra {
		extra[k] = v
	}
	extra[key] = value
	c.Extra = extra
	return WithCarrier(ctx, c)
}

// Accessor functions for retrieving individual carrier fields.

// TraceID returns the trace_id from the context carrier, or "" if not set.
func TraceID(ctx context.Context) string { return CarrierFrom(ctx).TraceID }

// RequestID returns the request_id from the context carrier, or "" if not set.
func RequestID(ctx context.Context) string { return CarrierFrom(ctx).RequestID }

// CorrelationID returns the correlation_id from the context carrier, or "" if not set.
func CorrelationID(ctx context.Context) string { return CarrierFrom(ctx).CorrelationID }

// SessionID returns the session_id from the context carrier, or "" if not set.
func SessionID(ctx context.Context) string { return CarrierFrom(ctx).SessionID }

// UserID returns the user_id from the context carrier, or "" if not set.
func UserID(ctx context.Context) string { return CarrierFrom(ctx).UserID }

// Username returns the username from the context carrier, or "" if not set.
func Username(ctx context.Context) string { return CarrierFrom(ctx).Username }

// Role returns the role from the context carrier, or "" if not set.
func Role(ctx context.Context) string { return CarrierFrom(ctx).Role }

// IP returns the client IP from the context carrier, or "" if not set.
func IP(ctx context.Context) string { return CarrierFrom(ctx).IP }

// Method returns the HTTP method from the context carrier, or "" if not set.
func Method(ctx context.Context) string { return CarrierFrom(ctx).Method }

// Path returns the HTTP path from the context carrier, or "" if not set.
func Path(ctx context.Context) string { return CarrierFrom(ctx).Path }

// StatusCode returns the status_code from the context carrier, or 0 if not set.
func StatusCode(ctx context.Context) int { return CarrierFrom(ctx).StatusCode }

// Extra returns an extra field value from the context carrier, or "" if not set.
func Extra(ctx context.Context, key string) string {
	c := CarrierFrom(ctx)
	if c.Extra == nil {
		return ""
	}
	return c.Extra[key]
}

// W3C TraceContext support for cross-service correlation.
// Spec: https://www.w3.org/TR/trace-context/
//
// traceparent header format: version-trace_id-parent_id-trace_flags
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01

const (
	// W3CTraceparentHeader is the standard W3C traceparent header name.
	W3CTraceparentHeader = "traceparent"
	// W3CTracestateHeader is the standard W3C tracestate header name.
	W3CTracestateHeader = "tracestate"
)

// ExtractTraceContext extracts W3C TraceContext from HTTP headers into a Carrier.
// The trace_id from traceparent is used as the Carrier's TraceID.
// If traceparent is absent or malformed, a zero-value Carrier is returned.
func ExtractTraceContext(headers http.Header) Carrier {
	tp := headers.Get(W3CTraceparentHeader)
	if tp == "" {
		return Carrier{}
	}
	parts := strings.Split(tp, "-")
	if len(parts) < 4 {
		return Carrier{}
	}
	// parts[0] = version, parts[1] = trace_id, parts[2] = parent_id, parts[3] = flags
	traceID := parts[1]
	if len(traceID) != 32 {
		return Carrier{}
	}
	return Carrier{TraceID: traceID}
}

// InjectTraceContext adds a traceparent header to outbound HTTP request headers
// based on the Carrier in the context. Enables cross-service trace propagation.
// If no Carrier or trace_id is present, headers are returned unchanged.
func InjectTraceContext(ctx context.Context, headers http.Header) http.Header {
	if headers == nil {
		headers = http.Header{}
	}
	c := CarrierFrom(ctx)
	if c.TraceID == "" {
		return headers
	}
	// Generate a W3C-compliant traceparent: version 00, trace_id from carrier,
	// random 8-byte parent_id, flags 01 (sampled).
	tp := "00-" + c.TraceID + "-" + newParentID() + "-01"
	headers.Set(W3CTraceparentHeader, tp)
	return headers
}

// newParentID generates a random 16-hex-character parent_id for W3C traceparent.
// Falls back to zeros if the random source fails.
func newParentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
