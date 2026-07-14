package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// FormatterRegistry maps formatter names to factory functions.
type FormatterRegistry struct {
	mu        sync.RWMutex
	factories map[string]func() Formatter
}

var (
	defaultRegistry = &FormatterRegistry{
		factories: make(map[string]func() Formatter),
	}
	registryOnce sync.Once
)

// init registers built-in formatters.
func init() {
	defaultRegistry.Register("json", func() Formatter { return NewJSONFormatter() })
	defaultRegistry.Register("json_pretty", func() Formatter { return NewJSONFormatterPretty() })
	defaultRegistry.Register("console", func() Formatter { return NewConsoleFormatter() })
	defaultRegistry.Register("console_color", func() Formatter { return NewConsoleFormatterColor() })
}

// Register adds a formatter factory to the registry.
func (r *FormatterRegistry) Register(name string, factory func() Formatter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// NewFormatterByName creates a formatter by its registered name.
func NewFormatterByName(name string) (Formatter, error) {
	defaultRegistry.mu.RLock()
	factory, ok := defaultRegistry.factories[name]
	defaultRegistry.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("handler: no formatter registered with name %q", name)
	}
	return factory(), nil
}

// RegisteredFormatters returns the names of all registered formatters.
func RegisteredFormatters() []string {
	defaultRegistry.mu.RLock()
	defer defaultRegistry.mu.RUnlock()
	names := make([]string, 0, len(defaultRegistry.factories))
	for name := range defaultRegistry.factories {
		names = append(names, name)
	}
	return names
}

// Ensure imports are used.
var (
	_ = bytes.NewBuffer
	_ = json.Marshal
	_ = slog.LevelInfo
)
