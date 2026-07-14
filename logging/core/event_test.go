package core

import (
	"testing"
)

func TestEventRegistry_RegisterAndGet(t *testing.T) {
	r := NewEventRegistry()
	meta := EventMeta{
		ID:       "custom.event",
		Category: EventCategoryBusiness,
		Name:     "Custom Event",
		Severity: LevelInfo,
	}
	if err := r.Register(meta); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, ok := r.Get("custom.event")
	if !ok {
		t.Fatal("event not found after register")
	}
	if got.Name != "Custom Event" {
		t.Errorf("Name = %q, want 'Custom Event'", got.Name)
	}
}

func TestEventRegistry_DuplicateRegister(t *testing.T) {
	r := NewEventRegistry()
	meta := EventMeta{ID: "dup.event", Category: EventCategoryBusiness, Name: "Dup"}
	if err := r.Register(meta); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := r.Register(meta); err == nil {
		t.Error("second Register should fail with ErrEventAlreadyRegistered")
	}
}

func TestEventRegistry_EmptyID(t *testing.T) {
	r := NewEventRegistry()
	meta := EventMeta{ID: "", Category: EventCategoryBusiness}
	if err := r.Register(meta); err == nil {
		t.Error("Register with empty ID should fail")
	}
}

func TestEventRegistry_ListByCategory(t *testing.T) {
	r := NewEventRegistry()
	r.MustRegister(EventMeta{ID: "b.1", Category: EventCategoryBusiness, Name: "B1"})
	r.MustRegister(EventMeta{ID: "s.1", Category: EventCategorySecurity, Name: "S1"})
	r.MustRegister(EventMeta{ID: "b.2", Category: EventCategoryBusiness, Name: "B2"})

	business := r.ListByCategory(EventCategoryBusiness)
	if len(business) != 2 {
		t.Errorf("business events = %d, want 2", len(business))
	}
	security := r.ListByCategory(EventCategorySecurity)
	if len(security) != 1 {
		t.Errorf("security events = %d, want 1", len(security))
	}
}

func TestDefaultBusinessEvents(t *testing.T) {
	events := DefaultBusinessEvents()
	if len(events) != 15 {
		t.Errorf("DefaultBusinessEvents count = %d, want 15", len(events))
	}
	// Verify all have valid IDs.
	for _, e := range events {
		if e.ID == "" {
			t.Error("business event has empty ID")
		}
		if e.Category != EventCategoryBusiness {
			t.Errorf("event %s category = %q, want 'business'", e.ID, e.Category)
		}
	}
}

func TestDefaultSecurityEvents(t *testing.T) {
	events := DefaultSecurityEvents()
	if len(events) != 16 {
		t.Errorf("DefaultSecurityEvents count = %d, want 16", len(events))
	}
	for _, e := range events {
		if e.ID == "" {
			t.Error("security event has empty ID")
		}
		if e.Category != EventCategorySecurity {
			t.Errorf("event %s category = %q, want 'security'", e.ID, e.Category)
		}
	}
}

func TestNewDefaultEventRegistry(t *testing.T) {
	r := NewDefaultEventRegistry()
	total := r.Count()
	// 15 business + 16 security = 31
	if total != 31 {
		t.Errorf("default registry count = %d, want 31", total)
	}
}
