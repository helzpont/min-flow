package flowerrors

import (
	"github.com/lguimbarda/min-flow/flow/core"
)

// This file provides convenience functions for registering error-handling interceptors.
// These interceptors observe errors for logging, counting, and collection purposes.
// They do not modify the data flow - for error transformation (retry, fallback, catch),
// use the transformer-based operators in error.go and resilience.go.
//
// Usage pattern:
//
//	ctx, registry := core.WithRegistry(context.Background())
//	flowerrors.OnErrorDo(registry, func(err error) { log.Error(err) })
//	counter, _ := flowerrors.WithErrorCounter(registry, nil)
//
//	// Stream processing automatically invokes registered observers
//	result := someMapper.Apply(ctx, stream)
//	fmt.Printf("Errors encountered: %d\n", counter.Count())

// OnErrorDo registers a callback that fires for each error in any stream.
// This is the interceptor-based alternative to the OnError transformer.
func OnErrorDo(registry *core.Registry, handler func(error)) error {
	interceptor := NewErrorHandlerInterceptor(handler)
	return registry.Register(interceptor)
}

// WithErrorCounter registers an error counter interceptor and returns it for querying.
// If predicate is nil, all errors are counted.
func WithErrorCounter(registry *core.Registry, predicate func(error) bool) (*ErrorCounterInterceptor, error) {
	counter := NewErrorCounterInterceptor(predicate)
	if err := registry.Register(counter); err != nil {
		return nil, err
	}
	return counter, nil
}

// WithErrorCollector registers an error collector interceptor and returns it for querying.
func WithErrorCollector(registry *core.Registry, opts ...ErrorCollectorOption) (*ErrorCollectorInterceptor, error) {
	collector := NewErrorCollectorInterceptor(opts...)
	if err := registry.Register(collector); err != nil {
		return nil, err
	}
	return collector, nil
}

// WithCircuitBreakerMonitor registers a circuit breaker interceptor that monitors error rates.
// When threshold errors occur, it calls onThresholdHit and returns ErrCircuitOpen from Do().
func WithCircuitBreakerMonitor(registry *core.Registry, threshold int, onThresholdHit func()) (*CircuitBreakerInterceptor, error) {
	cb := NewCircuitBreakerInterceptor(threshold, onThresholdHit)
	if err := registry.Register(cb); err != nil {
		return nil, err
	}
	return cb, nil
}
