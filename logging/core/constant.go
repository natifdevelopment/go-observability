package core

// constant.go defines shared constants used across the framework.
// Centralized here to avoid magic strings and ensure consistency.

const (
	// DefaultTimeFormat is the default timestamp format for log output.
	// RFC3339Nano is ISO8601 compliant, sortable, and human-readable.
	// Compatible with Loki, Elasticsearch, Datadog timestamp parsing.
	DefaultTimeFormat = "2006-01-02T15:04:05.999999999Z07:00"

	// DefaultMaskChar is the default character used for masking.
	DefaultMaskChar = "*"

	// DefaultMaskPreserveFirst is the default number of leading chars preserved.
	DefaultMaskPreserveFirst = 0

	// DefaultMaskPreserveLast is the default number of trailing chars preserved.
	DefaultMaskPreserveLast = 0

	// DefaultAuditHashAlgo is the default hash algorithm for audit chain.
	DefaultAuditHashAlgo = "sha256"

	// DefaultSlowQueryThresholdMs is the default slow query threshold in milliseconds.
	DefaultSlowQueryThresholdMs = 200

	// DefaultBodyMaxBytes is the default max body size for request body logging.
	DefaultBodyMaxBytes = 1024

	// DefaultAsyncBufferSize is the default async channel buffer size.
	DefaultAsyncBufferSize = 4096

	// DefaultAsyncWorkers is the default number of async worker goroutines.
	DefaultAsyncWorkers = 1

	// DefaultShutdownTimeoutMs is the default graceful shutdown timeout in milliseconds.
	DefaultShutdownTimeoutMs = 5000

	// DefaultSinkWriteTimeoutMs is the default timeout for a single sink write.
	DefaultSinkWriteTimeoutMs = 2000

	// DefaultDedupWindowSize is the default dedup filter window size (entries).
	DefaultDedupWindowSize = 10000

	// DefaultDedupWindowDuration is the default dedup filter window duration.
	DefaultDedupWindowDuration = "5m"

	// DefaultBatchSize is the default batch size for BatchSink.
	DefaultBatchSize = 100

	// DefaultBatchFlushInterval is the default batch flush interval.
	DefaultBatchFlushInterval = "1s"

	// DefaultRetryMaxAttempts is the default max retry attempts for RetrySink.
	DefaultRetryMaxAttempts = 3

	// DefaultRetryInitialDelay is the default initial retry delay.
	DefaultRetryInitialDelay = "100ms"

	// DefaultRetryMaxDelay is the default max retry delay.
	DefaultRetryMaxDelay = "1s"

	// DefaultRetryMultiplier is the default retry backoff multiplier.
	DefaultRetryMultiplier = 2.0

	// ReflectCacheMaxTypes is the max number of struct types cached for masking reflect.
	ReflectCacheMaxTypes = 1000
)

// EnvKey defines environment variable names for configuration.
// Centralized to avoid typos and enable documentation generation.
const (
	EnvLogLevel            = "LOG_LEVEL"
	EnvLogFormat           = "LOG_FORMAT"
	EnvLogOutput           = "LOG_OUTPUT"
	EnvLogFile             = "LOG_FILE"
	EnvLogAuditFile        = "LOG_AUDIT_FILE"
	EnvLogMaxSize          = "LOG_MAX_SIZE"
	EnvLogMaxBackup        = "LOG_MAX_BACKUP"
	EnvLogMaxAge           = "LOG_MAX_AGE"
	EnvLogCompress         = "LOG_COMPRESS"
	EnvServiceName         = "SERVICE_NAME"
	EnvServiceVersion      = "SERVICE_VERSION"
	EnvEnvironment         = "ENVIRONMENT"
	EnvApplication         = "APPLICATION"
	EnvEnableCaller        = "ENABLE_CALLER"
	EnvEnableStacktrace    = "ENABLE_STACKTRACE"
	EnvEnableColor         = "ENABLE_COLOR"
	EnvEnableBodyLog       = "ENABLE_BODY_LOG"
	EnvEnableAsync         = "ENABLE_ASYNC"
	EnvAsyncBufferSize     = "LOG_ASYNC_BUFFER_SIZE"
	EnvAsyncWorkers        = "LOG_ASYNC_WORKERS"
	EnvBackpressure        = "LOG_BACKPRESSURE"
	EnvSlowQueryThreshold  = "LOG_SLOW_QUERY_THRESHOLD"
	EnvMaskEnabled         = "LOG_MASK_ENABLED"
	EnvAuditEnabled        = "LOG_AUDIT_ENABLED"
	EnvAuditHashAlgo       = "LOG_AUDIT_HASH_ALGO"
	EnvTimeFormat          = "LOG_TIME_FORMAT"
	EnvSamplingRate        = "LOG_SAMPLING_RATE"
	EnvSinkRateLimit       = "LOG_SINK_RATE_LIMIT"
	EnvShutdownTimeout     = "LOG_SHUTDOWN_TIMEOUT"
	EnvSinkWriteTimeout    = "LOG_SINK_WRITE_TIMEOUT"
	EnvBodyMaxBytes        = "LOG_BODY_MAX_BYTES"
	EnvHeaderWhitelist     = "LOG_HEADER_WHITELIST"
	EnvHeaderBlacklist     = "LOG_HEADER_BLACKLIST"
	EnvKafkaBodyLogTopics  = "LOG_KAFKA_BODY_TOPICS"
	EnvRabbitMQBodyLogQueues = "LOG_RABBITMQ_BODY_LOG_QUEUES"
)

// DefaultHeaderBlacklist is the default list of headers that are NEVER logged.
// Security: prevent credential/PII leakage in request logs.
var DefaultHeaderBlacklist = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-API-Key",
	"X-Auth-Token",
	"Proxy-Authorization",
	"X-CSRF-Token",
}
