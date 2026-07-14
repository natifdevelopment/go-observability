# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `logging/core/` package with domain layer:
  - `Level` — 7 log levels (TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC)
  - `Fields` — 50+ standardized field name constants (snake_case)
  - `Carrier` — immutable context.Context carrier for trace/request/correlation IDs
  - `MaskEngine` — automatic sensitive data masking (14 categories)
  - `EventRegistry` — extensible business & security event registry (31 default events)
  - `AuditHash` — SHA256 hash chain for immutable audit log
  - `Hook` / `Filter` interfaces for extensibility
  - `DynamicLevel` — runtime-changeable level threshold
  - `CapturePanic` — panic recovery helper
  - W3C TraceContext extraction/injection
- `logging/` package documentation
- `metrics/` package placeholder (planned)
- `tracing/` package placeholder (planned)
- README, LICENSE, CONTRIBUTING, CI/CD pipeline
- 91% test coverage with race detector clean
- Benchmarks: zero-allocation hot path for masking, <1ns for level filter

### Security
- Masking defaults to fail-secure (all categories enabled)
- CVV fields are always dropped (never logged)
- Typed context keys prevent carrier injection
- Canonical JSON (sorted keys) for deterministic audit hash
