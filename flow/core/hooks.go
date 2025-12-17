package core

import (
	"context"
)

// Hooks holds typed observation callbacks for a stream.
// All fields are optional - nil means no observation for that event.
// Hooks are invoked synchronously during stream processing, so they
// should be fast to avoid blocking the pipeline.
type Hooks[T any] struct {
	OnStart    func()      // Stream begins processing
	OnValue    func(T)     // Successful value received
	OnError    func(error) // Error received
	OnSentinel func(error) // Sentinel received
	OnComplete func()      // Stream finished (called even on context cancellation)
}

// hooksKey is unexported to prevent collisions with user context keys.
type hooksKey[T any] struct{}

// hooksContainer holds multiple hook sets for FIFO invocation.
type hooksContainer[T any] struct {
	hookSets []*Hooks[T]
}

// WithHooks attaches typed hooks to the context.
// Multiple calls to WithHooks compose in FIFO order - hooks from earlier
// calls are invoked before hooks from later calls.
//
// Example:
//
//	ctx := core.WithHooks(ctx, core.Hooks[int]{
//	    OnValue: func(v int) { log.Printf("Value: %d", v) },
//	})
func WithHooks[T any](ctx context.Context, hooks Hooks[T]) context.Context {
	if ctx == nil {
		panic("nil context")
	}

	existing := getHooksContainer[T](ctx)
	if existing == nil {
		// First hooks for this type
		return context.WithValue(ctx, hooksKey[T]{}, &hooksContainer[T]{
			hookSets: []*Hooks[T]{&hooks},
		})
	}

	// Append to existing hooks (FIFO order)
	newContainer := &hooksContainer[T]{
		hookSets: make([]*Hooks[T], len(existing.hookSets)+1),
	}
	copy(newContainer.hookSets, existing.hookSets)
	newContainer.hookSets[len(existing.hookSets)] = &hooks

	return context.WithValue(ctx, hooksKey[T]{}, newContainer)
}

// getHooksContainer retrieves the hooks container from context.
// Returns nil if no hooks are registered for type T.
func getHooksContainer[T any](ctx context.Context) *hooksContainer[T] {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.Value(hooksKey[T]{}).(*hooksContainer[T]); ok {
		return c
	}
	return nil
}

// hookInvoker wraps a hooks container for efficient invocation.
// It caches whether specific hook types exist to avoid repeated nil checks.
type hookInvoker[T any] struct {
	container   *hooksContainer[T]
	hasStart    bool
	hasValue    bool
	hasError    bool
	hasSentinel bool
	hasComplete bool
}

// newHookInvoker creates a hook invoker for the given context.
// This should be called once at the start of stream processing
// to cache hook existence flags.
func newHookInvoker[T any](ctx context.Context) *hookInvoker[T] {
	container := getHooksContainer[T](ctx)
	if container == nil {
		return &hookInvoker[T]{} // No hooks
	}

	invoker := &hookInvoker[T]{container: container}

	// Check which hook types exist
	for _, h := range container.hookSets {
		if h.OnStart != nil {
			invoker.hasStart = true
		}
		if h.OnValue != nil {
			invoker.hasValue = true
		}
		if h.OnError != nil {
			invoker.hasError = true
		}
		if h.OnSentinel != nil {
			invoker.hasSentinel = true
		}
		if h.OnComplete != nil {
			invoker.hasComplete = true
		}
	}

	return invoker
}

// invokeStart calls all OnStart hooks in FIFO order.
func (h *hookInvoker[T]) invokeStart() {
	if !h.hasStart || h.container == nil {
		return
	}
	for _, hooks := range h.container.hookSets {
		if hooks.OnStart != nil {
			hooks.OnStart()
		}
	}
}

// invokeValue calls all OnValue hooks in FIFO order.
func (h *hookInvoker[T]) invokeValue(value T) {
	if !h.hasValue || h.container == nil {
		return
	}
	for _, hooks := range h.container.hookSets {
		if hooks.OnValue != nil {
			hooks.OnValue(value)
		}
	}
}

// invokeError calls all OnError hooks in FIFO order.
func (h *hookInvoker[T]) invokeError(err error) {
	if !h.hasError || h.container == nil {
		return
	}
	for _, hooks := range h.container.hookSets {
		if hooks.OnError != nil {
			hooks.OnError(err)
		}
	}
}

// invokeSentinel calls all OnSentinel hooks in FIFO order.
func (h *hookInvoker[T]) invokeSentinel(err error) {
	if !h.hasSentinel || h.container == nil {
		return
	}
	for _, hooks := range h.container.hookSets {
		if hooks.OnSentinel != nil {
			hooks.OnSentinel(err)
		}
	}
}

// invokeComplete calls all OnComplete hooks in FIFO order.
func (h *hookInvoker[T]) invokeComplete() {
	if !h.hasComplete || h.container == nil {
		return
	}
	for _, hooks := range h.container.hookSets {
		if hooks.OnComplete != nil {
			hooks.OnComplete()
		}
	}
}

// hasAny returns true if there are any hooks registered.
func (h *hookInvoker[T]) hasAny() bool {
	return h.container != nil
}

// SafeHooks wraps Hooks[T] to recover from panics in hook functions.
// Use this when hooks are user-provided and panics should not crash the pipeline.
type SafeHooks[T any] struct {
	Hooks[T]
	panicHandler func(any) // Called when a hook panics
}

// NewSafeHooks creates SafeHooks from regular Hooks.
// If panicHandler is nil, panics are silently recovered.
func NewSafeHooks[T any](hooks Hooks[T], panicHandler func(any)) SafeHooks[T] {
	if panicHandler == nil {
		panicHandler = func(any) {} // Silent recovery
	}

	safe := SafeHooks[T]{
		panicHandler: panicHandler,
	}

	// Wrap each hook with panic recovery
	if hooks.OnStart != nil {
		originalStart := hooks.OnStart
		safe.OnStart = func() {
			defer func() {
				if r := recover(); r != nil {
					safe.panicHandler(r)
				}
			}()
			originalStart()
		}
	}

	if hooks.OnValue != nil {
		originalValue := hooks.OnValue
		safe.OnValue = func(v T) {
			defer func() {
				if r := recover(); r != nil {
					safe.panicHandler(r)
				}
			}()
			originalValue(v)
		}
	}

	if hooks.OnError != nil {
		originalError := hooks.OnError
		safe.OnError = func(err error) {
			defer func() {
				if r := recover(); r != nil {
					safe.panicHandler(r)
				}
			}()
			originalError(err)
		}
	}

	if hooks.OnSentinel != nil {
		originalSentinel := hooks.OnSentinel
		safe.OnSentinel = func(err error) {
			defer func() {
				if r := recover(); r != nil {
					safe.panicHandler(r)
				}
			}()
			originalSentinel(err)
		}
	}

	if hooks.OnComplete != nil {
		originalComplete := hooks.OnComplete
		safe.OnComplete = func() {
			defer func() {
				if r := recover(); r != nil {
					safe.panicHandler(r)
				}
			}()
			originalComplete()
		}
	}

	return safe
}

// WithSafeHooks is a convenience function that wraps hooks with panic recovery
// before attaching them to the context.
func WithSafeHooks[T any](ctx context.Context, hooks Hooks[T], panicHandler func(any)) context.Context {
	safe := NewSafeHooks(hooks, panicHandler)
	return WithHooks(ctx, safe.Hooks)
}
