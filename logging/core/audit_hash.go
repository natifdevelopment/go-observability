package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// AuditHash is a hex-encoded SHA256 hash used in the audit log hash chain.
type AuditHash string

// ZeroHash is the initial hash for the first audit record in a chain.
// It represents "no previous record" and is the starting point of the chain.
var ZeroHash AuditHash = "0000000000000000000000000000000000000000000000000000000000000000"

// AuditPayload is the payload of an audit log record.
// This is the data that gets hashed — it must be serialized deterministically
// (canonical JSON with sorted keys) to ensure hash reproducibility.
type AuditPayload struct {
	Timestamp string         `json:"timestamp"`
	User      string         `json:"user"`
	Role      string         `json:"role"`
	Action    string         `json:"action"`
	Resource  string         `json:"resource"`
	Before    any            `json:"before,omitempty"`
	After     any            `json:"after,omitempty"`
	Device    string         `json:"device,omitempty"`
	Browser   string         `json:"browser,omitempty"`
	OS        string         `json:"os,omitempty"`
	IP        string         `json:"ip"`
	Reason    string         `json:"reason,omitempty"`
	TraceID   string         `json:"trace_id,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// AuditChainEntry is a single line in the audit log file.
// Each entry links to the previous entry via PrevHash, forming a chain.
// CurrHash = SHA256(PrevHash || canonical_JSON(Payload)).
type AuditChainEntry struct {
	Seq      uint64       `json:"seq"`
	PrevHash AuditHash    `json:"prev_hash"`
	Payload  AuditPayload `json:"payload"`
	CurrHash AuditHash    `json:"curr_hash"`
}

// ComputeAuditHash computes the hash for a single audit record.
// curr = SHA256(prev || canonical_JSON(payload))
//
// canonical_JSON uses sorted keys to ensure deterministic serialization
// across different Go versions and platforms. This is critical for
// hash chain integrity verification.
func ComputeAuditHash(prev AuditHash, payload AuditPayload) (AuditHash, error) {
	canonical, err := canonicalJSON(payload)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalize audit payload: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(string(prev)))
	h.Write(canonical)
	return AuditHash(hex.EncodeToString(h.Sum(nil))), nil
}

// VerifyAuditChain verifies the integrity of an audit hash chain.
// It replays the chain from the first entry and checks that every
// CurrHash matches the recomputed hash.
//
// Returns:
//   - tamperedIndex: index of the first tampered entry (-1 if all valid)
//   - err: error if chain is empty or verification fails
//
// Tamper detection:
//   - If any entry's CurrHash doesn't match the recomputed hash,
//     that entry (or the previous one) has been modified.
//   - If PrevHash doesn't match the previous entry's CurrHash,
//     the chain is broken (entry inserted or removed).
func VerifyAuditChain(records []AuditChainEntry) (tamperedIndex int, err error) {
	if len(records) == 0 {
		return -1, fmt.Errorf("empty audit chain")
	}

	var prevHash AuditHash = ZeroHash
	for i, rec := range records {
		// Check prev_hash linkage.
		if rec.PrevHash != prevHash {
			return i, fmt.Errorf("%w: broken chain at seq %d (prev_hash mismatch)", ErrAuditHashMismatch, rec.Seq)
		}
		// Recompute curr_hash.
		expected, err := ComputeAuditHash(prevHash, rec.Payload)
		if err != nil {
			return i, fmt.Errorf("hash computation failed at seq %d: %w", rec.Seq, err)
		}
		if rec.CurrHash != expected {
			return i, fmt.Errorf("%w: hash mismatch at seq %d", ErrAuditHashMismatch, rec.Seq)
		}
		prevHash = rec.CurrHash
	}
	return -1, nil
}

// canonicalJSON serializes a value to JSON with sorted keys.
// This ensures deterministic output for hash computation.
//
// Implementation: marshal to map[string]any, then re-marshal with sorted keys.
// This handles nested objects correctly. For arrays, order is preserved
// (array order is significant in JSON, unlike object key order).
func canonicalJSON(v any) ([]byte, error) {
	// First marshal to JSON.
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Unmarshal to interface{} to get generic structure.
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	// Re-marshal with sorted keys.
	return marshalSorted(generic)
}

// marshalSorted marshals a value with sorted object keys (recursive).
func marshalSorted(v any) ([]byte, error) {
	switch x := v.(type) {
	case map[string]any:
		// Sort keys.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Build JSON manually for deterministic key order.
		var buf []byte
		buf = append(buf, '{')
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf = append(buf, kb...)
			buf = append(buf, ':')
			vb, err := marshalSorted(x[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, '}')
		return buf, nil
	case []any:
		var buf []byte
		buf = append(buf, '[')
		for i, item := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			ib, err := marshalSorted(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, ib...)
		}
		buf = append(buf, ']')
		return buf, nil
	default:
		return json.Marshal(v)
	}
}
