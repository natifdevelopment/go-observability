package core

import (
	"log/slog"
	"testing"
)

// Test all Attr helper functions for coverage.

func TestAllAttrHelpers(t *testing.T) {
	tests := []struct {
		name string
		attr slog.Attr
		key  string
	}{
		{"Timestamp", TimestampAttr("2026-01-01"), "timestamp"},
		{"Service", ServiceAttr("api"), "service"},
		{"ServiceVersion", ServiceVersionAttr("1.0.0"), "service_version"},
		{"Environment", EnvironmentAttr("production"), "environment"},
		{"Hostname", HostnameAttr("host-1"), "hostname"},
		{"Application", ApplicationAttr("app"), "application"},
		{"Module", ModuleAttr("mod"), "module"},
		{"Package", PackageAttr("pkg"), "package"},
		{"Function", FunctionAttr("fn"), "function"},
		{"File", FileAttr("file.go"), "file"},
		{"Line", LineAttr(42), "line"},
		{"TraceID", TraceIDAttr("t1"), "trace_id"},
		{"RequestID", RequestIDAttr("r1"), "request_id"},
		{"CorrelationID", CorrelationIDAttr("c1"), "correlation_id"},
		{"SessionID", SessionIDAttr("s1"), "session_id"},
		{"UserID", UserIDAttr("u1"), "user_id"},
		{"Username", UsernameAttr("john"), "username"},
		{"Role", RoleAttr("admin"), "role"},
		{"IP", IPAttr("10.0.0.1"), "ip"},
		{"Method", MethodAttr("GET"), "method"},
		{"Path", PathAttr("/api"), "path"},
		{"StatusCode", StatusCodeAttr(200), "status_code"},
		{"DurationMs", DurationMsAttr(150), "duration_ms"},
		{"ErrorCode", ErrorCodeAttr("ERR_001"), "error_code"},
		{"Stacktrace", StacktraceAttr("stack..."), "stacktrace"},
		{"Metadata", MetadataAttr(map[string]any{"k": "v"}), "metadata"},
	}
	for _, tt := range tests {
		if tt.attr.Key != tt.key {
			t.Errorf("%s: key = %q, want %q", tt.name, tt.attr.Key, tt.key)
		}
	}
}

func TestFieldConstants(t *testing.T) {
	// Verify a sample of field constants have correct string values.
	if FieldTimestamp != "timestamp" {
		t.Error("FieldTimestamp wrong")
	}
	if FieldTraceID != "trace_id" {
		t.Error("FieldTraceID wrong")
	}
	if FieldDurationMs != "duration_ms" {
		t.Error("FieldDurationMs wrong")
	}
	if FieldStacktrace != "stacktrace" {
		t.Error("FieldStacktrace wrong")
	}
	if FieldMetadata != "metadata" {
		t.Error("FieldMetadata wrong")
	}
}
