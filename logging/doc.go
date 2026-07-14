// Package logging provides an enterprise-grade structured logging framework
// for Go applications, built on top of the standard library's log/slog package.
//
// # Overview
//
// The logging framework is designed for production use in enterprise environments
// with the following key features:
//
//   - Structured JSON logging (Loki/ELK/Datadog compatible)
//   - 7 log levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, PANIC
//   - Automatic sensitive data masking (14 categories)
//   - Immutable audit logging with SHA256 hash chain
//   - Context-first API with trace/request/correlation ID propagation
//   - W3C TraceContext support for cross-service correlation
//   - Panic recovery middleware for HTTP/gRPC
//   - High availability: failover sinks, circuit breaker, async with backpressure
//   - Pluggable: custom sinks, formatters, hooks, filters via interfaces
//   - Framework adapters: net/http, Gin, Echo, Fiber, gRPC, Kafka, RabbitMQ
//   - OpenTelemetry bridge for unified observability
//
// # Quick Start
//
//	log, err := logging.New(logging.FromEnv())
//	if err != nil {
//	    panic(err)
//	}
//	defer log.Close()
//
//	log.Info(ctx, "server started",
//	    logging.String("port", "8080"),
//	)
//
// # Sub-packages
//
//   - core: Domain layer (Level, Fields, Carrier, MaskEngine, EventRegistry, AuditHash)
//   - sink: Output destinations (Console, File, Rotate, Multi, Async, Audit)
//   - handler: slog.Handler implementation with masking and formatting
//   - config: Configuration via environment variables and struct
//   - builder: Functional options and factory for Logger construction
//   - adapter: Framework integrations (Gin, Echo, Fiber, gRPC, Kafka, RabbitMQ)
//   - audit: Immutable audit logging facade
//   - security: Security event logging facade
//   - request: HTTP request logging facade
//   - database: Database query logging facade
//   - external: External API call logging facade
//   - helper: Utility functions (sanitization, redaction, time formatting)
//   - metrics: Logger health metrics and exporters
//   - lifecycle: Graceful shutdown management
//
// # Monitoring Compatibility
//
// All log output is structured JSON with consistent snake_case field names,
// making logs directly queryable in:
//
//   - Grafana Loki (field-based LogQL queries)
//   - Elasticsearch/Kibana (field-based KQL queries)
//   - Datadog (automatic field parsing and faceted search)
//   - Splunk (SPL field extraction)
//
// # Standards Compliance
//
//   - OWASP Logging Cheat Sheet
//   - Google SRE Logging Best Practices
//   - Twelve-Factor App (§11 Logs)
//   - Go Code Review Comments
//   - Effective Go
//   - Clean Architecture / SOLID
package logging
