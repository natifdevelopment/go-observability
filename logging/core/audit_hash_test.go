package core

import (
	"testing"
)

func TestComputeAuditHash_Deterministic(t *testing.T) {
	payload := AuditPayload{
		Timestamp: "2026-07-14T12:00:00Z",
		User:      "user-1",
		Action:    "user.update",
		Resource:  "user:456",
		IP:        "10.0.0.1",
	}

	h1, err := ComputeAuditHash(ZeroHash, payload)
	if err != nil {
		t.Fatalf("ComputeAuditHash failed: %v", err)
	}
	h2, err := ComputeAuditHash(ZeroHash, payload)
	if err != nil {
		t.Fatalf("second ComputeAuditHash failed: %v", err)
	}
	if h1 != h2 {
		t.Error("hash should be deterministic for same input")
	}
	if h1 == ZeroHash {
		t.Error("hash should not equal ZeroHash")
	}
}

func TestComputeAuditHash_DifferentInput(t *testing.T) {
	p1 := AuditPayload{User: "user-1", Action: "create", IP: "10.0.0.1", Timestamp: "t1"}
	p2 := AuditPayload{User: "user-2", Action: "create", IP: "10.0.0.1", Timestamp: "t1"}

	h1, _ := ComputeAuditHash(ZeroHash, p1)
	h2, _ := ComputeAuditHash(ZeroHash, p2)
	if h1 == h2 {
		t.Error("different payloads should produce different hashes")
	}
}

func TestVerifyAuditChain_Valid(t *testing.T) {
	payloads := []AuditPayload{
		{Timestamp: "t1", User: "u1", Action: "a1", IP: "ip1"},
		{Timestamp: "t2", User: "u2", Action: "a2", IP: "ip2"},
		{Timestamp: "t3", User: "u3", Action: "a3", IP: "ip3"},
	}

	records := make([]AuditChainEntry, len(payloads))
	prev := ZeroHash
	for i, p := range payloads {
		h, err := ComputeAuditHash(prev, p)
		if err != nil {
			t.Fatalf("ComputeAuditHash failed: %v", err)
		}
		records[i] = AuditChainEntry{
			Seq:      uint64(i + 1),
			PrevHash: prev,
			Payload:  p,
			CurrHash: h,
		}
		prev = h
	}

	idx, err := VerifyAuditChain(records)
	if err != nil {
		t.Errorf("valid chain should verify, got error: %v", err)
	}
	if idx != -1 {
		t.Errorf("valid chain tamperedIndex = %d, want -1", idx)
	}
}

func TestVerifyAuditChain_Tampered(t *testing.T) {
	payloads := []AuditPayload{
		{Timestamp: "t1", User: "u1", Action: "a1", IP: "ip1"},
		{Timestamp: "t2", User: "u2", Action: "a2", IP: "ip2"},
	}

	records := make([]AuditChainEntry, len(payloads))
	prev := ZeroHash
	for i, p := range payloads {
		h, _ := ComputeAuditHash(prev, p)
		records[i] = AuditChainEntry{
			Seq:      uint64(i + 1),
			PrevHash: prev,
			Payload:  p,
			CurrHash: h,
		}
		prev = h
	}

	// Tamper with the second record's payload.
	records[1].Payload.User = "hacker"

	idx, err := VerifyAuditChain(records)
	if err == nil {
		t.Error("tampered chain should fail verification")
	}
	if idx != 1 {
		t.Errorf("tamperedIndex = %d, want 1", idx)
	}
}

func TestVerifyAuditChain_Empty(t *testing.T) {
	_, err := VerifyAuditChain(nil)
	if err == nil {
		t.Error("empty chain should return error")
	}
}

func TestVerifyAuditChain_BrokenLink(t *testing.T) {
	p1 := AuditPayload{Timestamp: "t1", User: "u1", Action: "a1", IP: "ip1"}
	h1, _ := ComputeAuditHash(ZeroHash, p1)

	// Second record claims wrong prev_hash.
	p2 := AuditPayload{Timestamp: "t2", User: "u2", Action: "a2", IP: "ip2"}
	h2, _ := ComputeAuditHash(ZeroHash, p2) // should use h1 as prev, but we use ZeroHash

	records := []AuditChainEntry{
		{Seq: 1, PrevHash: ZeroHash, Payload: p1, CurrHash: h1},
		{Seq: 2, PrevHash: ZeroHash, Payload: p2, CurrHash: h2}, // wrong prev_hash
	}

	idx, err := VerifyAuditChain(records)
	if err == nil {
		t.Error("broken link should fail verification")
	}
	if idx != 1 {
		t.Errorf("tamperedIndex = %d, want 1", idx)
	}
}

func TestCanonicalJSON_SortedKeys(t *testing.T) {
	// Two payloads with same data but different field declaration order
	// should produce the same canonical JSON (and thus same hash).
	p1 := AuditPayload{User: "u1", Action: "a1", IP: "ip1", Timestamp: "t1"}
	p2 := AuditPayload{Timestamp: "t1", IP: "ip1", Action: "a1", User: "u1"}

	h1, _ := ComputeAuditHash(ZeroHash, p1)
	h2, _ := ComputeAuditHash(ZeroHash, p2)

	if h1 != h2 {
		t.Error("canonical JSON should produce same hash regardless of field order")
	}
}
