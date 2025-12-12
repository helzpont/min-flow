package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Delegate represents a generic component in the flow system that requires
// initialization and cleanup. It serves as a base interface for more specific
// delegate types like Interceptor, Factory, and Pool.
type Delegate interface {
	Init() error
	Close() error
}

// Event represents a named occurrence in the stream processing lifecycle.
// Events use a hierarchical naming convention with colons as separators
// (e.g., "stream:start", "item:value").
type Event string

// Stream lifecycle events
const (
	StreamStart Event = "stream:start"
	StreamEnd   Event = "stream:end"
)

// Item-level events - fired for each item processed in a stream
const (
	// ItemReceived is fired when any item (value, error, or sentinel) is received
	ItemReceived Event = "item:received"
	// ItemEmitted is fired when any item is about to be emitted
	ItemEmitted Event = "item:emitted"
	// ValueReceived is fired when a successful value is received
	ValueReceived Event = "value:received"
	// ErrorOccurred is fired when an error result is received
	ErrorOccurred Event = "error:occurred"
	// SentinelReceived is fired when a sentinel is received
	SentinelReceived Event = "sentinel:received"
)

// EventPattern defines type-safe event matching.
type EventPattern interface {
	Matches(Event) bool
}

// ExactMatch matches an event exactly (no wildcards).
type ExactMatch Event

func (e ExactMatch) Matches(event Event) bool {
	return event == Event(e)
}

// AllEventsPattern matches any event.
type AllEventsPattern struct{}

func (p AllEventsPattern) Matches(Event) bool {
	return true
}

// PrefixMatch matches events starting with a prefix (e.g., "stream:" or "item:").
type PrefixMatch string

func (p PrefixMatch) Matches(event Event) bool {
	return strings.HasPrefix(string(event), string(p))
}

// SuffixMatch matches events ending with a suffix (e.g., ":start" or ":end").
type SuffixMatch string

func (s SuffixMatch) Matches(event Event) bool {
	return strings.HasSuffix(string(event), string(s))
}

// Pattern constructors for backward compatibility
var (
	AllEvents       EventPattern = AllEventsPattern{}
	AllStreamEvents EventPattern = PrefixMatch("stream:")
	AllItemEvents   EventPattern = PrefixMatch("item:")
	AllStartEvents  EventPattern = SuffixMatch(":start")
)

// Matches checks if an event matches a pattern string.
// Patterns can use "*" as a wildcard prefix or suffix.
// - "*" matches any event
// - "item:*" matches all item events
// - "*:start" matches all start events
// - "item:received" matches exactly
func (e Event) Matches(pattern string) bool {
	if pattern == "*" {
		return true
	}
	eventStr := string(e)
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(eventStr, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(eventStr, prefix)
	}
	return eventStr == pattern
}

// Interceptor represents a component that can intercept events during
// the flow processing. It extends the Delegate interface and adds a Do method
// to handle specific events with optional arguments.
type Interceptor interface {
	Delegate

	Events() []Event
	Do(context.Context, Event, ...any) error
}

// Factory represents a component responsible for creating instances
// of type T. It extends the Delegate interface and provides a New method
// to create new instances with optional arguments.
type Factory[T any] interface {
	Delegate

	New(...any) (T, error)
}

// Pool represents a component that manages a pool of reusable instances
// of type T. It extends the Delegate interface and provides Get and Put methods
// to acquire and release instances from the pool.
type Pool[T any] interface {
	Delegate

	Get() T
	Put(T)
}

// Config represents a configuration component that extends the Delegate
// interface and adds a Validate method to ensure the configuration is valid.
type Config interface {
	Delegate

	Validate() error
}

// Registry manages a collection of Delegate instances, allowing for
// registration and retrieval based on their type. It ensures thread-safe
// access to the registered delegates.
type Registry struct {
	mu    sync.RWMutex
	items map[string]Delegate
	order []string

	// Cached interceptors for fast lookup, built lazily on first access
	interceptorCache     []Interceptor
	interceptorCacheDone bool
}

func typeKey(d Delegate) string {
	return fmt.Sprintf("%T", d)
}

func (r *Registry) Register(d Delegate) error {
	key := typeKey(d)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[key]; exists {
		return fmt.Errorf("delegate with key %s already registered", key)
	}
	r.items[key] = d
	r.order = append(r.order, key)
	// Invalidate cache when new delegate is registered
	r.interceptorCacheDone = false
	r.interceptorCache = nil
	return nil
}

func (r *Registry) Get(id string) Delegate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.items[id]
}

// Interceptors returns all registered interceptors in registration order.
// The result is cached for repeated calls.
func (r *Registry) Interceptors() []Interceptor {
	r.mu.RLock()
	if r.interceptorCacheDone {
		result := r.interceptorCache
		r.mu.RUnlock()
		return result
	}
	r.mu.RUnlock()

	// Build cache under write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if r.interceptorCacheDone {
		return r.interceptorCache
	}

	interceptors := make([]Interceptor, 0, len(r.order))
	for _, key := range r.order {
		if i, ok := r.items[key].(Interceptor); ok {
			interceptors = append(interceptors, i)
		}
	}
	r.interceptorCache = interceptors
	r.interceptorCacheDone = true
	return interceptors
}

type registryKey struct{}

func WithRegistry(ctx context.Context) (context.Context, *Registry) {
	registry := &Registry{
		mu:    sync.RWMutex{},
		items: make(map[string]Delegate),
		order: make([]string, 0),
	}
	return context.WithValue(ctx, registryKey{}, registry), registry
}

// GetRegistry retrieves the Registry from the context, if present.
func GetRegistry(ctx context.Context) (*Registry, bool) {
	registry, ok := ctx.Value(registryKey{}).(*Registry)
	return registry, ok
}

// InvokeInterceptors invokes all interceptors matching the given event.
// Interceptors are invoked in registration order.
// If any interceptor returns an error, execution stops and the error is returned.
func InvokeInterceptors(ctx context.Context, event Event, args ...any) error {
	registry, ok := GetRegistry(ctx)
	if !ok {
		return nil
	}

	for _, interceptor := range registry.Interceptors() {
		for _, pattern := range interceptor.Events() {
			if event.Matches(string(pattern)) {
				if err := interceptor.Do(ctx, event, args...); err != nil {
					return err
				}
				break // Only invoke once per interceptor
			}
		}
	}
	return nil
}

// getDelegate is a generic helper that retrieves a delegate of type D from the registry.
// It handles the common pattern of context lookup, registry access, and type assertion.
func getDelegate[D Delegate](ctx context.Context) (D, bool) {
	var zero D
	registry, ok := GetRegistry(ctx)
	if !ok {
		return zero, false
	}
	delegate, ok := registry.Get(typeKey(zero)).(D)
	return delegate, ok
}

// GetInterceptor retrieves a registered Interceptor of type I from the context.
func GetInterceptor[I Interceptor](ctx context.Context) (I, bool) {
	return getDelegate[I](ctx)
}

// GetProvider retrieves a registered Factory of type P from the context.
func GetProvider[P Factory[T], T any](ctx context.Context) (P, bool) {
	return getDelegate[P](ctx)
}

// GetConfig retrieves a registered Config of type C from the context.
func GetConfig[C Config](ctx context.Context) (C, bool) {
	return getDelegate[C](ctx)
}

// GetPool retrieves a registered Pool of type P from the context.
func GetPool[P Pool[T], T any](ctx context.Context) (P, bool) {
	return getDelegate[P](ctx)
}
