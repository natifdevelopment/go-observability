//go:build !echo

// Package echo provides Echo middleware for the enterprise logging framework.
//
// This is a stub file. The Echo adapter is guarded by the "echo" build tag
// because its dependency (github.com/labstack/echo/v4) is not installed by
// default.
//
// # Enabling the Echo adapter
//
// To enable this adapter, install the dependency and build with the tag:
//
//	go get github.com/labstack/echo/v4
//	go build -tags echo ./...
//
// Once enabled, use the Middleware function:
//
//	e := echo.New()
//	e.Use(echoadapter.Middleware(log))
package echo
