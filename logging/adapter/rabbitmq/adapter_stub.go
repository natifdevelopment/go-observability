//go:build !rabbitmq

// Package rabbitmq provides structured logging helpers for RabbitMQ
// consumers and publishers.
//
// This is the stub file used when the "rabbitmq" build tag is NOT
// enabled. All functions are no-ops so that code importing this package
// compiles without the github.com/rabbitmq/amqp091-go dependency.
// External types (amqp.Delivery) are replaced with any so the stub has
// no third-party imports.
//
// # Enabling the real implementation
//
// To enable the real RabbitMQ adapter, build with the "rabbitmq" tag
// and add the dependency:
//
//	go get github.com/rabbitmq/amqp091-go
//	go build -tags rabbitmq
//
// When the tag is active, adapter.go (instead of this file) is compiled
// and the functions emit structured log records.
package rabbitmq

import (
	"time"

	"github.com/natifdevelopment/go-observability/logging/logger"
)

// LogDelivery is a no-op stub for logging a RabbitMQ message consumption.
//
// To enable the real implementation, build with the "rabbitmq" tag:
//
//	go get github.com/rabbitmq/amqp091-go
//	go build -tags rabbitmq
func LogDelivery(log *logger.Logger, delivery any) {}

// LogPublish is a no-op stub for logging a RabbitMQ publish.
//
// To enable the real implementation, build with the "rabbitmq" tag:
//
//	go get github.com/rabbitmq/amqp091-go
//	go build -tags rabbitmq
func LogPublish(log *logger.Logger, exchange, routingKey string, duration time.Duration) {
}

// LogError is a no-op stub for logging a RabbitMQ error.
//
// To enable the real implementation, build with the "rabbitmq" tag:
//
//	go get github.com/rabbitmq/amqp091-go
//	go build -tags rabbitmq
func LogError(log *logger.Logger, queue string, err error) {}
