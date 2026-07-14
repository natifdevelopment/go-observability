// Package core contains the domain layer of the enterprise logging framework.
//
// This package is pure Go with NO I/O and NO dependency on slog handler/sink.
// It provides:
//   - Level: 7 log levels (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC)
//   - Fields: standardized field name constants for cross-service log querying
//   - Carrier: immutable context.Context carrier for trace/request/correlation IDs
//   - MaskEngine: automatic sensitive data masking (14 categories)
//   - EventRegistry: extensible business & security event registry
//   - AuditHash: SHA256 hash chain for immutable audit log
//   - Panic capture helper
//   - Hook & Filter interfaces for extensibility
//
// Design principles:
//   - All types are immutable after construction (thread-safe by design)
//   - All functions are pure where possible (testable without I/O)
//   - No cyclic dependencies (core imports nothing from sibling packages)
//
// Monitoring compatibility:
//   - Field names are snake_case (queryable in Loki/ELK/Datadog)
//   - Level labels are uppercase strings (Grafana label-friendly)
//   - Timestamp format RFC3339Nano (sortable, ISO8601 compliant)
package core
