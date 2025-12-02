package core

import (
	"context"
	"fmt"
	"sync"
)

// Delegate represents a generic component in the flow system that requires
// initialization and cleanup. It serves as a base interface for more specific
// delegate types like Interceptor, Factory, and Pool.
type Delegate interface {
	Init() error
	Close() error
}

type Event string

// TODO handle more complex event patterns
const (
	StreamStart Event = "stream:start"
	StreamEnd   Event = "stream:end"

	// TODO use regex for wildcards?
	AllEvents       = "*"
	AllStreamEvents = "stream:*"
	AllStartEvents  = "*:start"
)

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
}

func typeKey(d Delegate) string {
	return fmt.Sprintf("%T", d)
}

func (r *Registry) Register(d Delegate) error {
	key := typeKey(d)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[key]; !exists {
		return fmt.Errorf("delegate with key %s already registered", key)
	}
	r.items[key] = d
	r.order = append(r.order, key)
	return nil
}

func (r *Registry) Get(id string) Delegate {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.items[id]
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

func GetInterceptor[I Interceptor](ctx context.Context) (I, bool) {
	var zero I
	registry, ok := ctx.Value(registryKey{}).(*Registry)
	if !ok {
		return zero, false
	}
	interceptor, ok := registry.Get(typeKey(zero)).(I)
	return interceptor, ok
}

func GetProvider[P Factory[T], T any](ctx context.Context) (P, bool) {
	var zero P
	registry, ok := ctx.Value(registryKey{}).(*Registry)
	if !ok {
		return zero, false
	}
	provider, ok := registry.Get(typeKey(zero)).(P)
	return provider, ok
}

func GetConfig[C Config](ctx context.Context) (C, bool) {
	var zero C
	registry, ok := ctx.Value(registryKey{}).(*Registry)
	if !ok {
		return zero, false
	}
	config, ok := registry.Get(typeKey(zero)).(C)
	return config, ok
}
