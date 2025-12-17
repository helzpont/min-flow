package observe

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lguimbarda/min-flow/flow/core"
)

// This file provides convenience functions for creating typed hooks-based observers.
// The hooks system is type-parameterized, so observers must be registered with
// the specific type they want to observe.
//
// Usage pattern:
//
//	// Create typed hooks for int streams
//	ctx := WithValueHook(ctx, func(v int) { fmt.Println("Value:", v) })
//	ctx = WithErrorHook(ctx, func(err error) { log.Error(err) })
//
//	// Stream processing automatically invokes registered hooks\n//\tresult := someMapper.Apply(stream)\n\n// WithValueHook attaches a value observation hook for type T to the context.
//
// The callback fires for each successful value emitted.
func WithValueHook[T any](ctx context.Context, callback func(T)) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnValue: callback,
	})
}

// WithErrorHook attaches an error observation hook for type T to the context.
// The callback fires for each error encountered.
func WithErrorHook[T any](ctx context.Context, callback func(error)) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnError: callback,
	})
}

// WithStartHook attaches a stream start hook for type T to the context.
// The callback fires when the stream starts processing.
func WithStartHook[T any](ctx context.Context, callback func()) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnStart: callback,
	})
}

// WithCompleteHook attaches a stream completion hook for type T to the context.
// The callback fires when the stream completes.
func WithCompleteHook[T any](ctx context.Context, callback func()) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnComplete: callback,
	})
}

// WithSentinelHook attaches a sentinel observation hook for type T to the context.
// The callback fires for each sentinel value encountered.
func WithSentinelHook[T any](ctx context.Context, callback func(error)) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnSentinel: callback,
	})
}

// Counter provides thread-safe counting of values and errors.
type Counter struct {
	values atomic.Int64
	errors atomic.Int64
}

// Values returns the count of values processed.
func (c *Counter) Values() int64 { return c.values.Load() }

// Errors returns the count of errors encountered.
func (c *Counter) Errors() int64 { return c.errors.Load() }

// Total returns the total count of values and errors.
func (c *Counter) Total() int64 { return c.values.Load() + c.errors.Load() }

// WithCounter attaches counting hooks for type T and returns the counter for querying.
func WithCounter[T any](ctx context.Context) (context.Context, *Counter) {
	counter := &Counter{}
	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnValue: func(T) { counter.values.Add(1) },
		OnError: func(error) { counter.errors.Add(1) },
	})
	return ctx, counter
}

// ValueCounter counts only values.
type ValueCounter struct {
	count atomic.Int64
}

// Count returns the current count.
func (c *ValueCounter) Count() int64 { return c.count.Load() }

// WithValueCounter attaches a value counting hook for type T and returns the counter.
func WithValueCounter[T any](ctx context.Context) (context.Context, *ValueCounter) {
	counter := &ValueCounter{}
	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnValue: func(T) { counter.count.Add(1) },
	})
	return ctx, counter
}

// ErrorCollector collects all errors encountered in streams.
type ErrorCollector struct {
	mu     sync.Mutex
	errors []error
}

// Errors returns a copy of all collected errors.
func (c *ErrorCollector) Errors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]error, len(c.errors))
	copy(result, c.errors)
	return result
}

// HasErrors returns true if any errors were collected.
func (c *ErrorCollector) HasErrors() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.errors) > 0
}

// Count returns the number of collected errors.
func (c *ErrorCollector) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.errors)
}

// WithErrorCollector attaches an error collecting hook for type T and returns the collector.
func WithErrorCollector[T any](ctx context.Context) (context.Context, *ErrorCollector) {
	collector := &ErrorCollector{}
	ctx = core.WithHooks(ctx, core.Hooks[T]{
		OnError: func(err error) {
			collector.mu.Lock()
			collector.errors = append(collector.errors, err)
			collector.mu.Unlock()
		},
	})
	return ctx, collector
}

// Logger is a function type for logging messages.
type Logger func(format string, args ...any)

// WithLogging attaches logging hooks for type T to the context.
func WithLogging[T any](ctx context.Context, logger Logger) context.Context {
	return core.WithHooks(ctx, core.Hooks[T]{
		OnStart: func() {
			logger("stream started")
		},
		OnValue: func(v T) {
			logger("value: %v", v)
		},
		OnError: func(err error) {
			logger("error: %v", err)
		},
		OnSentinel: func(err error) {
			logger("sentinel: %v", err)
		},
		OnComplete: func() {
			logger("stream completed")
		},
	})
}
