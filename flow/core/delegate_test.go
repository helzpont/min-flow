package core

import (
	"context"
	"errors"
	"testing"
)

// Mock delegates for testing

type mockInterceptor struct {
	events []Event
}

func (m *mockInterceptor) Init() error     { return nil }
func (m *mockInterceptor) Close() error    { return nil }
func (m *mockInterceptor) Events() []Event { return m.events }
func (m *mockInterceptor) Do(ctx context.Context, event Event, args ...any) error {
	return nil
}

type mockFactory[T any] struct {
	value T
}

func (m *mockFactory[T]) Init() error                { return nil }
func (m *mockFactory[T]) Close() error               { return nil }
func (m *mockFactory[T]) New(args ...any) (T, error) { return m.value, nil }

type mockPool[T any] struct {
	value T
}

func (m *mockPool[T]) Init() error  { return nil }
func (m *mockPool[T]) Close() error { return nil }
func (m *mockPool[T]) Get() T       { return m.value }
func (m *mockPool[T]) Put(v T)      {}

type mockConfig struct {
	valid bool
}

func (m *mockConfig) Init() error  { return nil }
func (m *mockConfig) Close() error { return nil }
func (m *mockConfig) Validate() error {
	if !m.valid {
		return errors.New("invalid config")
	}
	return nil
}

func TestRegistry_Register(t *testing.T) {
	_, registry := WithRegistry(context.Background())

	interceptor := &mockInterceptor{events: []Event{StreamStart}}
	err := registry.Register(interceptor)
	if err != nil {
		t.Errorf("Register() error = %v", err)
	}

	// Duplicate registration should fail
	err = registry.Register(interceptor)
	if err == nil {
		t.Error("Register() should fail for duplicate registration")
	}
}

func TestRegistry_Get(t *testing.T) {
	_, registry := WithRegistry(context.Background())

	interceptor := &mockInterceptor{events: []Event{StreamStart}}
	_ = registry.Register(interceptor)

	key := typeKey(interceptor)
	got := registry.Get(key)
	if got != interceptor {
		t.Errorf("Get() = %v, want %v", got, interceptor)
	}

	// Non-existent key
	got = registry.Get("nonexistent")
	if got != nil {
		t.Errorf("Get() for nonexistent key = %v, want nil", got)
	}
}

func TestGetInterceptor(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		ctx, registry := WithRegistry(context.Background())
		interceptor := &mockInterceptor{events: []Event{StreamStart}}
		_ = registry.Register(interceptor)

		got, ok := GetInterceptor[*mockInterceptor](ctx)
		if !ok {
			t.Error("GetInterceptor() returned false, want true")
		}
		if got != interceptor {
			t.Errorf("GetInterceptor() = %v, want %v", got, interceptor)
		}
	})

	t.Run("without registry", func(t *testing.T) {
		ctx := context.Background()
		_, ok := GetInterceptor[*mockInterceptor](ctx)
		if ok {
			t.Error("GetInterceptor() returned true without registry, want false")
		}
	})

	t.Run("not registered", func(t *testing.T) {
		ctx, _ := WithRegistry(context.Background())
		_, ok := GetInterceptor[*mockInterceptor](ctx)
		if ok {
			t.Error("GetInterceptor() returned true for unregistered, want false")
		}
	})
}

func TestGetProvider(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		ctx, registry := WithRegistry(context.Background())
		factory := &mockFactory[string]{value: "test"}
		_ = registry.Register(factory)

		got, ok := GetProvider[*mockFactory[string], string](ctx)
		if !ok {
			t.Error("GetProvider() returned false, want true")
		}
		if got != factory {
			t.Errorf("GetProvider() = %v, want %v", got, factory)
		}
	})

	t.Run("without registry", func(t *testing.T) {
		ctx := context.Background()
		_, ok := GetProvider[*mockFactory[string], string](ctx)
		if ok {
			t.Error("GetProvider() returned true without registry, want false")
		}
	})
}

func TestGetConfig(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		ctx, registry := WithRegistry(context.Background())
		config := &mockConfig{valid: true}
		_ = registry.Register(config)

		got, ok := GetConfig[*mockConfig](ctx)
		if !ok {
			t.Error("GetConfig() returned false, want true")
		}
		if got != config {
			t.Errorf("GetConfig() = %v, want %v", got, config)
		}
	})

	t.Run("without registry", func(t *testing.T) {
		ctx := context.Background()
		_, ok := GetConfig[*mockConfig](ctx)
		if ok {
			t.Error("GetConfig() returned true without registry, want false")
		}
	})
}

func TestGetPool(t *testing.T) {
	t.Run("with registry", func(t *testing.T) {
		ctx, registry := WithRegistry(context.Background())
		pool := &mockPool[int]{value: 42}
		_ = registry.Register(pool)

		got, ok := GetPool[*mockPool[int], int](ctx)
		if !ok {
			t.Error("GetPool() returned false, want true")
		}
		if got != pool {
			t.Errorf("GetPool() = %v, want %v", got, pool)
		}
	})

	t.Run("without registry", func(t *testing.T) {
		ctx := context.Background()
		_, ok := GetPool[*mockPool[int], int](ctx)
		if ok {
			t.Error("GetPool() returned true without registry, want false")
		}
	})
}

func TestWithRegistry(t *testing.T) {
	ctx := context.Background()
	newCtx, registry := WithRegistry(ctx)

	if registry == nil {
		t.Error("WithRegistry() returned nil registry")
	}

	if newCtx == ctx {
		t.Error("WithRegistry() should return new context")
	}

	// Verify we can retrieve the registry from context
	cfg := &mockConfig{valid: true}
	_ = registry.Register(cfg)

	got, ok := GetConfig[*mockConfig](newCtx)
	if !ok || got != cfg {
		t.Error("Registry should be accessible from returned context")
	}
}

func TestTypeKey(t *testing.T) {
	interceptor := &mockInterceptor{}
	factory := &mockFactory[string]{}

	key1 := typeKey(interceptor)
	key2 := typeKey(factory)

	if key1 == key2 {
		t.Error("typeKey() should return different keys for different types")
	}

	// Same type should have same key
	interceptor2 := &mockInterceptor{}
	if typeKey(interceptor) != typeKey(interceptor2) {
		t.Error("typeKey() should return same key for same type")
	}
}
