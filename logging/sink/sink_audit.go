package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/natifdevelopment/go-observability/logging/core"
)

// AuditSink writes audit records to an append-only file with SHA256 hash chain.
//
// Security:
//   - File opened with O_APPEND|O_CREATE|O_WRONLY, mode 0600 (owner-only).
//   - Each record is hash-chained: curr = SHA256(prev || canonical_JSON(payload)).
//   - File is never rewritten; records can only be appended.
//   - Tamper-evident: Verify() replays the chain to detect modifications.
//
// HA:
//   - Audit writes are ALWAYS synchronous (not async) to preserve hash chain order.
//   - Audit throughput is typically low (business/security events only).
type AuditSink struct {
	file     *os.File
	name     string
	prevHash core.AuditHash
	seq      atomic.Uint64
	mu       sync.Mutex
	closed   atomic.Bool
}

// AuditSinkOption configures an AuditSink.
type AuditSinkOption func(*AuditSink)

// NewAuditSink opens an audit file for append-only writing with hash chain.
func NewAuditSink(path string, opts ...AuditSinkOption) (*AuditSink, error) {
	if path == "" {
		return nil, fmt.Errorf("sink: audit file path is empty")
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrAuditFileOpen, err)
	}

	s := &AuditSink{
		file:     f,
		name:     "audit:" + path,
		prevHash: core.ZeroHash,
	}

	// Try to resume chain from existing file content.
	// Read the last line to get the last curr_hash.
	if err := s.resumeChain(); err != nil {
		// Non-fatal: start fresh chain.
		s.prevHash = core.ZeroHash
	}

	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// resumeChain reads the last line of the audit file to resume the hash chain.
func (s *AuditSink) resumeChain() error {
	// Get file size.
	info, err := s.file.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return nil // empty file, start fresh
	}

	// Reopen file for reading (the write handle is O_WRONLY).
	path := s.file.Name()
	readFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer readFile.Close()

	// Read last 4096 bytes (or whole file if smaller).
	size := info.Size()
	readSize := int64(4096)
	if readSize > size {
		readSize = size
	}
	buf := make([]byte, readSize)
	if _, err := readFile.ReadAt(buf, size-readSize); err != nil {
		return err
	}

	// Find the last complete line.
	lines := splitLines(buf)
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if len(line) == 0 {
			continue
		}
		var entry struct {
			Seq      uint64         `json:"seq"`
			CurrHash core.AuditHash `json:"curr_hash"`
		}
		if err := json.Unmarshal(line, &entry); err == nil && entry.CurrHash != "" {
			s.prevHash = entry.CurrHash
			s.seq.Store(entry.Seq)
			return nil
		}
	}
	return nil
}

// WriteAudit writes a single audit record with hash chain.
// This does NOT implement the Sink interface (different signature).
func (s *AuditSink) WriteAudit(_ context.Context, payload core.AuditPayload) error {
	if s.closed.Load() {
		return core.ErrAuditWrite
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Compute hash.
	hash, err := core.ComputeAuditHash(s.prevHash, payload)
	if err != nil {
		return err
	}

	// Build the chain entry.
	seq := s.seq.Add(1)
	entry := core.AuditChainEntry{
		Seq:      seq,
		PrevHash: s.prevHash,
		Payload:  payload,
		CurrHash: hash,
	}

	// Marshal to JSON.
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("%w: marshal failed: %v", core.ErrAuditWrite, err)
	}
	data = append(data, '\n')

	// Write to file.
	if _, err := s.file.Write(data); err != nil {
		return fmt.Errorf("%w: write failed: %v", core.ErrAuditWrite, err)
	}

	// Update chain state.
	s.prevHash = hash
	return nil
}

// Verify reads the audit file and verifies the hash chain integrity.
// Returns the index of the first tampered entry (-1 if all valid).
func (s *AuditSink) Verify(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get file path from Stat.
	info, err := s.file.Stat()
	if err != nil {
		return -1, err
	}
	size := info.Size()
	if size == 0 {
		return -1, nil // empty file is valid
	}

	// Reopen file for reading (the write file handle is O_WRONLY).
	path := s.file.Name()
	readFile, err := os.Open(path)
	if err != nil {
		return -1, err
	}
	defer readFile.Close()

	buf := make([]byte, size)
	if _, err := readFile.Read(buf); err != nil {
		return -1, err
	}

	lines := splitLines(buf)
	records := make([]core.AuditChainEntry, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry core.AuditChainEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue // skip malformed lines
		}
		records = append(records, entry)
	}

	if len(records) == 0 {
		return -1, nil
	}

	return core.VerifyAuditChain(records)
}

func (s *AuditSink) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

func (s *AuditSink) Name() string {
	return s.name
}

// PrevHash returns the current hash chain tip (for inspection/debugging).
func (s *AuditSink) PrevHash() core.AuditHash {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prevHash
}

// Seq returns the current sequence number.
func (s *AuditSink) Seq() uint64 {
	return s.seq.Load()
}

// splitLines splits a byte slice into lines (excluding newline chars).
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			line := data[start:i]
			// Trim carriage return if present.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	// Handle last line without trailing newline.
	if start < len(data) {
		line := data[start:]
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
	}
	return lines
}
