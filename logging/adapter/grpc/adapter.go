//go:build grpc

// Package grpc provides gRPC interceptors for the enterprise logging framework.
//
// It wraps the main logger with gRPC-compatible unary and stream server
// interceptors that log each RPC's method, status code, duration, and error
// (if any).
//
// # Quick Start
//
//	log, _ := logger.New(logger.FromEnv())
//	defer log.Close()
//
//	s := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpcadapter.UnaryServerInterceptor(log)),
//	    grpc.StreamInterceptor(grpcadapter.StreamServerInterceptor(log)),
//	)
//
// # Enabling
//
// This adapter is guarded by the "grpc" build tag. Install the dependencies and
// build with the tag:
//
//	go get google.golang.org/grpc
//	go build -tags grpc ./...
package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/natifdevelopment/go-observability/logging/core"
	"github.com/natifdevelopment/go-observability/logging/logger"
)

// grpcCodeLevel maps a gRPC status code to an appropriate log level:
//   - OK: INFO
//   - Client errors (canceled, invalid argument, not found, etc.): WARN
//   - Server errors (internal, unavailable, deadline exceeded, etc.): ERROR
func grpcCodeLevel(code codes.Code) core.Level {
	switch code {
	case codes.OK:
		return core.LevelInfo
	case codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.ResourceExhausted,
		codes.FailedPrecondition,
		codes.Aborted,
		codes.OutOfRange:
		return core.LevelWarn
	default:
		// codes.Unknown, codes.DeadlineExceeded, codes.Internal,
		// codes.Unimplemented, codes.DataLoss, codes.Unavailable
		return core.LevelError
	}
}

// UnaryServerInterceptor returns a gRPC unary server interceptor that logs each
// RPC's method, status code, duration, and error using the provided logger.
//
// The log level is determined by the gRPC status code:
//   - OK: INFO
//   - 4xx-like codes (InvalidArgument, NotFound, etc.): WARN
//   - 5xx-like codes (Internal, Unavailable, etc.): ERROR
func UnaryServerInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		// Log RPC start.
		log.Debug(ctx, "rpc started",
			slog.String(string(core.FieldMethod), info.FullMethod),
		)

		// Execute the handler.
		resp, err := handler(ctx, req)

		duration := time.Since(start)

		attrs := []slog.Attr{
			slog.String(string(core.FieldMethod), info.FullMethod),
			slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
			slog.Duration(string(core.FieldDuration), duration),
		}

		if err != nil {
			st, _ := status.FromError(err)
			code := st.Code()
			attrs = append(attrs,
				slog.String(string(core.FieldStatusCode), code.String()),
				slog.String(string(core.FieldError), err.Error()),
			)

			level := grpcCodeLevel(code)
			msg := "rpc completed with error"
			switch level {
			case core.LevelError:
				log.Error(ctx, msg, attrs...)
			case core.LevelWarn:
				log.Warn(ctx, msg, attrs...)
			default:
				log.Info(ctx, msg, attrs...)
			}
		} else {
			attrs = append(attrs,
				slog.String(string(core.FieldStatusCode), "OK"),
			)
			log.Info(ctx, "rpc completed", attrs...)
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that logs
// each streaming RPC's method, status code, duration, and error using the
// provided logger.
//
// The log level is determined by the gRPC status code (same as the unary
// interceptor).
func StreamServerInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		ctx := ss.Context()

		// Log stream start.
		log.Debug(ctx, "stream started",
			slog.String(string(core.FieldMethod), info.FullMethod),
		)

		// Execute the handler.
		err := handler(srv, ss)

		duration := time.Since(start)

		attrs := []slog.Attr{
			slog.String(string(core.FieldMethod), info.FullMethod),
			slog.Int64(string(core.FieldDurationMs), duration.Milliseconds()),
			slog.Duration(string(core.FieldDuration), duration),
		}

		if err != nil {
			st, _ := status.FromError(err)
			code := st.Code()
			attrs = append(attrs,
				slog.String(string(core.FieldStatusCode), code.String()),
				slog.String(string(core.FieldError), err.Error()),
			)

			level := grpcCodeLevel(code)
			msg := "stream completed with error"
			switch level {
			case core.LevelError:
				log.Error(ctx, msg, attrs...)
			case core.LevelWarn:
				log.Warn(ctx, msg, attrs...)
			default:
				log.Info(ctx, msg, attrs...)
			}
		} else {
			attrs = append(attrs,
				slog.String(string(core.FieldStatusCode), "OK"),
			)
			log.Info(ctx, "stream completed", attrs...)
		}

		return err
	}
}
