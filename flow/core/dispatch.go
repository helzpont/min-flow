package core

import "context"

// interceptorDispatch provides efficient interceptor invocation for stream processing.
// It caches the registry lookup and interceptor list, and provides specialized
// invoke methods to avoid variadic allocation overhead.
type interceptorDispatch struct {
	interceptors []Interceptor
	hasAny       bool
}

// newInterceptorDispatch creates a dispatch helper from the context.
// Returns a dispatcher with hasAny=false if no registry or no interceptors.
func newInterceptorDispatch(ctx context.Context) interceptorDispatch {
	registry, ok := GetRegistry(ctx)
	if !ok {
		return interceptorDispatch{}
	}
	interceptors := registry.Interceptors()
	return interceptorDispatch{
		interceptors: interceptors,
		hasAny:       len(interceptors) > 0,
	}
}

// invokeNoArg handles events with no arguments (StreamStart, StreamEnd).
func (d *interceptorDispatch) invokeNoArg(ctx context.Context, event Event) {
	if !d.hasAny {
		return
	}
	for _, interceptor := range d.interceptors {
		for _, pattern := range interceptor.Events() {
			if event.Matches(string(pattern)) {
				_ = interceptor.Do(ctx, event)
				break
			}
		}
	}
}

// invokeOneArg handles events with one argument (most item events).
func (d *interceptorDispatch) invokeOneArg(ctx context.Context, event Event, arg any) {
	if !d.hasAny {
		return
	}
	for _, interceptor := range d.interceptors {
		for _, pattern := range interceptor.Events() {
			if event.Matches(string(pattern)) {
				_ = interceptor.Do(ctx, event, arg)
				break
			}
		}
	}
}

// invokeResult fires the appropriate event for a Result (ValueReceived, ErrorOccurred, or SentinelReceived).
// This does NOT fire ItemReceived - that should be fired separately for input items.
func (d *interceptorDispatch) invokeResult(ctx context.Context, res Result[any]) {
	if !d.hasAny {
		return
	}
	if res.IsValue() {
		d.invokeOneArg(ctx, ValueReceived, res.Value())
	} else if res.IsSentinel() {
		d.invokeOneArg(ctx, SentinelReceived, res.Error())
	} else {
		d.invokeOneArg(ctx, ErrorOccurred, res.Error())
	}
}

// toAnyResult converts a Result[T] to Result[any] for use with interceptors.
func toAnyResult[T any](res Result[T]) Result[any] {
	return NewResult(any(res.Value()), res.Error(), res.IsSentinel())
}
