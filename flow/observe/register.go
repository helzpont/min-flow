package observe

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/lguimbarda/min-flow/flow/core"
)

// This file provides convenience functions for registering interceptor-based observers.
// These functions create interceptors and register them with a Registry, returning
// the context with the registry attached. Since transformers now auto-invoke interceptors,
// these registered observers will automatically receive events without needing explicit
// pipeline stages.
//
// Usage pattern:
//
//	ctx, registry := core.WithRegistry(context.Background())
//	observe.OnValue(registry, func(v any) { fmt.Println("Value:", v) })
//	observe.OnError(registry, func(err error) { log.Error(err) })
//	observe.OnComplete(registry, func() { fmt.Println("Done!") })
//
//	// Stream processing automatically invokes registered observers
//	result := someMapper.Apply(ctx, stream)

// compositeInterceptor accumulates multiple callbacks and invokes them for events.
// It supports adding callbacks after creation, unlike CallbackInterceptor.
type compositeInterceptor struct {
	mu         sync.RWMutex
	onValue    []func(any)
	onError    []func(error)
	onStart    []func()
	onComplete []func()
}

func (c *compositeInterceptor) Init() error  { return nil }
func (c *compositeInterceptor) Close() error { return nil }

func (c *compositeInterceptor) Events() []core.Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var events []core.Event
	if len(c.onStart) > 0 {
		events = append(events, core.StreamStart)
	}
	if len(c.onComplete) > 0 {
		events = append(events, core.StreamEnd)
	}
	if len(c.onValue) > 0 {
		events = append(events, core.ValueReceived)
	}
	if len(c.onError) > 0 {
		events = append(events, core.ErrorOccurred)
	}
	return events
}

func (c *compositeInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	switch event {
	case core.StreamStart:
		for _, fn := range c.onStart {
			fn()
		}
	case core.StreamEnd:
		for _, fn := range c.onComplete {
			fn()
		}
	case core.ValueReceived:
		var value any
		if len(args) > 0 {
			value = args[0]
		}
		for _, fn := range c.onValue {
			fn(value)
		}
	case core.ErrorOccurred:
		if len(args) > 0 {
			if err, ok := args[0].(error); ok {
				for _, fn := range c.onError {
					fn(err)
				}
			}
		}
	}
	return nil
}

// getOrCreateComposite retrieves the shared compositeInterceptor from the registry,
// or creates and registers one if it doesn't exist.
func getOrCreateComposite(registry *core.Registry) (*compositeInterceptor, error) {
	// Check if already registered
	if existing := registry.Get("*observe.compositeInterceptor"); existing != nil {
		return existing.(*compositeInterceptor), nil
	}

	// Create and register new composite
	composite := &compositeInterceptor{}
	if err := registry.Register(composite); err != nil {
		// Another goroutine may have registered it - try to get it
		if existing := registry.Get("*observe.compositeInterceptor"); existing != nil {
			return existing.(*compositeInterceptor), nil
		}
		return nil, err
	}
	return composite, nil
}

// OnValue registers a callback that fires for each successful value in any stream.
// The callback receives the value as `any` since interceptors are not generic.
// Multiple callbacks can be registered and all will be invoked.
func OnValue(registry *core.Registry, callback func(any)) error {
	composite, err := getOrCreateComposite(registry)
	if err != nil {
		return err
	}
	composite.mu.Lock()
	composite.onValue = append(composite.onValue, callback)
	composite.mu.Unlock()
	return nil
}

// OnError registers a callback that fires for each error in any stream.
// Multiple callbacks can be registered and all will be invoked.
func OnError(registry *core.Registry, callback func(error)) error {
	composite, err := getOrCreateComposite(registry)
	if err != nil {
		return err
	}
	composite.mu.Lock()
	composite.onError = append(composite.onError, callback)
	composite.mu.Unlock()
	return nil
}

// OnStart registers a callback that fires when any stream starts processing.
// Multiple callbacks can be registered and all will be invoked.
func OnStart(registry *core.Registry, callback func()) error {
	composite, err := getOrCreateComposite(registry)
	if err != nil {
		return err
	}
	composite.mu.Lock()
	composite.onStart = append(composite.onStart, callback)
	composite.mu.Unlock()
	return nil
}

// OnComplete registers a callback that fires when any stream completes.
// Multiple callbacks can be registered and all will be invoked.
func OnComplete(registry *core.Registry, callback func()) error {
	composite, err := getOrCreateComposite(registry)
	if err != nil {
		return err
	}
	composite.mu.Lock()
	composite.onComplete = append(composite.onComplete, callback)
	composite.mu.Unlock()
	return nil
}

// WithMetrics registers a MetricsInterceptor and returns it for querying.
// The onComplete callback is called when any stream ends.
func WithMetrics(registry *core.Registry, onComplete func(StreamMetrics)) (*MetricsInterceptor, error) {
	interceptor := NewMetricsInterceptor(onComplete)
	if err := registry.Register(interceptor); err != nil {
		return nil, err
	}
	return interceptor, nil
}

// WithLiveMetrics registers a LiveMetricsInterceptor and returns the LiveMetrics for querying.
func WithLiveMetrics(registry *core.Registry) (*LiveMetrics, error) {
	metrics := &LiveMetrics{}
	interceptor := NewLiveMetricsInterceptor(metrics)
	if err := registry.Register(interceptor); err != nil {
		return nil, err
	}
	return metrics, nil
}

// WithCounter registers a CounterInterceptor and returns it for querying.
func WithCounter(registry *core.Registry) (*CounterInterceptor, error) {
	counter := NewCounterInterceptor()
	if err := registry.Register(counter); err != nil {
		return nil, err
	}
	return counter, nil
}

// WithLogging registers a LogInterceptor with the given logger.
// If no events are specified, all events are logged.
func WithLogging(registry *core.Registry, logger func(format string, args ...any), events ...core.Event) error {
	interceptor := NewLogInterceptor(logger, events...)
	return registry.Register(interceptor)
}

// ValueCounter is a simple atomic counter for values that can be registered as an interceptor.
type ValueCounter struct {
	count atomic.Int64
}

// Count returns the current count.
func (c *ValueCounter) Count() int64 {
	return c.count.Load()
}

func (c *ValueCounter) Init() error  { return nil }
func (c *ValueCounter) Close() error { return nil }

func (c *ValueCounter) Events() []core.Event {
	return []core.Event{core.ValueReceived}
}

func (c *ValueCounter) Do(ctx context.Context, event core.Event, args ...any) error {
	c.count.Add(1)
	return nil
}

// WithValueCounter registers a simple value counter and returns it for querying.
func WithValueCounter(registry *core.Registry) (*ValueCounter, error) {
	counter := &ValueCounter{}
	if err := registry.Register(counter); err != nil {
		return nil, err
	}
	return counter, nil
}

// ErrorCollector collects all errors encountered in streams.
type ErrorCollector struct {
	errors []error
}

// Errors returns all collected errors.
func (c *ErrorCollector) Errors() []error {
	return c.errors
}

// HasErrors returns true if any errors were collected.
func (c *ErrorCollector) HasErrors() bool {
	return len(c.errors) > 0
}

func (c *ErrorCollector) Init() error  { return nil }
func (c *ErrorCollector) Close() error { return nil }

func (c *ErrorCollector) Events() []core.Event {
	return []core.Event{core.ErrorOccurred}
}

func (c *ErrorCollector) Do(ctx context.Context, event core.Event, args ...any) error {
	if len(args) > 0 {
		if err, ok := args[0].(error); ok {
			c.errors = append(c.errors, err)
		}
	}
	return nil
}

// WithErrorCollector registers an error collector and returns it for querying.
func WithErrorCollector(registry *core.Registry) (*ErrorCollector, error) {
	collector := &ErrorCollector{}
	if err := registry.Register(collector); err != nil {
		return nil, err
	}
	return collector, nil
}
