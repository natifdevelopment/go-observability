# Contributing to go-observability

Thank you for your interest in contributing to `go-observability`! This document outlines the process and guidelines for contributing.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Standards](#code-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Release Process](#release-process)
- [Design Principles](#design-principles)

## Getting Started

### Prerequisites

- Go 1.24+
- Git
- Make (optional, for convenience commands)

### Setup

```bash
git clone https://github.com/natifdevelopment/go-observability.git
cd go-observability
go mod download
go build ./...
go test ./...
```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### 2. Make Changes

Follow the [Code Standards](#code-standards) and [Testing Requirements](#testing-requirements).

### 3. Run Checks

```bash
# Build
go build ./...

# Vet
go vet ./...

# Test
go test ./... -race -count=1

# Coverage (target: 90%+)
go test ./... -cover -coverprofile=coverage.out
go tool cover -func=coverage.out

# Benchmark (if performance-sensitive changes)
go test ./... -bench=. -benchmem -run=^$
```

### 4. Commit

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(logging): add Kafka producer hook
fix(logging/core): handle nil pointer in mask engine
docs(logging): update README with Fiber adapter example
test(logging/sink): add integration test for async sink
refactor(logging/handler): simplify formatter interface
```

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
```

Create a Pull Request on GitHub with:
- Clear description of what and why
- Link to related issues
- Test results (coverage, benchmark if applicable)

## Code Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Use `gofmt` / `goimports` for formatting
- Run `go vet` before committing

### Package Design

- **Accept interfaces, return structs**
- **Context-first parameter** for all I/O operations
- **Functional options** for configurable APIs
- **Immutable after construction** where possible
- **No cyclic imports** — verify with `go vet`

### Error Handling

- Return errors explicitly (no silent failures)
- Use sentinel errors from `core/error.go`
- Wrap with context: `fmt.Errorf("context: %w", err)`
- Use `errors.Is()` / `errors.As()` for error checking

### Naming Conventions

- Package names: lowercase, single word (`core`, `sink`, `handler`)
- Exported types: PascalCase (`Logger`, `MaskEngine`)
- Unexported types: camelCase (`asyncSinkConfig`)
- Constants: PascalCase for exported, camelCase for unexported
- Field names in JSON logs: snake_case (`trace_id`, `user_id`)

### File Organization

- One concern per file
- `doc.go` for package documentation
- Test files: `*_test.go` alongside source
- Benchmark files: `benchmark_test.go`
- Keep files under 500 lines where possible

## Testing Requirements

### Coverage

- **Minimum 90% coverage** per package
- Run: `go test ./... -cover`

### Test Types

| Type | Required | Where |
|---|---|---|
| Unit tests | Yes | `*_test.go` |
| Benchmark tests | For hot paths | `benchmark_test.go` |
| Race tests | Yes (all) | `go test -race` |
| Integration tests | For I/O code | `*_integration_test.go` |

### Test Style

- Table-driven tests for multi-case scenarios
- `t.Run` for subtests
- Descriptive test names: `TestMaskEngine_PasswordPartial`
- Test public API, not implementation details
- Use `testing.T` helpers: `t.Errorf`, `t.Fatalf`, `t.Skip`

### Benchmark Style

```go
func BenchmarkXxx(b *testing.B) {
    // Setup (not timed)
    b.ResetTimer()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        // Code to benchmark
    }
}
```

## Pull Request Process

1. **Self-review** your code before submitting
2. **Run all checks**: `go build && go vet && go test -race -cover`
3. **Update documentation** if API changes
4. **Add changelog entry** (if applicable)
5. **Request review** from maintainers
6. **Address feedback** promptly
7. **Squash commits** before merge (if requested)

### PR Template

```markdown
## Description
[What does this PR do? Why?]

## Changes
- [Change 1]
- [Change 2]

## Testing
- [x] Unit tests pass
- [x] Race detector clean
- [x] Coverage ≥ 90%
- [x] Benchmarks run (if applicable)

## Breaking Changes
[None / List them]
```

## Release Process

This project follows [Semantic Versioning](https://semver.org/):

- **MAJOR**: Breaking API changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Steps

1. Update `CHANGELOG.md`
2. Create tag: `git tag v1.2.3`
3. Push tag: `git push origin v1.2.3`
4. GitHub Actions auto-publishes to pkg.go.dev

## Design Principles

1. **Clean Architecture** — layered, dependency rule (panah ke bawah)
2. **SOLID** — SRP, OCP, LSP, ISP, DIP
3. **DRY** — don't repeat yourself
4. **KISS** — keep it simple
5. **Fail-secure** — when in doubt, mask/drop (security first)
6. **High availability** — logging must never crash the application
7. **Context-first** — all I/O operations accept `context.Context`
8. **Immutability** — configs and carriers are immutable after construction
9. **Composition over inheritance** — use interfaces and embedding
10. **Performance** — zero-allocation hot path where possible

## Security Guidelines

- **Never log credentials** — use MaskEngine
- **Never log raw request bodies** by default — toggle via config
- **Sanitize control characters** — prevent log injection
- **Audit log immutability** — hash chain must not be breakable
- **File permissions** — 0640 for logs, 0600 for audit
- **Report security vulnerabilities** privately to maintainers

## Questions?

- Open an [Issue](https://github.com/natifdevelopment/go-observability/issues)
- Start a [Discussion](https://github.com/natifdevelopment/go-observability/discussions)
