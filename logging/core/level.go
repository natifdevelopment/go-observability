package core

import (
	"log/slog"
	"strings"
)

// Level is a type alias for slog.Level to ensure interoperability with
// the standard library slog package while extending it with custom levels.
//
// Extended levels (beyond slog's Debug/Info/Warn/Error):
//   - LevelTrace (-8): below Debug, for very verbose tracing
//   - LevelFatal (12): above Error, indicates fatal error (process will exit)
//   - LevelPanic (16): above Fatal, indicates panic (process will panic)
//
// Monitoring compatibility:
//   - LevelLabel() returns uppercase strings ("TRACE", "INFO", etc.)
//     which are Grafana/Loki label-friendly and ELK field-friendly.
//   - For third-party libraries expecting standard slog levels,
//     use LevelToSlogStandard() to map TRACE→DEBUG, FATAL/PANIC→ERROR.
type Level = slog.Level

// Extended log levels.
const (
	LevelTrace Level = slog.Level(-8) // below Debug
	LevelDebug Level = slog.LevelDebug
	LevelInfo  Level = slog.LevelInfo
	LevelWarn  Level = slog.LevelWarn
	LevelError Level = slog.LevelError
	LevelFatal Level = slog.Level(12) // above Error
	LevelPanic Level = slog.Level(16) // above Fatal
)

// levelLabels maps Level values to their string labels.
// Keys are the integer values of slog.Level.
var levelLabels = map[Level]string{
	LevelTrace: "TRACE",
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
	LevelFatal: "FATAL",
	LevelPanic: "PANIC",
}

// levelParse maps string (case-insensitive) to Level.
var levelParse = map[string]Level{
	"trace":   LevelTrace,
	"debug":   LevelDebug,
	"info":    LevelInfo,
	"warn":    LevelWarn,
	"warning": LevelWarn,
	"error":   LevelError,
	"err":     LevelError,
	"fatal":   LevelFatal,
	"panic":   LevelPanic,
}

// ParseLevel converts a string to a Level.
// Case-insensitive. Accepts "WARN" and "WARNING", "ERROR" and "ERR".
// Returns ErrInvalidLevel if the string is not recognized (fail-fast).
func ParseLevel(s string) (Level, error) {
	l, ok := levelParse[strings.ToLower(strings.TrimSpace(s))]
	if !ok {
		return 0, ErrInvalidLevel
	}
	return l, nil
}

// LevelLabel converts a Level to its uppercase string label.
// For levels between standard levels (e.g., Level(1)), returns "INFO+1".
// This ensures every level has a deterministic label for log output.
func LevelLabel(l Level) string {
	if label, ok := levelLabels[l]; ok {
		return label
	}
	// Fallback for non-standard levels: show base level + offset.
	// This handles edge cases like Level(2) → "INFO+2".
	switch {
	case l < LevelDebug:
		return "TRACE+" + itoa(int(LevelDebug-l))
	case l < LevelInfo:
		return "DEBUG+" + itoa(int(l-LevelDebug))
	case l < LevelWarn:
		return "INFO+" + itoa(int(l-LevelInfo))
	case l < LevelError:
		return "WARN+" + itoa(int(l-LevelWarn))
	case l < LevelFatal:
		return "ERROR+" + itoa(int(l-LevelError))
	case l < LevelPanic:
		return "FATAL+" + itoa(int(l-LevelFatal))
	default:
		return "PANIC+" + itoa(int(l-LevelPanic))
	}
}

// LevelEnabled reports whether level l meets or exceeds the minimum threshold min.
// Example: LevelEnabled(LevelInfo, LevelDebug) == true (Info >= Debug in severity).
func LevelEnabled(min, l Level) bool {
	return l >= min
}

// LevelToSlogStandard maps extended levels to standard slog levels
// for third-party library interoperability.
// TRACE → DEBUG, FATAL → ERROR, PANIC → ERROR.
// This is used by Logger.Slog() when handing off to libraries that
// only understand standard slog levels.
func LevelToSlogStandard(l Level) Level {
	switch {
	case l <= LevelTrace:
		return LevelDebug
	case l >= LevelFatal:
		return LevelError
	default:
		return l
	}
}

// itoa converts int to string without importing strconv (keep deps minimal).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
