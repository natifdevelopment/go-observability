//go:build kafka

// Package kafka provides structured logging helpers for Kafka consumers
// and producers, integrating the segmentio/kafka-go message types with
// the enterprise logger.
//
// This file is only compiled when the "kafka" build tag is enabled.
// Without the tag, adapter_stub.go is used instead and all functions
// are no-ops.
//
// # Enabling
//
//	go build -tags kafka
//
// The github.com/segmentio/kafka-go dependency must be available in
// the module (e.g. `go get github.com/segmentio/kafka-go`).
package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// LogMessage logs a Kafka message consumption event at INFO level.
//
// Standard fields recorded: topic, partition, offset, msg_key.
// The message key (if any) is recorded under the msg_key field so that
// consumers can correlate messages across services.
//
// A nil logger or message is a no-op.
func LogMessage(log *logger.Logger, msg *kafka.Message) {
	if log == nil || msg == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String(string(core.FieldTopic), msg.Topic),
		slog.Int(string(core.FieldPartition), int(msg.Partition)),
		slog.Int64(string(core.FieldOffset), msg.Offset),
	}
	if len(msg.Key) > 0 {
		attrs = append(attrs, slog.String(string(core.FieldMsgKey), string(msg.Key)))
	}
	log.Info(context.Background(), "kafka: message consumed", attrs...)
}

// LogProduce logs a Kafka produce event at INFO level.
//
// Standard fields recorded: topic, partition, offset, duration.
// The duration reflects the time taken to produce the message to the
// broker and is useful for latency monitoring.
//
// A nil logger is a no-op.
func LogProduce(log *logger.Logger, topic string, partition int32, offset int64, duration time.Duration) {
	if log == nil {
		return
	}
	log.Info(context.Background(), "kafka: message produced",
		slog.String(string(core.FieldTopic), topic),
		slog.Int(string(core.FieldPartition), int(partition)),
		slog.Int64(string(core.FieldOffset), offset),
		slog.Duration(string(core.FieldDuration), duration),
	)
}

// LogError logs a Kafka error at ERROR level.
//
// The error is recorded under the standard error field and the topic
// under the topic field so that operators can filter Kafka errors by
// topic in log aggregation systems.
//
// A nil logger is a no-op. If err is nil, the call is still logged but
// no error field is appended.
func LogError(log *logger.Logger, topic string, err error) {
	if log == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String(string(core.FieldTopic), topic),
	}
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	log.Error(context.Background(), "kafka: error", attrs...)
}
