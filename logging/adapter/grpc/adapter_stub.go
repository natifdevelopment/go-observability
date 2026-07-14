//go:build !grpc

// Package grpc provides gRPC interceptors for the enterprise logging framework.
//
// This is a stub file. The gRPC adapter is guarded by the "grpc" build tag
// because its dependency (google.golang.org/grpc) is not installed by default.
//
// # Enabling the gRPC adapter
//
// To enable this adapter, install the dependency and build with the tag:
//
//	go get google.golang.org/grpc
//	go build -tags grpc ./...
//
// Once enabled, use the interceptors:
//
//	s := grpc.NewServer(
//	    grpc.UnaryInterceptor(grpcadapter.UnaryServerInterceptor(log)),
//	    grpc.StreamInterceptor(grpcadapter.StreamServerInterceptor(log)),
//	)
package grpc
