package core

import (
	"fmt"
	"sync"
)

// EventCategory categorizes log events for filtering and querying.
type EventCategory string

const (
	EventCategoryBusiness EventCategory = "business"
	EventCategorySecurity EventCategory = "security"
	EventCategoryAudit    EventCategory = "audit"
)

// EventMeta describes a registered log event.
// Events provide structured, queryable log entries with consistent fields.
//
// Event IDs use dot-notation for hierarchy (e.g., "user.login.success")
// making them filterable in Grafana/Loki/ELK via prefix queries.
type EventMeta struct {
	ID          string         `json:"id"`
	Category    EventCategory  `json:"category"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Severity    Level          `json:"severity"`
	Fields      []Field        `json:"fields"`
}

// EventRegistry stores registered log events.
// Thread-safe (sync.RWMutex). Extensible: developers can register
// custom events without modifying the framework (Open/Closed Principle).
type EventRegistry struct {
	mu     sync.RWMutex
	events map[string]EventMeta
}

// NewEventRegistry creates an empty EventRegistry.
func NewEventRegistry() *EventRegistry {
	return &EventRegistry{
		events: make(map[string]EventMeta, 64),
	}
}

// NewDefaultEventRegistry creates an EventRegistry pre-populated with
// all default business and security events.
func NewDefaultEventRegistry() *EventRegistry {
	r := NewEventRegistry()
	for _, e := range DefaultBusinessEvents() {
		r.MustRegister(e)
	}
	for _, e := range DefaultSecurityEvents() {
		r.MustRegister(e)
	}
	return r
}

// Register adds a new event to the registry.
// Returns ErrEventAlreadyRegistered if the ID already exists.
func (r *EventRegistry) Register(meta EventMeta) error {
	if meta.ID == "" {
		return fmt.Errorf("%w: event ID is empty", ErrInvalidConfig)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.events[meta.ID]; exists {
		return fmt.Errorf("%w: %s", ErrEventAlreadyRegistered, meta.ID)
	}
	r.events[meta.ID] = meta
	return nil
}

// MustRegister is like Register but panics on error.
// Intended for use in package init() functions.
func (r *EventRegistry) MustRegister(meta EventMeta) {
	if err := r.Register(meta); err != nil {
		panic(err)
	}
}

// Get retrieves an EventMeta by ID.
// Returns the meta and true if found, or zero-value and false if not.
func (r *EventRegistry) Get(id string) (EventMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.events[id]
	return meta, ok
}

// List returns a snapshot of all registered events (sorted by ID).
func (r *EventRegistry) List() []EventMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]EventMeta, 0, len(r.events))
	for _, meta := range r.events {
		result = append(result, meta)
	}
	return result
}

// ListByCategory returns all events in a given category.
func (r *EventRegistry) ListByCategory(cat EventCategory) []EventMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]EventMeta, 0)
	for _, meta := range r.events {
		if meta.Category == cat {
			result = append(result, meta)
		}
	}
	return result
}

// Count returns the total number of registered events.
func (r *EventRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
