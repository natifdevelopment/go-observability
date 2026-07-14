package core

// This file is intentionally minimal. The PanicInfo struct and CapturePanic
// functions are defined in stacktrace.go (they share the runtime/debug import).
//
// This file exists per the package design to keep the panic-recovery concern
// in its own file for discoverability, even though the implementation
// lives alongside stacktrace.go due to shared imports.
//
// See stacktrace.go for:
//   - PanicInfo struct
//   - CapturePanic(fn func()) (PanicInfo, bool)
//   - CapturePanicCtx(ctx context.Context, fn func()) (PanicInfo, bool)
