package flowerrors

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lguimbarda/min-flow/flow/core"
)

// This file provides hooks-based error observation utilities.
// These hooks observe errors for logging, counting, and collection purposes.
// They do not modify the data flow - for error transformation (retry, fallback, catch),
// use the transformer-based operators in error.go and resilience.go.

// ErrorCounter counts errors that match a predicate.
type ErrorCounter struct {
	predicate func(error) bool
	count     atomic.Int64
}

// Count returns the number of errors counted.
func (c *ErrorCounter) Count() int64 {
	return c.count.Load()
}

// WithErrorCounter attaches an error counting hook for type T and returns the counter.
// If predicate is nil, all errors are counted.
func WithErrorCounter[T any](ctx context.Context, predicate func(error) bool) (context.Context, *ErrorCounter) {
	if predicate == nil {
		predicate = func(error) bool { return true }
	}
	counter := &ErrorCounter{predicate: predicate}
	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnError: func(err error) {
			if counter.predicate(err) {
				counter.count.Add(1)
			}
		},
	})
	return ctx, counter
}

// ErrorCollector collects errors for later inspection.
type ErrorCollector struct {
	mu        sync.Mutex
	errors    []error
	predicate func(error) bool
	maxErrors int // 0 = unlimited
}

// ErrorCollectorOption configures an ErrorCollector.
type ErrorCollectorOption func(*ErrorCollector)

// WithPredicate filters which errors to collect.
func WithPredicate(predicate func(error) bool) ErrorCollectorOption {
	return func(c *ErrorCollector) {
		c.predicate = predicate
	}
}

// WithMaxErrors limits the number of errors to collect.
func WithMaxErrors(max int) ErrorCollectorOption {
	return func(c *ErrorCollector) {
		c.maxErrors = max
	}
}

// Errors returns a copy of all collected errors.
func (c *ErrorCollector) Errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]error, len(c.errors))
	copy(result, c.errors)
	return result
}

// Count returns the number of collected errors.
func (c *ErrorCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.errors)
}

// WithErrorCollector attaches an error collecting hook for type T and returns the collector.
func WithErrorCollector[T any](ctx context.Context, opts ...ErrorCollectorOption) (context.Context, *ErrorCollector) {
	collector := &ErrorCollector{
		predicate: func(error) bool { return true },
	}
	for _, opt := range opts {
		opt(collector)
	}

	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnError: func(err error) {
			if !collector.predicate(err) {
				return
			}
			collector.mu.Lock()
			defer collector.mu.Unlock()
			if collector.maxErrors > 0 && len(collector.errors) >= collector.maxErrors {
				return
			}
			collector.errors = append(collector.errors, err)
		},
	})
	return ctx, collector
}

// OnErrorDo attaches an error handler hook for type T.
func OnErrorDo[T any](ctx context.Context, handler func(error)) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnError: handler,
	})
}

// CircuitBreakerMonitor monitors error rates and trips when a threshold is reached.
// For a full circuit breaker implementation with retry logic, see CircuitBreaker in resilience.go.
type CircuitBreakerMonitor struct {
	threshold       int
	failureCount    atomic.Int64
	isOpen          atomic.Bool
	onThresholdHit  func()
}

// IsOpen returns true if the circuit breaker has tripped.
func (cb *CircuitBreakerMonitor) IsOpen() bool {
	return cb.isOpen.Load()
}

// FailureCount returns the current failure count.
func (cb *CircuitBreakerMonitor) FailureCount() int64 {
	return cb.failureCount.Load()
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreakerMonitor) Reset() {
	cb.failureCount.Store(0)
	cb.isOpen.Store(false)
}

// WithCircuitBreakerMonitor attaches a circuit breaker monitoring hook for type T and returns the monitor.
// This is a lightweight monitor for error rate tracking. For a full circuit breaker
// with retry logic and half-open state, see WithCircuitBreaker in resilience.go.
func WithCircuitBreakerMonitor[T any](ctx context.Context, threshold int, onThresholdHit func()) (context.Context, *CircuitBreakerMonitor) {
	cb := &CircuitBreakerMonitor{
		threshold:      threshold,
		onThresholdHit: onThresholdHit,
	}

	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnError: func(err error) {
			if cb.isOpen.Load() {
				return
			}
			count := cb.failureCount.Add(1)
			if int(count) >= cb.threshold {
				cb.isOpen.Store(true)
				if cb.onThresholdHit != nil {
					cb.onThresholdHit()
				}
			}
		},
		OnValue: func(T) {
			// Reset on success
			cb.failureCount.Store(0)
		},
	})
	return ctx, cb
}
