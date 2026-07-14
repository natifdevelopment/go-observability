package sink

import (
	"fmt"
	"sync"
)

// SinkFactory creates a Sink from a config map (for plugin/config-driven sink creation).
type SinkFactory func(cfg map[string]any) (Sink, error)

var (
	sinkRegistryMu sync.RWMutex
	sinkRegistry   = make(map[string]SinkFactory)
)

// RegisterSink registers a SinkFactory by name.
// Allows custom sinks to be created from config without code changes.
// Not safe to call concurrently with NewSinkByName.
func RegisterSink(name string, factory SinkFactory) {
	sinkRegistryMu.Lock()
	defer sinkRegistryMu.Unlock()
	sinkRegistry[name] = factory
}

// NewSinkByName creates a Sink by its registered name.
// Returns error if the name is not registered.
func NewSinkByName(name string, cfg map[string]any) (Sink, error) {
	sinkRegistryMu.RLock()
	factory, ok := sinkRegistry[name]
	sinkRegistryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sink: no sink registered with name %q", name)
	}
	return factory(cfg)
}

// RegisteredSinks returns the names of all registered sinks.
func RegisteredSinks() []string {
	sinkRegistryMu.RLock()
	defer sinkRegistryMu.RUnlock()
	names := make([]string, 0, len(sinkRegistry))
	for name := range sinkRegistry {
		names = append(names, name)
	}
	return names
}

// init registers built-in sinks.
func init() {
	RegisterSink("console", func(cfg map[string]any) (Sink, error) {
		if useStderr, _ := cfg["stderr"].(bool); useStderr {
			return NewConsoleSinkStderr(), nil
		}
		return NewConsoleSink(), nil
	})
	RegisterSink("file", func(cfg map[string]any) (Sink, error) {
		path, _ := cfg["path"].(string)
		if path == "" {
			return nil, fmt.Errorf("sink: file sink requires 'path' config")
		}
		return NewFileSink(path)
	})
	RegisterSink("rotate", func(cfg map[string]any) (Sink, error) {
		path, _ := cfg["path"].(string)
		if path == "" {
			return nil, fmt.Errorf("sink: rotate sink requires 'path' config")
		}
		maxSize, _ := cfg["max_size_mb"].(int)
		maxBackups, _ := cfg["max_backups"].(int)
		maxAge, _ := cfg["max_age_days"].(int)
		compress, _ := cfg["compress"].(bool)
		return NewRotateSink(RotateConfig{
			Path:       path,
			MaxSizeMB:  maxSize,
			MaxBackups: maxBackups,
			MaxAgeDays: maxAge,
			Compress:   compress,
		})
	})
}
