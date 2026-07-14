package core

import "log/slog"

// Field is the standardized name of a log field.
// All field names are snake_case for consistency across services,
// making logs queryable in Grafana Loki, Elasticsearch, and Datadog.
//
// Usage:
//
//	logger.Info(ctx, "user logged in",
//	    slog.String(string(core.FieldUserID), "12345"),
//	    slog.String(string(core.FieldUsername), "john"))
//
// Or use the Attr helpers in this file for type safety:
//
//	logger.Info(ctx, "user logged in",
//	    core.UserID("12345"),
//	    core.Username("john"))
type Field string

// Standard log fields. These are the MINIMUM required fields
// that every log record should contain (where applicable).
const (
	FieldTimestamp      Field = "timestamp"
	FieldLevel          Field = "level"
	FieldMessage        Field = "message"
	FieldService        Field = "service"
	FieldServiceVersion Field = "service_version"
	FieldEnvironment    Field = "environment"
	FieldHostname       Field = "hostname"
	FieldApplication    Field = "application"
	FieldModule         Field = "module"
	FieldPackage        Field = "package"
	FieldFunction       Field = "function"
	FieldFile           Field = "file"
	FieldLine           Field = "line"
	FieldTraceID        Field = "trace_id"
	FieldRequestID      Field = "request_id"
	FieldCorrelationID  Field = "correlation_id"
	FieldSessionID      Field = "session_id"
	FieldUserID         Field = "user_id"
	FieldUsername       Field = "username"
	FieldRole           Field = "role"
	FieldIP             Field = "ip"
	FieldMethod         Field = "method"
	FieldPath           Field = "path"
	FieldStatusCode     Field = "status_code"
	FieldDurationMs     Field = "duration_ms"
	FieldErrorCode      Field = "error_code"
	FieldError          Field = "error"
	FieldStacktrace     Field = "stacktrace"
	FieldMetadata       Field = "metadata"
)

// Additional fields for specific log types.
const (
	// Request logging
	FieldQuery        Field = "query"
	FieldUserAgent    Field = "user_agent"
	FieldRequestSize  Field = "request_size"
	FieldResponseSize Field = "response_size"
	FieldRequestBody  Field = "request_body"
	FieldResponseBody Field = "response_body"
	FieldRemoteIP     Field = "remote_ip"
	FieldHeaders      Field = "headers"

	// Database logging
	FieldDBType        Field = "db_type"
	FieldDBName        Field = "db_name"
	FieldDBQuery       Field = "db_query"
	FieldRows          Field = "rows"
	FieldAffectedRows  Field = "affected_rows"
	FieldTransactionID Field = "transaction_id"
	FieldTxAction      Field = "tx_action"
	FieldIsSlowQuery   Field = "is_slow_query"
	FieldDBError       Field = "db_error"

	// External API logging
	FieldExternalService     Field = "external_service"
	FieldBaseURL             Field = "base_url"
	FieldEndpoint            Field = "endpoint"
	FieldLatency             Field = "latency_ms"
	FieldRetryCount          Field = "retry_count"
	FieldTimeout             Field = "timeout"
	FieldCircuitBreakerState Field = "circuit_breaker_state"

	// Audit logging
	FieldAuditSeq      Field = "seq"
	FieldAuditAction   Field = "audit_action"
	FieldAuditResource Field = "audit_resource"
	FieldAuditBefore   Field = "audit_before"
	FieldAuditAfter    Field = "audit_after"
	FieldAuditDevice   Field = "audit_device"
	FieldAuditBrowser  Field = "audit_browser"
	FieldAuditOS       Field = "audit_os"
	FieldAuditReason   Field = "audit_reason"
	FieldAuditPrevHash Field = "prev_hash"
	FieldAuditCurrHash Field = "curr_hash"
	FieldAuditSeverity Field = "audit_severity"

	// Security logging
	FieldSecurityEvent  Field = "security_event"
	FieldAttemptCount   Field = "attempt_count"
	FieldDetectionType  Field = "detection_type"
	FieldDetectionInput Field = "detection_input"

	// Kafka/RabbitMQ
	FieldTopic     Field = "topic"
	FieldPartition Field = "partition"
	FieldOffset    Field = "offset"
	FieldQueueName Field = "queue_name"
	FieldMsgKey    Field = "msg_key"

	// Event
	FieldEventID       Field = "event_id"
	FieldEventCategory Field = "event_category"
	FieldEventName     Field = "event_name"
)

// Attr helper functions for type-safe field construction.
// These wrap slog.Attr builders with the standard field names.
// Named with "Attr" suffix to avoid collision with context accessors
// (e.g., core.TraceID() reads from context; core.TraceIDAttr() builds an attr).

// TimestampAttr returns a slog.Attr for the timestamp field.
func TimestampAttr(s string) slog.Attr { return slog.String(string(FieldTimestamp), s) }

// ServiceAttr returns a slog.Attr for the service field.
func ServiceAttr(s string) slog.Attr { return slog.String(string(FieldService), s) }

// ServiceVersionAttr returns a slog.Attr for the service_version field.
func ServiceVersionAttr(s string) slog.Attr { return slog.String(string(FieldServiceVersion), s) }

// EnvironmentAttr returns a slog.Attr for the environment field.
func EnvironmentAttr(s string) slog.Attr { return slog.String(string(FieldEnvironment), s) }

// HostnameAttr returns a slog.Attr for the hostname field.
func HostnameAttr(s string) slog.Attr { return slog.String(string(FieldHostname), s) }

// ApplicationAttr returns a slog.Attr for the application field.
func ApplicationAttr(s string) slog.Attr { return slog.String(string(FieldApplication), s) }

// ModuleAttr returns a slog.Attr for the module field.
func ModuleAttr(s string) slog.Attr { return slog.String(string(FieldModule), s) }

// PackageAttr returns a slog.Attr for the package field.
func PackageAttr(s string) slog.Attr { return slog.String(string(FieldPackage), s) }

// FunctionAttr returns a slog.Attr for the function field.
func FunctionAttr(s string) slog.Attr { return slog.String(string(FieldFunction), s) }

// FileAttr returns a slog.Attr for the file field.
func FileAttr(s string) slog.Attr { return slog.String(string(FieldFile), s) }

// LineAttr returns a slog.Attr for the line field.
func LineAttr(n int) slog.Attr { return slog.Int(string(FieldLine), n) }

// TraceIDAttr returns a slog.Attr for the trace_id field.
func TraceIDAttr(s string) slog.Attr { return slog.String(string(FieldTraceID), s) }

// RequestIDAttr returns a slog.Attr for the request_id field.
func RequestIDAttr(s string) slog.Attr { return slog.String(string(FieldRequestID), s) }

// CorrelationIDAttr returns a slog.Attr for the correlation_id field.
func CorrelationIDAttr(s string) slog.Attr { return slog.String(string(FieldCorrelationID), s) }

// SessionIDAttr returns a slog.Attr for the session_id field.
func SessionIDAttr(s string) slog.Attr { return slog.String(string(FieldSessionID), s) }

// UserIDAttr returns a slog.Attr for the user_id field.
func UserIDAttr(s string) slog.Attr { return slog.String(string(FieldUserID), s) }

// UsernameAttr returns a slog.Attr for the username field.
func UsernameAttr(s string) slog.Attr { return slog.String(string(FieldUsername), s) }

// RoleAttr returns a slog.Attr for the role field.
func RoleAttr(s string) slog.Attr { return slog.String(string(FieldRole), s) }

// IPAttr returns a slog.Attr for the ip field.
func IPAttr(s string) slog.Attr { return slog.String(string(FieldIP), s) }

// MethodAttr returns a slog.Attr for the method field.
func MethodAttr(s string) slog.Attr { return slog.String(string(FieldMethod), s) }

// PathAttr returns a slog.Attr for the path field.
func PathAttr(s string) slog.Attr { return slog.String(string(FieldPath), s) }

// StatusCodeAttr returns a slog.Attr for the status_code field.
func StatusCodeAttr(n int) slog.Attr { return slog.Int(string(FieldStatusCode), n) }

// DurationMsAttr returns a slog.Attr for the duration_ms field.
func DurationMsAttr(n int64) slog.Attr { return slog.Int64(string(FieldDurationMs), n) }

// ErrorCodeAttr returns a slog.Attr for the error_code field.
func ErrorCodeAttr(s string) slog.Attr { return slog.String(string(FieldErrorCode), s) }

// StacktraceAttr returns a slog.Attr for the stacktrace field.
func StacktraceAttr(s string) slog.Attr { return slog.String(string(FieldStacktrace), s) }

// MetadataAttr returns a slog.Attr for the metadata field (nested object).
func MetadataAttr(v any) slog.Attr { return slog.Any(string(FieldMetadata), v) }
