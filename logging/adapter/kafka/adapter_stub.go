//go:build !kafka

// Package kafka provides structured logging helpers for Kafka consumers
// and producers.
//
// This is the stub file used when the "kafka" build tag is NOT enabled.
// All functions are no-ops so that code importing this package compiles
// without the github.com/segmentio/kafka-go dependency. External types
// (kafka.Message) are replaced with any so the stub has no third-party
// imports.
//
// # Enabling the real implementation
//
// To enable the real Kafka adapter, build with the "kafka" tag and add
// the dependency:
//
//	go get github.com/segmentio/kafka-go
//	go build -tags kafka
//
// When the tag is active, adapter.go (instead of this file) is compiled
// and the functions emit structured log records.
package kafka

import (
	"time"

	"github.com/natifdevelopment/go-observability/logging/logger"
)

// LogMessage is a no-op stub for logging a Kafka message consumption.
//
// To enable the real implementation, build with the "kafka" tag:
//
//	go get github.com/segmentio/kafka-go
//	go build -tags kafka
func LogMessage(log *logger.Logger, msg any) {}

// LogProduce is a no-op stub for logging a Kafka produce.
//
// To enable the real implementation, build with the "kafka" tag:
//
//	go get github.com/segmentio/kafka-go
//	go build -tags kafka
func LogProduce(log *logger.Logger, topic string, partition int32, offset int64, duration time.Duration) {
}

// LogError is a no-op stub for logging a Kafka error.
//
// To enable the real implementation, build with the "kafka" tag:
//
//	go get github.com/segmentio/kafka-go
//	go build -tags kafka
func LogError(log *logger.Logger, topic string, err error) {}
