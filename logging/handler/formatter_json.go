package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// JSONFormatter formats slog.Record as a single-line JSON object.
//
// Output format:
//
//	{"timestamp":"2026-01-01T12:00:00.000Z","level":"INFO","msg":"hello","trace_id":"abc","user_id":"123",...}
//
// Features:
//   - RFC3339Nano timestamp (sortable, ISO8601 compatible)
//   - Uppercase level labels (INFO, WARN, ERROR, etc.)
//   - snake_case field names (from core.Fields constants)
//   - Nested groups flattened with dot notation (parent.child)
//   - No trailing newline in Format(); sink adds it
type JSONFormatter struct {
	// PrettyPrint enables indented JSON (for development only).
	PrettyPrint bool
}

// NewJSONFormatter creates a JSONFormatter.
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// NewJSONFormatterPretty creates a JSONFormatter with pretty printing.
func NewJSONFormatterPretty() *JSONFormatter {
	return &JSONFormatter{PrettyPrint: true}
}

func (f *JSONFormatter) Name() string {
	return "json"
}

// Format converts a slog.Record to JSON bytes (with trailing newline).
func (f *JSONFormatter) Format(record slog.Record) ([]byte, error) {
	// Use a map for deterministic field ordering via custom marshaling.
	// We build the JSON manually to control field order.
	var buf bytes.Buffer
	buf.WriteByte('{')

	// 1. timestamp
	writeJSONString(&buf, "timestamp", record.Time.Format(time.RFC3339Nano))

	// 2. level
	buf.WriteByte(',')
	writeJSONString(&buf, "level", levelLabel(record.Level))

	// 3. message
	buf.WriteByte(',')
	writeJSONString(&buf, "msg", record.Message)

	// 4. attrs (in order they were added).
	// For grouped attrs, we flatten with dot notation.
	prefix := ""
	record.Attrs(func(attr slog.Attr) bool {
		buf.WriteByte(',')
		f.writeAttr(&buf, prefix, attr)
		return true
	})

	buf.WriteByte('}')
	buf.WriteByte('\n')

	return buf.Bytes(), nil
}

// writeAttr writes a single attr to the buffer, handling groups.
func (f *JSONFormatter) writeAttr(buf *bytes.Buffer, prefix string, attr slog.Attr) {
	// Resolve LogValuer.
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return // skipped
	}

	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if attr.Value.Kind() == slog.KindGroup {
		// Flatten group attrs with dot notation.
		children := attr.Value.Group()
		for i, ga := range children {
			if i > 0 {
				buf.WriteByte(',')
			}
			f.writeAttr(buf, key, ga)
		}
		return
	}

	writeJSONKey(buf, key)
	buf.WriteByte(':')
	writeJSONValue(buf, attr.Value)
}

// levelLabel returns the uppercase label for a slog.Level using core.LevelLabel.
func levelLabel(l slog.Level) string {
	return core.LevelLabel(core.Level(l))
}

// writeJSONString writes "key":"value" to the buffer.
func writeJSONString(buf *bytes.Buffer, key, value string) {
	writeJSONKey(buf, key)
	buf.WriteByte(':')
	// Use json.Marshal for proper escaping.
	encoded, _ := json.Marshal(value)
	buf.Write(encoded)
}

// writeJSONKey writes "key" to the buffer.
func writeJSONKey(buf *bytes.Buffer, key string) {
	buf.WriteByte('"')
	// Escape key if needed.
	if needsEscape(key) {
		encoded, _ := json.Marshal(key)
		buf.Write(encoded[1 : len(encoded)-1]) // strip outer quotes
	} else {
		buf.WriteString(key)
	}
	buf.WriteByte('"')
}

// writeJSONValue writes a slog.Value as JSON.
func writeJSONValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		encoded, _ := json.Marshal(v.String())
		buf.Write(encoded)
	case slog.KindInt64:
		buf.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		buf.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		encoded, _ := json.Marshal(v.Float64())
		buf.Write(encoded)
	case slog.KindBool:
		if v.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case slog.KindDuration:
		encoded, _ := json.Marshal(v.Duration().String())
		buf.Write(encoded)
	case slog.KindTime:
		encoded, _ := json.Marshal(v.Time().Format(time.RFC3339Nano))
		buf.Write(encoded)
	case slog.KindAny:
		val := v.Any()
		if val == nil {
			buf.WriteString("null")
			return
		}
		// Try json.Marshal first.
		encoded, err := json.Marshal(val)
		if err != nil {
			// Fallback to fmt.Sprintf.
			encoded, _ = json.Marshal(fmt.Sprintf("%v", val))
		}
		buf.Write(encoded)
	case slog.KindLogValuer:
		writeJSONValue(buf, v.Resolve())
	case slog.KindGroup:
		// Should not reach here (handled in writeAttr).
		buf.WriteString("null")
	default:
		buf.WriteString("null")
	}
}

// needsEscape checks if a string contains characters that need JSON escaping.
func needsEscape(s string) bool {
	for _, r := range s {
		if r == '"' || r == '\\' || r == '\n' || r == '\r' || r == '\t' || r < 0x20 {
			return true
		}
	}
	return false
}

// Ensure strings import is used.
var _ = strings.Contains
