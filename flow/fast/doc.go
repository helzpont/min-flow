// Package fast provides high-performance stream processing primitives that
// sacrifice min-flow's safety and observability features for raw speed.
//
// # DISCLAIMER: USE AT YOUR OWN RISK
//
// This package intentionally bypasses the following min-flow features:
//
//   - Result wrapping: Values flow directly without Ok/Err/Sentinel wrappers
//   - Error handling: Panics from user functions are NOT recovered
//   - Context cancellation: No per-item context checks (only at channel ops)
//   - Interceptors: No delegate registry lookups or event invocations
//   - Backpressure signals: No sentinel propagation
//
// # When to use this package
//
//   - CPU-bound transformations where channel overhead dominates
//   - Trusted code paths where panics are acceptable
//   - Benchmarking to measure the cost of min-flow's features
//   - Hot paths in production after extensive testing
//
// # When NOT to use this package
//
//   - Processing untrusted or user-provided data
//   - I/O-bound operations where safety matters more than speed
//   - When you need error propagation or recovery
//   - When observability (metrics, logging) is required
//
// This package serves a secondary purpose: benchmarking the overhead of each
// min-flow feature to identify optimization opportunities in the core package.
package fast
