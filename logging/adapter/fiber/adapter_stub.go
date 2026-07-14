//go:build !fiber

// Package fiber provides Fiber middleware for the enterprise logging framework.
//
// This is a stub file. The Fiber adapter is guarded by the "fiber" build tag
// because its dependency (github.com/gofiber/fiber/v2) is not installed by
// default.
//
// # Enabling the Fiber adapter
//
// To enable this adapter, install the dependency and build with the tag:
//
//	go get github.com/gofiber/fiber/v2
//	go build -tags fiber ./...
//
// Once enabled, use the Middleware function:
//
//	app := fiber.New()
//	app.Use(fiberadapter.Middleware(log))
package fiber
