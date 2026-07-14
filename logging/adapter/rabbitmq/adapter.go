//go:build rabbitmq

// Package rabbitmq provides structured logging helpers for RabbitMQ
// consumers and publishers, integrating the rabbitmq/amqp091-go
// delivery types with the enterprise logger.
//
// This file is only compiled when the "rabbitmq" build tag is enabled.
// Without the tag, adapter_stub.go is used instead and all functions
// are no-ops.
//
// # Enabling
//
//	go build -tags rabbitmq
//
// The github.com/rabbitmq/amqp091-go dependency must be available in
// the module (e.g. `go get github.com/rabbitmq/amqp091-go`).
package rabbitmq

import (
	"context"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// LogDelivery logs a RabbitMQ message consumption event at INFO level.
//
// Standard fields recorded: queue_name, msg_key.
// The delivery routing key (if any) is recorded under the msg_key field
// so that consumers can correlate messages across services.
//
// A nil logger or delivery with an empty routing key is handled
// gracefully; a nil logger is a no-op.
func LogDelivery(log *logger.Logger, delivery amqp.Delivery) {
	if log == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String(string(core.FieldQueueName), delivery.Queue),
	}
	if delivery.RoutingKey != "" {
		attrs = append(attrs, slog.String(string(core.FieldMsgKey), delivery.RoutingKey))
	}
	log.Info(context.Background(), "rabbitmq: message consumed", attrs...)
}

// LogPublish logs a RabbitMQ publish event at INFO level.
//
// Standard fields recorded: exchange, routing_key (as msg_key), duration.
// The duration reflects the time taken to publish the message to the
// broker and is useful for latency monitoring.
//
// A nil logger is a no-op.
func LogPublish(log *logger.Logger, exchange, routingKey string, duration time.Duration) {
	if log == nil {
		return
	}
	log.Info(context.Background(), "rabbitmq: message published",
		slog.String("exchange", exchange),
		slog.String(string(core.FieldMsgKey), routingKey),
		slog.Duration(string(core.FieldDuration), duration),
	)
}

// LogError logs a RabbitMQ error at ERROR level.
//
// The error is recorded under the standard error field and the queue
// under the queue_name field so that operators can filter RabbitMQ
// errors by queue in log aggregation systems.
//
// A nil logger is a no-op. If err is nil, the call is still logged but
// no error field is appended.
func LogError(log *logger.Logger, queue string, err error) {
	if log == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String(string(core.FieldQueueName), queue),
	}
	if err != nil {
		attrs = append(attrs, slog.String(string(core.FieldError), err.Error()))
	}
	log.Error(context.Background(), "rabbitmq: error", attrs...)
}
