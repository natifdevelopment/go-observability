# go-observability

[![Go Reference](https://pkg.go.dev/badge/github.com/natifdevelopment/go-observability.svg)](https://pkg.go.dev/github.com/natifdevelopment/go-observability)
[![Go Report Card](https://goreportcard.com/badge/github.com/natifdevelopment/go-observability)](https://goreportcard.com/report/github.com/natifdevelopment/go-observability)
[![CI Status](https://github.com/natifdevelopment/go-observability/workflows/CI/badge.svg)](https://github.com/natifdevelopment/go-observability/actions)
[![Coverage](https://codecov.io/gh/natifdevelopment/go-observability/branch/main/graph/badge.svg)](https://codecov.io/gh/natifdevelopment/go-observability)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Enterprise-grade observability framework for Go applications.

## Features

### Logging (`logging/`)

- **Structured JSON logging** — Loki/ELK/Datadog/Splunk compatible
- **7 log levels** — TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC
- **Automatic sensitive data masking** — 14 categories (password, JWT, credit card, NIK, NPWP, etc.)
- **Immutable audit logging** — SHA256 hash chain, tamper-evident
- **Context-first API** — trace/request/correlation ID propagation via `context.Context`
- **W3C TraceContext** — cross-service correlation (microservices)
- **Panic recovery** — middleware for net/http, Gin, Echo, Fiber, gRPC
- **High availability** — failover sinks, circuit breaker, async with backpressure
- **Pluggable architecture** — custom sinks, formatters, hooks, filters
- **Framework adapters** — Gin, Echo, Fiber, gRPC, Kafka, RabbitMQ
- **OpenTelemetry bridge** — log-to-trace correlation
- **Performance** — zero-allocation hot path, sync.Pool, reflect cache

### Metrics (`metrics/`) — *Planned*

### Tracing (`tracing/`) — *Planned*

## Quick Start

```bash
go get github.com/natifdevelopment/go-observability/logging
```

```go
package main

import (
    "context"
    "github.com/natifdevelopment/go-observability/logging"
    "github.com/natifdevelopment/go-observability/logging/core"
)

func main() {
    log, err := logging.New(logging.FromEnv())
    if err != nil {
        panic(err)
    }
    defer log.Close()

    ctx := context.Background()
    ctx = core.WithTraceID(ctx, "trace-123")
    ctx = core.WithUser(ctx, "user-1", "john", "admin")

    log.Info(ctx, "server started",
        core.ServiceAttr("my-api"),
        core.EnvironmentAttr("production"),
    )
}
```

## Configuration (Environment Variables)

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `INFO` | Log level (TRACE/DEBUG/INFO/WARN/ERROR/FATAL/PANIC) |
| `LOG_FORMAT` | `json` | Output format (json/console) |
| `LOG_OUTPUT` | `console` | Output destination (console/file/both) |
| `LOG_FILE` | — | Log file path |
| `LOG_AUDIT_FILE` | — | Audit log file path |
| `LOG_MAX_SIZE` | `100` | Max log file size in MB |
| `LOG_MAX_BACKUP` | `10` | Max number of old log files to retain |
| `LOG_MAX_AGE` | `30` | Max number of days to retain old log files |
| `LOG_COMPRESS` | `false` | Compress rotated log files |
| `SERVICE_NAME` | `unknown` | Service name |
| `SERVICE_VERSION` | `0.0.0` | Service version |
| `ENVIRONMENT` | `development` | Environment (development/staging/production) |
| `ENABLE_CALLER` | `true` | Include caller file:line:function |
| `ENABLE_STACKTRACE` | `true` | Include stacktrace for ERROR+ |
| `ENABLE_COLOR` | `false` | Color output in console mode |
| `ENABLE_BODY_LOG` | `false` | Log HTTP request/response bodies |
| `ENABLE_ASYNC` | `false` | Enable async logging |
| `LOG_MASK_ENABLED` | `true` | Enable sensitive data masking |
| `LOG_AUDIT_ENABLED` | `false` | Enable audit logging |

Full list in [`logging/core/constant.go`](logging/core/constant.go).

## Framework Adapters

### Gin

```go
import ginadapter "github.com/natifdevelopment/go-observability/logging/adapter/gin"

r := gin.New()
r.Use(ginadapter.Middleware(log))
```

### Echo

```go
import echoadapter "github.com/natifdevelopment/go-observability/logging/adapter/echo"

e := echo.New()
e.Use(echoadapter.Middleware(log))
```

### Fiber

```go
import fiberadapter "github.com/natifdevelopment/go-observability/logging/adapter/fiber"

app := fiber.New()
app.Use(fiberadapter.Middleware(log))
```

### gRPC

```go
import grpcadapter "github.com/natifdevelopment/go-observability/logging/adapter/grpc"

srv := grpc.NewServer(
    grpc.UnaryInterceptor(grpcadapter.UnaryInterceptor(log)),
)
```

### net/http

```go
import httpadapter "github.com/natifdevelopment/go-observability/logging/adapter/http"

handler := httpadapter.Middleware(log)(myHandler)
http.ListenAndServe(":8080", handler)
```

## Audit Logging

```go
err := log.Audit().Log(ctx, audit.AuditRecord{
    User:     "user-123",
    Role:     "admin",
    Action:   "user.update",
    Resource: "user:456",
    Before:   oldUser,
    After:    newUser,
    IP:       clientIP,
    Reason:   "admin edit",
})
if err != nil {
    // Audit failure must be handled explicitly (compliance)
}
```

### Verify Audit Integrity

```go
tamperedIndex, err := log.Audit().Verify(ctx)
if err != nil {
    log.Error(ctx, "audit chain tampered", core.ErrorCodeAttr("AUDIT_TAMPER"))
}
```

## Security Logging

```go
log.Security().LoginFailed(ctx, username, ip,
    slog.Int("attempts", 5),
)

log.Security().SQLInjection(ctx, maliciousQuery, ip)

log.Security().BruteForce(ctx, ip, attemptCount)
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  CONSUMER LAYER  (adapter/)                              │
│  net/http │ Gin │ Echo │ Fiber │ gRPC │ Kafka │ RabbitMQ │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│  FACADE LAYER  (logging/)                                │
│  Logger │ Builder │ Factory │ Functional Options         │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│  DOMAIN LAYER  (logging/core/)                           │
│  Level │ Fields │ Carrier │ MaskEngine │ EventRegistry   │
│  AuditHash │ PanicCapture │ Hook │ Filter                │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│  HANDLER LAYER  (logging/handler/)                       │
│  EnterpriseHandler (slog.Handler) │ Formatter            │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────┐
│  SINK LAYER  (logging/sink/)                             │
│  Console │ File │ Rotate │ Multi(failover) │ Async       │
│  Audit(hash chain) │ Buffered │ RateLimit │ Retry │ Batch │
└─────────────────────────────────────────────────────────┘
```

## Package Structure

```
go-observability/
├── logging/              # Enterprise logging framework
│   ├── core/             # Domain layer (pure Go, no I/O)
│   ├── handler/          # slog.Handler implementation
│   ├── sink/             # Output destinations (port + adapters)
│   ├── config/           # Configuration (env + struct)
│   ├── builder/          # Functional options + factory
│   ├── adapter/          # Framework integrations
│   │   ├── http/         # net/http middleware
│   │   ├── gin/          # Gin adapter
│   │   ├── echo/         # Echo adapter
│   │   ├── fiber/        # Fiber adapter
│   │   ├── grpc/         # gRPC interceptors
│   │   ├── kafka/        # Kafka producer/consumer hooks
│   │   ├── rabbitmq/     # RabbitMQ producer/consumer hooks
│   │   └── otel/         # OpenTelemetry bridge
│   ├── audit/            # Audit logging facade
│   ├── security/         # Security event logging facade
│   ├── request/          # HTTP request logging facade
│   ├── database/         # Database query logging facade
│   ├── external/         # External API logging facade
│   ├── helper/           # Utility functions
│   ├── integration/      # Third-party integrations
│   │   └── sentry/       # Sentry error tracking hook (build tag: sentry)
│   ├── metrics/          # Logger health metrics
│   └── lifecycle/        # Graceful shutdown
├── metrics/              # Metrics framework (planned)
├── tracing/              # Tracing framework (planned)
├── go.mod
└── README.md
```

## Sentry Integration (Error Tracking)

The framework includes a Sentry hook that automatically captures ERROR, FATAL,
and PANIC log records and sends them to Sentry for error tracking and alerting.

### Enabling

```bash
go get github.com/getsentry/sentry-go
go build -tags sentry
```

Without the `sentry` build tag, the hook is a no-op stub (zero overhead,
no dependency required).

### Quick Start

```go
import (
    "github.com/natifdevelopment/go-observability/logging/builder"
    sentryhook "github.com/natifdevelopment/go-observability/logging/integration/sentry"
    "github.com/natifdevelopment/go-observability/logging/logger"
)

sentryHook, err := sentryhook.NewHookFromEnv()
if err != nil {
    log.Fatalf("sentry init failed: %v", err)
}
defer sentryHook.Flush()

log, _ := logger.NewWithBuilder(builder.New(
    builder.WithConfig(logger.FromEnv()),
    builder.WithHooks(sentryHook),
))
```

### Configuration (Environment Variables)

| Variable | Default | Description |
|---|---|---|
| `SENTRY_DSN` | — | Sentry project DSN (empty = disabled) |
| `SENTRY_SAMPLE_RATE` | `1.0` | Fraction of events to send (0.0–1.0) |
| `SERVICE_NAME` | `unknown` | Service tag on Sentry events |
| `ENVIRONMENT` | `development` | Environment tag |
| `SERVICE_VERSION` | `0.0.0` | Release tag |

### Features

- **Automatic capture**: ERROR, FATAL, PANIC logs sent to Sentry
- **Breadcrumbs**: INFO/WARN logs recorded as breadcrumbs for context
- **Structured tags**: `service`, `environment`, `trace_id`, `request_id` attached
- **Stacktrace**: Sentry renders stacktraces from log records
- **Sampling**: Control event volume via `SENTRY_SAMPLE_RATE`
- **Graceful degradation**: Empty DSN = no-op (safe for development)

## Standards Compliance

- [OWASP Logging Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Cheat_Sheet.html)
- [Google SRE Logging Best Practices](https://sre.google/sre-book/monitoring-distributed-systems/)
- [Twelve-Factor App §11 Logs](https://12factor.net/logs)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
- Clean Architecture / SOLID Principles

## Performance

Benchmark results (Apple M1, arm64):

| Operation | ns/op | allocs/op |
|---|---|---|
| Mask (no sensitive) | 2,559 | 0 |
| Mask (4 sensitive fields) | 1,020 | 4 |
| Mask (disabled) | 2.2 | 0 |
| Carrier extract (ctx) | 14.1 | 0 |
| Level filter | 0.34 | 0 |
| DynamicLevel.Get | 0.86 | 0 |
| Audit hash (SHA256) | 3,591 | 48 |

## Requirements

- Go 1.24+
- `gopkg.in/natefinch/lumberjack.v2` (log rotation, only dependency)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).
