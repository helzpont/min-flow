package flowerrors

import (
	"context"
	"errors"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

func TestErrorHandlerInterceptor(t *testing.T) {
	var handledErrors []error
	interceptor := NewErrorHandlerInterceptor(func(err error) {
		handledErrors = append(handledErrors, err)
	})

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	// Create a stream with errors
	errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int])
		go func() {
			defer close(ch)
			ch <- core.Ok(1)
			ch <- core.Err[int](errors.New("error 1"))
			ch <- core.Ok(2)
			ch <- core.Err[int](errors.New("error 2"))
		}()
		return ch
	})

	intercepted := core.Intercept[int]().Apply(errStream)

	for range intercepted.All(ctx) {
	}

	if len(handledErrors) != 2 {
		t.Errorf("expected 2 handled errors, got %d", len(handledErrors))
	}
}

func TestErrorCounterInterceptor(t *testing.T) {
	tests := []struct {
		name      string
		predicate func(error) bool
		errors    []error
		wantCount int64
	}{
		{
			name:      "count all errors",
			predicate: nil,
			errors:    []error{errors.New("a"), errors.New("b"), errors.New("c")},
			wantCount: 3,
		},
		{
			name:      "count matching errors",
			predicate: func(err error) bool { return err.Error() == "specific" },
			errors:    []error{errors.New("other"), errors.New("specific"), errors.New("another")},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := NewErrorCounterInterceptor(tt.predicate)

			ctx, registry := core.WithRegistry(context.Background())
			_ = registry.Register(interceptor)

			errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
				ch := make(chan core.Result[int])
				go func() {
					defer close(ch)
					for _, err := range tt.errors {
						ch <- core.Err[int](err)
					}
				}()
				return ch
			})

			intercepted := core.Intercept[int]().Apply(errStream)
			for range intercepted.All(ctx) {
			}

			if got := interceptor.Count(); got != tt.wantCount {
				t.Errorf("Count() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}

func TestErrorCollectorInterceptor(t *testing.T) {
	t.Run("collects all errors", func(t *testing.T) {
		collector := NewErrorCollectorInterceptor()

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(collector)

		errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				ch <- core.Err[int](errors.New("error 1"))
				ch <- core.Ok(1)
				ch <- core.Err[int](errors.New("error 2"))
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(errStream)
		for range intercepted.All(ctx) {
		}

		if collector.Count() != 2 {
			t.Errorf("Count() = %d, want 2", collector.Count())
		}

		errs := collector.Errors()
		if len(errs) != 2 {
			t.Errorf("len(Errors()) = %d, want 2", len(errs))
		}
	})

	t.Run("respects max errors", func(t *testing.T) {
		collector := NewErrorCollectorInterceptor(WithMaxErrors(2))

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(collector)

		errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				for i := 0; i < 5; i++ {
					ch <- core.Err[int](errors.New("error"))
				}
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(errStream)
		for range intercepted.All(ctx) {
		}

		if collector.Count() != 2 {
			t.Errorf("Count() = %d, want 2", collector.Count())
		}
	})

	t.Run("clear resets errors", func(t *testing.T) {
		collector := NewErrorCollectorInterceptor()

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(collector)

		errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				ch <- core.Err[int](errors.New("error"))
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(errStream)
		for range intercepted.All(ctx) {
		}

		collector.Clear()
		if collector.Count() != 0 {
			t.Errorf("Count() after Clear() = %d, want 0", collector.Count())
		}
	})
}

func TestCircuitBreakerInterceptor(t *testing.T) {
	t.Run("trips after threshold", func(t *testing.T) {
		var tripped bool
		cb := NewCircuitBreakerInterceptor(3, func() { tripped = true })

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(cb)

		errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				for i := 0; i < 5; i++ {
					ch <- core.Err[int](errors.New("error"))
				}
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(errStream)
		for range intercepted.All(ctx) {
		}

		if !tripped {
			t.Error("circuit breaker should have tripped")
		}
		if !cb.IsOpen() {
			t.Error("IsOpen() should be true")
		}
	})

	t.Run("resets on success", func(t *testing.T) {
		cb := NewCircuitBreakerInterceptor(3, nil)

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(cb)

		// Send 2 errors then a success
		mixedStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				ch <- core.Err[int](errors.New("error 1"))
				ch <- core.Err[int](errors.New("error 2"))
				ch <- core.Ok(1) // This should reset
				ch <- core.Err[int](errors.New("error 3"))
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(mixedStream)
		for range intercepted.All(ctx) {
		}

		if cb.IsOpen() {
			t.Error("circuit breaker should not be open after reset")
		}
		if cb.FailureCount() != 1 {
			t.Errorf("FailureCount() = %d, want 1", cb.FailureCount())
		}
	})

	t.Run("reset clears state", func(t *testing.T) {
		cb := NewCircuitBreakerInterceptor(2, nil)

		ctx, registry := core.WithRegistry(context.Background())
		_ = registry.Register(cb)

		errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int])
			go func() {
				defer close(ch)
				ch <- core.Err[int](errors.New("error 1"))
				ch <- core.Err[int](errors.New("error 2"))
			}()
			return ch
		})

		intercepted := core.Intercept[int]().Apply(errStream)
		for range intercepted.All(ctx) {
		}

		cb.Reset()
		if cb.IsOpen() {
			t.Error("IsOpen() should be false after Reset()")
		}
		if cb.FailureCount() != 0 {
			t.Errorf("FailureCount() = %d, want 0", cb.FailureCount())
		}
	})
}
