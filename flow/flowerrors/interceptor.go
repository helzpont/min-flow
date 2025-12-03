package flowerrors

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lguimbarda/min-flow/flow/core"
)

// This file contains interceptor-based error handlers that can be registered
// with a Registry and invoked automatically when streams are processed through
// core.Intercept(). These interceptors are for observation/side effects only
// (e.g., logging, counting, collecting errors) and do not modify the data flow.
//
// For error handling that transforms the stream (retry, fallback, catch, etc.),
// use the transformer-based operators in error.go and resilience.go instead.

// ErrorHandlerInterceptor handles errors through callbacks.
// It can log, count, or process errors as they occur in the stream.
type ErrorHandlerInterceptor struct {
	handler func(error)
}

// NewErrorHandlerInterceptor creates an interceptor that calls the handler for each error.
func NewErrorHandlerInterceptor(handler func(error)) *ErrorHandlerInterceptor {
	return &ErrorHandlerInterceptor{handler: handler}
}

func (e *ErrorHandlerInterceptor) Init() error  { return nil }
func (e *ErrorHandlerInterceptor) Close() error { return nil }

func (e *ErrorHandlerInterceptor) Events() []core.Event {
	return []core.Event{core.ErrorOccurred}
}

func (e *ErrorHandlerInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	if event == core.ErrorOccurred && len(args) > 0 {
		if err, ok := args[0].(error); ok && e.handler != nil {
			e.handler(err)
		}
	}
	return nil
}

// ErrorCounterInterceptor counts errors that match a predicate.
type ErrorCounterInterceptor struct {
	predicate func(error) bool
	count     atomic.Int64
}

// NewErrorCounterInterceptor creates an interceptor that counts errors.
// If predicate is nil, all errors are counted.
func NewErrorCounterInterceptor(predicate func(error) bool) *ErrorCounterInterceptor {
	if predicate == nil {
		predicate = func(error) bool { return true }
	}
	return &ErrorCounterInterceptor{predicate: predicate}
}

func (e *ErrorCounterInterceptor) Init() error  { return nil }
func (e *ErrorCounterInterceptor) Close() error { return nil }

func (e *ErrorCounterInterceptor) Events() []core.Event {
	return []core.Event{core.ErrorOccurred}
}

func (e *ErrorCounterInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	if event == core.ErrorOccurred && len(args) > 0 {
		if err, ok := args[0].(error); ok && e.predicate(err) {
			e.count.Add(1)
		}
	}
	return nil
}

// Count returns the number of errors counted.
func (e *ErrorCounterInterceptor) Count() int64 {
	return e.count.Load()
}

// ErrorCollectorInterceptor collects errors for later inspection.
type ErrorCollectorInterceptor struct {
	mu        sync.Mutex
	errors    []error
	predicate func(error) bool
	maxErrors int // 0 = unlimited
}

// ErrorCollectorOption configures an ErrorCollectorInterceptor.
type ErrorCollectorOption func(*ErrorCollectorInterceptor)

// WithErrorPredicate filters which errors to collect.
func WithErrorPredicate(predicate func(error) bool) ErrorCollectorOption {
	return func(e *ErrorCollectorInterceptor) {
		e.predicate = predicate
	}
}

// WithMaxErrors limits the number of errors to collect.
func WithMaxErrors(max int) ErrorCollectorOption {
	return func(e *ErrorCollectorInterceptor) {
		e.maxErrors = max
	}
}

// NewErrorCollectorInterceptor creates an interceptor that collects errors.
func NewErrorCollectorInterceptor(opts ...ErrorCollectorOption) *ErrorCollectorInterceptor {
	e := &ErrorCollectorInterceptor{
		predicate: func(error) bool { return true },
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *ErrorCollectorInterceptor) Init() error  { return nil }
func (e *ErrorCollectorInterceptor) Close() error { return nil }

func (e *ErrorCollectorInterceptor) Events() []core.Event {
	return []core.Event{core.ErrorOccurred}
}

func (e *ErrorCollectorInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	if event == core.ErrorOccurred && len(args) > 0 {
		if err, ok := args[0].(error); ok && e.predicate(err) {
			e.mu.Lock()
			defer e.mu.Unlock()
			if e.maxErrors <= 0 || len(e.errors) < e.maxErrors {
				e.errors = append(e.errors, err)
			}
		}
	}
	return nil
}

// Errors returns a copy of the collected errors.
func (e *ErrorCollectorInterceptor) Errors() []error {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make([]error, len(e.errors))
	copy(result, e.errors)
	return result
}

// Count returns the number of collected errors.
func (e *ErrorCollectorInterceptor) Count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.errors)
}

// Clear resets the collected errors.
func (e *ErrorCollectorInterceptor) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.errors = nil
}

// CircuitBreakerInterceptor monitors error rates and can trip a circuit breaker.
// When the error threshold is reached, it returns an error to stop processing.
type CircuitBreakerInterceptor struct {
	mu             sync.Mutex
	failureCount   int
	threshold      int
	onThresholdHit func()
	thresholdHit   bool
}

// NewCircuitBreakerInterceptor creates a circuit breaker that trips after threshold errors.
func NewCircuitBreakerInterceptor(threshold int, onThresholdHit func()) *CircuitBreakerInterceptor {
	return &CircuitBreakerInterceptor{
		threshold:      threshold,
		onThresholdHit: onThresholdHit,
	}
}

func (c *CircuitBreakerInterceptor) Init() error  { return nil }
func (c *CircuitBreakerInterceptor) Close() error { return nil }

func (c *CircuitBreakerInterceptor) Events() []core.Event {
	return []core.Event{core.ErrorOccurred, core.ValueReceived}
}

func (c *CircuitBreakerInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch event {
	case core.ErrorOccurred:
		c.failureCount++
		if c.failureCount >= c.threshold && !c.thresholdHit {
			c.thresholdHit = true
			if c.onThresholdHit != nil {
				c.onThresholdHit()
			}
			return ErrCircuitOpen
		}
	case core.ValueReceived:
		// Reset on success (simple reset policy)
		c.failureCount = 0
	}

	return nil
}

// IsOpen returns true if the circuit breaker has tripped.
func (c *CircuitBreakerInterceptor) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.thresholdHit
}

// Reset resets the circuit breaker.
func (c *CircuitBreakerInterceptor) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failureCount = 0
	c.thresholdHit = false
}

// FailureCount returns the current failure count.
func (c *CircuitBreakerInterceptor) FailureCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failureCount
}
