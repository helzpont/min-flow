package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// Helper function to create a stream from a slice for testing
func fromSlice[T any](items []T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T], len(items))
		go func() {
			defer close(out)
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(item):
				}
			}
		}()
		return out
	})
}

func TestWithHooks(t *testing.T) {
	t.Run("basic hooks invocation", func(t *testing.T) {
		ctx := context.Background()
		var started, completed atomic.Bool
		var valueCount atomic.Int64

		ctx = WithHooks(ctx, Hooks[int]{
			OnStart: func() {
				started.Store(true)
			},
			OnValue: func(v int) {
				valueCount.Add(1)
			},
			OnComplete: func() {
				completed.Store(true)
			},
		})

		mapper := Map(func(v int) (int, error) {
			return v * 2, nil
		})

		stream := fromSlice([]int{1, 2, 3})
		result := mapper.Apply(stream)
		values, err := Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}
		if !started.Load() {
			t.Error("OnStart was not called")
		}
		if !completed.Load() {
			t.Error("OnComplete was not called")
		}
		if valueCount.Load() != 3 {
			t.Errorf("expected 3 OnValue calls, got %d", valueCount.Load())
		}
	})

	t.Run("error hooks invocation", func(t *testing.T) {
		ctx := context.Background()
		var errorCount atomic.Int64
		var lastError error

		ctx = WithHooks(ctx, Hooks[int]{
			OnError: func(err error) {
				errorCount.Add(1)
				lastError = err
			},
		})

		testErr := errors.New("test error")
		mapper := Map(func(v int) (int, error) {
			if v == 2 {
				return 0, testErr
			}
			return v, nil
		})

		stream := fromSlice([]int{1, 2, 3})
		result := mapper.Apply(stream)
		_, err := Slice(ctx, result)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if errorCount.Load() != 1 {
			t.Errorf("expected 1 OnError call, got %d", errorCount.Load())
		}
		if !errors.Is(lastError, testErr) {
			t.Errorf("expected test error, got %v", lastError)
		}
	})

	t.Run("FIFO hook composition", func(t *testing.T) {
		ctx := context.Background()
		var order []string

		ctx = WithHooks(ctx, Hooks[int]{
			OnValue: func(v int) {
				order = append(order, "first")
			},
		})

		ctx = WithHooks(ctx, Hooks[int]{
			OnValue: func(v int) {
				order = append(order, "second")
			},
		})

		ctx = WithHooks(ctx, Hooks[int]{
			OnValue: func(v int) {
				order = append(order, "third")
			},
		})

		mapper := Map(func(v int) (int, error) { return v, nil })
		stream := fromSlice([]int{1})
		result := mapper.Apply(stream)
		_, _ = Slice(ctx, result)

		if len(order) != 3 {
			t.Fatalf("expected 3 hooks to be called, got %d", len(order))
		}
		if order[0] != "first" || order[1] != "second" || order[2] != "third" {
			t.Errorf("wrong order: %v", order)
		}
	})

	t.Run("nil hooks don't cause panic", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithHooks(ctx, Hooks[int]{
			// All hooks are nil
		})

		mapper := Map(func(v int) (int, error) { return v, nil })
		stream := fromSlice([]int{1, 2, 3})
		result := mapper.Apply(stream)
		values, err := Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}
	})

	t.Run("hooks work with FlatMapper", func(t *testing.T) {
		ctx := context.Background()
		var valueCount atomic.Int64

		ctx = WithHooks(ctx, Hooks[int]{
			OnValue: func(v int) {
				valueCount.Add(1)
			},
		})

		flatMapper := FlatMap(func(v int) ([]int, error) {
			return []int{v, v * 2}, nil
		})

		stream := fromSlice([]int{1, 2, 3})
		result := flatMapper.Apply(stream)
		values, err := Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 6 {
			t.Fatalf("expected 6 values, got %d", len(values))
		}
		if valueCount.Load() != 6 {
			t.Errorf("expected 6 OnValue calls, got %d", valueCount.Load())
		}
	})
}

func TestSafeHooks(t *testing.T) {
	t.Run("panic recovery", func(t *testing.T) {
		ctx := context.Background()
		var panicCaught atomic.Bool

		hooks := NewSafeHooks(Hooks[int]{
			OnValue: func(v int) {
				panic("test panic")
			},
		}, func(r any) {
			panicCaught.Store(true)
		})

		ctx = WithHooks(ctx, hooks.Hooks)

		mapper := Map(func(v int) (int, error) { return v, nil })
		stream := fromSlice([]int{1, 2, 3})
		result := mapper.Apply(stream)
		values, err := Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}
		if !panicCaught.Load() {
			t.Error("panic was not caught")
		}
	})

	t.Run("silent panic recovery", func(t *testing.T) {
		ctx := context.Background()

		hooks := NewSafeHooks(Hooks[int]{
			OnValue: func(v int) {
				panic("test panic")
			},
		}, nil) // nil handler = silent recovery

		ctx = WithHooks(ctx, hooks.Hooks)

		mapper := Map(func(v int) (int, error) { return v, nil })
		stream := fromSlice([]int{1, 2, 3})
		result := mapper.Apply(stream)
		values, err := Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(values) != 3 {
			t.Fatalf("expected 3 values, got %d", len(values))
		}
	})
}

func TestHookInvoker(t *testing.T) {
	t.Run("hasAny returns true when hooks exist", func(t *testing.T) {
		ctx := WithHooks(context.Background(), Hooks[int]{
			OnValue: func(v int) {},
		})

		invoker := newHookInvoker[int](ctx)
		if !invoker.hasAny() {
			t.Error("expected hasAny() to return true")
		}
	})

	t.Run("hasAny returns false when no hooks exist", func(t *testing.T) {
		ctx := context.Background()
		invoker := newHookInvoker[int](ctx)
		if invoker.hasAny() {
			t.Error("expected hasAny() to return false")
		}
	})

	t.Run("caches hook existence flags", func(t *testing.T) {
		ctx := WithHooks(context.Background(), Hooks[int]{
			OnStart: func() {},
			OnValue: func(v int) {},
			OnError: func(err error) {},
		})

		invoker := newHookInvoker[int](ctx)

		if !invoker.hasStart {
			t.Error("expected hasStart to be true")
		}
		if !invoker.hasValue {
			t.Error("expected hasValue to be true")
		}
		if !invoker.hasError {
			t.Error("expected hasError to be true")
		}
		if invoker.hasSentinel {
			t.Error("expected hasSentinel to be false")
		}
		if invoker.hasComplete {
			t.Error("expected hasComplete to be false")
		}
	})
}

func TestContextCancellationWithHooks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var completed atomic.Bool

	ctx = WithHooks(ctx, Hooks[int]{
		OnComplete: func() {
			completed.Store(true)
		},
	})

	mapper := Map(func(v int) (int, error) {
		if v == 2 {
			cancel()
		}
		return v, nil
	})

	stream := fromSlice([]int{1, 2, 3, 4, 5})
	result := mapper.Apply(stream)
	_, _ = Slice(ctx, result)

	// OnComplete should still be called even after cancellation
	if !completed.Load() {
		t.Error("OnComplete was not called after cancellation")
	}
}

func TestMultipleHookTypes(t *testing.T) {
	ctx := context.Background()
	var intHookCalled, stringHookCalled atomic.Bool

	// Add int hooks
	ctx = WithHooks(ctx, Hooks[int]{
		OnValue: func(v int) {
			intHookCalled.Store(true)
		},
	})

	// Add string hooks (different type)
	ctx = WithHooks(ctx, Hooks[string]{
		OnValue: func(v string) {
			stringHookCalled.Store(true)
		},
	})

	// Test int mapper
	intMapper := Map(func(v int) (int, error) { return v, nil })
	intStream := fromSlice([]int{1})
	intResult := intMapper.Apply(intStream)
	_, _ = Slice(ctx, intResult)

	// Test string mapper
	stringMapper := Map(func(v string) (string, error) { return v, nil })
	stringStream := fromSlice([]string{"hello"})
	stringResult := stringMapper.Apply(stringStream)
	_, _ = Slice(ctx, stringResult)

	if !intHookCalled.Load() {
		t.Error("int hook was not called")
	}
	if !stringHookCalled.Load() {
		t.Error("string hook was not called")
	}
}
