package core

import "context"

// Carrier carries trace/request/correlation IDs and user context
// across goroutines via context.Context.
//
// Design:
//   - Immutable after construction. Mutations create a new instance
//     (functional update) → thread-safe without mutex.
//   - Stored in context.Context via a typed (unexported) key to prevent
//     collision with other packages' context values (security: anti-injection).
//
// Monitoring compatibility:
//   - All fields map directly to standard log fields (trace_id, request_id, etc.)
//   - These IDs enable cross-service correlation in Grafana/Loki/Tempo/Jaeger.
type Carrier struct {
	TraceID       string
	RequestID     string
	CorrelationID string
	SessionID     string
	UserID        string
	Username      string
	Role          string
	IP            string
	Method        string
	Path          string
	StatusCode    int
	// Extra holds additional carrier fields not in the standard set.
	// Use for service-specific context (e.g., tenant_id, branch_code).
	Extra map[string]string
}

// carrierKey is an unexported typed key for context.Value.
// Using a struct type (not string) prevents collision with other packages.
type carrierKey struct{}

var carrierCtxKey = carrierKey{}

// WithCarrier stores a Carrier in the context, returning a new context.
// The original context is not modified (context is immutable).
func WithCarrier(ctx context.Context, c Carrier) context.Context {
	return context.WithValue(ctx, carrierCtxKey, c)
}

// CarrierFrom retrieves the Carrier from the context.
// Returns a zero-value Carrier if no carrier is present.
// The returned Carrier is a copy (defensive), safe to use without mutation concerns.
func CarrierFrom(ctx context.Context) Carrier {
	if ctx == nil {
		return Carrier{}
	}
	v, ok := ctx.Value(carrierCtxKey).(Carrier)
	if !ok {
		return Carrier{}
	}
	return v
}

// MergeCarrier merges two Carriers. Non-zero fields in override take precedence.
// Maps (Extra) are merged (override entries win on key conflict).
// This enables chaining: parent ctx has trace_id, child adds user_id.
//
// Example:
//
//	parent := Carrier{TraceID: "abc"}
//	child := MergeCarrier(parent, Carrier{UserID: "123"})
//	// child = {TraceID: "abc", UserID: "123"}
func MergeCarrier(parent, override Carrier) Carrier {
	result := parent

	if override.TraceID != "" {
		result.TraceID = override.TraceID
	}
	if override.RequestID != "" {
		result.RequestID = override.RequestID
	}
	if override.CorrelationID != "" {
		result.CorrelationID = override.CorrelationID
	}
	if override.SessionID != "" {
		result.SessionID = override.SessionID
	}
	if override.UserID != "" {
		result.UserID = override.UserID
	}
	if override.Username != "" {
		result.Username = override.Username
	}
	if override.Role != "" {
		result.Role = override.Role
	}
	if override.IP != "" {
		result.IP = override.IP
	}
	if override.Method != "" {
		result.Method = override.Method
	}
	if override.Path != "" {
		result.Path = override.Path
	}
	if override.StatusCode != 0 {
		result.StatusCode = override.StatusCode
	}

	// Merge Extra maps.
	if len(override.Extra) > 0 {
		if result.Extra == nil {
			result.Extra = make(map[string]string, len(override.Extra))
		}
		for k, v := range override.Extra {
			result.Extra[k] = v
		}
	}

	return result
}

// IsZero reports whether the Carrier has no populated fields.
// Used to skip injecting carrier fields when empty (avoid noise in logs).
func (c Carrier) IsZero() bool {
	return c.TraceID == "" &&
		c.RequestID == "" &&
		c.CorrelationID == "" &&
		c.SessionID == "" &&
		c.UserID == "" &&
		c.Username == "" &&
		c.Role == "" &&
		c.IP == "" &&
		c.Method == "" &&
		c.Path == "" &&
		c.StatusCode == 0 &&
		len(c.Extra) == 0
}
