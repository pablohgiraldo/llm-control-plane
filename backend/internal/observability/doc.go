// Package observability provides structured logging, metrics, and tracing
// for the LLM Control Plane.
//
// This package implements:
//   - Structured logging with contextual fields (zap-based)
//   - Prometheus-compatible metrics collection
//   - Distributed tracing (OpenTelemetry/Datadog)
//   - Request ID propagation
//   - Performance instrumentation
//
// All pipeline stages are instrumented for full observability.
package observability
