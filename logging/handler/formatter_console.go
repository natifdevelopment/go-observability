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

// ConsoleFormatter formats slog.Record as a human-readable console line.
//
// Output format:
//
//	2026-01-01T12:00:00.000Z INFO  msg="hello" trace_id="abc" user_id="123"
//
// Features:
//   - Color-coded level (optional, for terminal output)
//   - key="value" format (quoted, escaped)
//   - No JSON escaping (human-readable)
//   - Trailing newline
type ConsoleFormatter struct {
	UseColor bool
}

// NewConsoleFormatter creates a ConsoleFormatter.
func NewConsoleFormatter() *ConsoleFormatter {
	return &ConsoleFormatter{}
}

// NewConsoleFormatterColor creates a ConsoleFormatter with color output.
func NewConsoleFormatterColor() *ConsoleFormatter {
	return &ConsoleFormatter{UseColor: true}
}

func (f *ConsoleFormatter) Name() string {
	return "console"
}

// Format converts a slog.Record to console-style bytes.
func (f *ConsoleFormatter) Format(record slog.Record) ([]byte, error) {
	var buf bytes.Buffer

	// Timestamp.
	buf.WriteString(record.Time.Format(time.RFC3339Nano))
	buf.WriteByte(' ')

	// Level (color-coded if enabled).
	levelStr := levelLabel(record.Level)
	if f.UseColor {
		levelStr = colorize(levelStr, levelColor(record.Level))
	}
	// Pad level to 5 chars for alignment.
	if !f.UseColor {
		for len(levelStr) < 5 {
			levelStr += " "
		}
	}
	buf.WriteString(levelStr)
	buf.WriteByte(' ')

	// Message.
	buf.WriteString("msg=")
	writeConsoleString(&buf, record.Message)

	// Attrs.
	record.Attrs(func(attr slog.Attr) bool {
		attr.Value = attr.Value.Resolve()
		if attr.Equal(slog.Attr{}) {
			return true
		}
		buf.WriteByte(' ')
		f.writeAttr(&buf, "", attr)
		return true
	})

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func (f *ConsoleFormatter) writeAttr(buf *bytes.Buffer, prefix string, attr slog.Attr) {
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}

	if attr.Value.Kind() == slog.KindGroup {
		for _, ga := range attr.Value.Group() {
			f.writeAttr(buf, key, ga)
		}
		return
	}

	buf.WriteString(key)
	buf.WriteByte('=')
	writeConsoleValue(buf, attr.Value)
}

// writeConsoleValue writes a slog.Value in console format.
func writeConsoleValue(buf *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		writeConsoleString(buf, v.String())
	case slog.KindInt64:
		buf.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		buf.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindFloat64:
		buf.WriteString(strconv.FormatFloat(v.Float64(), 'f', -1, 64))
	case slog.KindBool:
		if v.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case slog.KindDuration:
		buf.WriteString(v.Duration().String())
	case slog.KindTime:
		buf.WriteString(v.Time().Format(time.RFC3339Nano))
	case slog.KindAny:
		val := v.Any()
		if val == nil {
			buf.WriteString("<nil>")
			return
		}
		// Try JSON for complex types.
		encoded, err := json.Marshal(val)
		if err != nil {
			writeConsoleString(buf, fmt.Sprintf("%v", val))
		} else {
			buf.Write(encoded)
		}
	case slog.KindLogValuer:
		writeConsoleValue(buf, v.Resolve())
	default:
		buf.WriteString("<unknown>")
	}
}

// writeConsoleString writes a string in console format (quoted if needed).
func writeConsoleString(buf *bytes.Buffer, s string) {
	if needsQuoting(s) {
		encoded, _ := json.Marshal(s)
		buf.Write(encoded)
	} else {
		buf.WriteString(s)
	}
}

// needsQuoting checks if a string needs to be quoted in console output.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, r := range s {
		if r == ' ' || r == '"' || r == '=' || r == '\n' || r == '\r' || r == '\t' || r < 0x20 {
			return true
		}
	}
	return false
}

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorCyan   = "\033[36m"
	colorPurple = "\033[35m"
)

// levelColor returns the ANSI color for a level.
func levelColor(l slog.Level) string {
	switch {
	case l >= core.LevelPanic:
		return colorPurple // PANIC
	case l >= core.LevelFatal:
		return colorRed // FATAL
	case l >= slog.LevelError:
		return colorRed // ERROR
	case l >= slog.LevelWarn:
		return colorYellow // WARN
	case l >= slog.LevelInfo:
		return colorGreen // INFO
	case l >= slog.LevelDebug:
		return colorBlue // DEBUG
	default:
		return colorGray // TRACE
	}
}

// colorize wraps a string with ANSI color codes.
func colorize(s, color string) string {
	return color + s + colorReset
}

// Ensure strings import is used.
var _ = strings.Contains
