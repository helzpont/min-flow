package flowerrors_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
)

func TestRetry(t *testing.T) {
	tests := []struct {
		name       string
		input      []int
		maxRetries int
		failUntil  int
		want       []int
		wantErrors int
	}{
		{
			name:       "no retries needed",
			input:      []int{1, 2, 3},
			maxRetries: 3,
			failUntil:  0,
			want:       []int{2, 4, 6},
			wantErrors: 0,
		},
		{
			name:       "retry succeeds on second attempt",
			input:      []int{1, 2, 3},
			maxRetries: 3,
			failUntil:  1,
			want:       []int{2, 4, 6},
			wantErrors: 0,
		},
		{
			name:       "retry succeeds on last attempt",
			input:      []int{1, 2, 3},
			maxRetries: 3,
			failUntil:  3,
			want:       []int{2, 4, 6},
			wantErrors: 0,
		},
		{
			name:       "retry exhausted",
			input:      []int{1, 2, 3},
			maxRetries: 2,
			failUntil:  5,
			want:       []int{},
			wantErrors: 3,
		},
		{
			name:       "empty stream",
			input:      []int{},
			maxRetries: 3,
			failUntil:  0,
			want:       []int{},
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)

			attempts := make(map[int]int)
			operation := func(v int) (int, error) {
				attempts[v]++
				if attempts[v] <= tt.failUntil {
					return 0, errors.New("transient error")
				}
				return v * 2, nil
			}

			result := flowerrors.Retry(tt.maxRetries, operation).Apply(stream)

			var values []int
			var errCount int
			for r := range result.Emit(ctx) {
				if r.IsValue() {
					values = append(values, r.Value())
				} else if r.IsError() {
					errCount++
				}
			}

			if len(values) != len(tt.want) {
				t.Fatalf("got %d values, want %d", len(values), len(tt.want))
			}
			for i := range values {
				if values[i] != tt.want[i] {
					t.Errorf("values[%d] = %v, want %v", i, values[i], tt.want[i])
				}
			}
			if errCount != tt.wantErrors {
				t.Errorf("got %d errors, want %d", errCount, tt.wantErrors)
			}
		})
	}
}

func TestRetryWithBackoff(t *testing.T) {
	tests := []struct {
		name     string
		strategy flowerrors.BackoffStrategy
		attempts int
		wantMin  time.Duration
		wantMax  time.Duration
	}{
		{
			name:     "constant backoff",
			strategy: flowerrors.ConstantBackoff(10 * time.Millisecond),
			attempts: 3,
			wantMin:  20 * time.Millisecond,
			wantMax:  100 * time.Millisecond,
		},
		{
			name:     "linear backoff",
			strategy: flowerrors.LinearBackoff(5 * time.Millisecond),
			attempts: 3,
			wantMin:  15 * time.Millisecond,
			wantMax:  100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice([]int{1})

			attemptCount := 0
			operation := func(v int) (int, error) {
				attemptCount++
				if attemptCount < tt.attempts {
					return 0, errors.New("retry")
				}
				return v * 2, nil
			}

			start := time.Now()
			result := flowerrors.RetryWithBackoff(tt.attempts, tt.strategy, operation).Apply(stream)
			got, err := flow.Slice[int](ctx, result)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != 2 {
				t.Errorf("got %v, want [2]", got)
			}
			if elapsed < tt.wantMin {
				t.Errorf("elapsed %v < minimum %v", elapsed, tt.wantMin)
			}
			if elapsed > tt.wantMax {
				t.Errorf("elapsed %v > maximum %v", elapsed, tt.wantMax)
			}
		})
	}
}

func TestRetryWhen(t *testing.T) {
	errRetryable := errors.New("retryable")
	errFatal := errors.New("fatal")

	tests := []struct {
		name        string
		maxRetries  int
		errToThrow  error
		shouldRetry func(error, int) bool
		wantValue   bool
	}{
		{
			name:       "retry on retryable error",
			maxRetries: 3,
			errToThrow: errRetryable,
			shouldRetry: func(err error, _ int) bool {
				return errors.Is(err, errRetryable)
			},
			wantValue: true,
		},
		{
			name:       "no retry on fatal error",
			maxRetries: 3,
			errToThrow: errFatal,
			shouldRetry: func(err error, _ int) bool {
				return errors.Is(err, errRetryable)
			},
			wantValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice([]int{1})

			attempts := 0
			operation := func(v int) (int, error) {
				attempts++
				if attempts == 1 {
					return 0, tt.errToThrow
				}
				return v * 2, nil
			}

			result := flowerrors.RetryWhen(tt.maxRetries, tt.shouldRetry, operation).Apply(stream)

			var hasValue bool
			for r := range result.Emit(ctx) {
				if r.IsValue() {
					hasValue = true
				}
			}

			if hasValue != tt.wantValue {
				t.Errorf("got value = %v, want %v", hasValue, tt.wantValue)
			}
		})
	}
}

func TestCircuitBreaker(t *testing.T) {
	t.Run("stays closed on success", func(t *testing.T) {
		operation := func(v int) (int, error) {
			return v * 2, nil
		}

		cb := flowerrors.NewCircuitBreaker(operation, 3, 100*time.Millisecond, 1)

		for i := 0; i < 10; i++ {
			result, err := cb.Execute(i)
			if err != nil {
				t.Fatalf("unexpected error on iteration %d: %v", i, err)
			}
			if result != i*2 {
				t.Errorf("got %d, want %d", result, i*2)
			}
		}

		if state := cb.State(); state != flowerrors.CircuitClosed {
			t.Errorf("got state %v, want CircuitClosed", state)
		}
	})

	t.Run("opens after threshold failures", func(t *testing.T) {
		failCount := 0
		operation := func(v int) (int, error) {
			failCount++
			return 0, errors.New("failure")
		}

		cb := flowerrors.NewCircuitBreaker(operation, 3, 100*time.Millisecond, 1)

		for i := 0; i < 3; i++ {
			_, err := cb.Execute(i)
			if err == nil {
				t.Fatal("expected error")
			}
		}

		if state := cb.State(); state != flowerrors.CircuitOpen {
			t.Errorf("got state %v, want CircuitOpen", state)
		}

		_, err := cb.Execute(0)
		if !errors.Is(err, flowerrors.ErrCircuitOpen) {
			t.Errorf("got error %v, want ErrCircuitOpen", err)
		}
	})

	t.Run("transitions to half-open after timeout", func(t *testing.T) {
		attempts := 0
		operation := func(v int) (int, error) {
			attempts++
			if attempts <= 3 {
				return 0, errors.New("failure")
			}
			return v * 2, nil
		}

		cb := flowerrors.NewCircuitBreaker(operation, 3, 50*time.Millisecond, 1)

		for i := 0; i < 3; i++ {
			cb.Execute(i)
		}

		if state := cb.State(); state != flowerrors.CircuitOpen {
			t.Fatalf("expected CircuitOpen, got %v", state)
		}

		time.Sleep(60 * time.Millisecond)

		result, err := cb.Execute(5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("got %d, want 10", result)
		}

		if state := cb.State(); state != flowerrors.CircuitClosed {
			t.Errorf("expected CircuitClosed, got %v", state)
		}
	})
}

func TestWithCircuitBreaker(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	operation := func(v int) (int, error) {
		return v * 2, nil
	}
	cb := flowerrors.NewCircuitBreaker(operation, 3, 100*time.Millisecond, 1)

	result := flowerrors.WithCircuitBreaker(cb).Apply(stream)
	got, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{2, 4, 6, 8, 10}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestFallback(t *testing.T) {
	ctx := context.Background()

	t.Run("fallback on error", func(t *testing.T) {
		emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			out := make(chan core.Result[int])
			go func() {
				defer close(out)
				out <- core.Ok(1)
				out <- core.Err[int](errors.New("error"))
				out <- core.Ok(3)
			}()
			return out
		})
		stream := emitter

		fallbackFn := func(v int, _ error) int {
			return v * 10
		}

		result := flowerrors.Fallback(fallbackFn).Apply(stream)

		var values []int
		for r := range result.Emit(ctx) {
			if r.IsValue() {
				values = append(values, r.Value())
			}
		}

		want := []int{1, 10, 3}
		if len(values) != len(want) {
			t.Fatalf("got %v, want %v", values, want)
		}
		for i := range values {
			if values[i] != want[i] {
				t.Errorf("values[%d] = %d, want %d", i, values[i], want[i])
			}
		}
	})

	t.Run("fallback value", func(t *testing.T) {
		emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			out := make(chan core.Result[int])
			go func() {
				defer close(out)
				out <- core.Err[int](errors.New("error"))
				out <- core.Ok(1)
				out <- core.Err[int](errors.New("error"))
			}()
			return out
		})
		stream := emitter

		result := flowerrors.FallbackValue(-1).Apply(stream)

		var values []int
		var errCount int
		for r := range result.Emit(ctx) {
			if r.IsValue() {
				values = append(values, r.Value())
			} else if r.IsError() {
				errCount++
			}
		}

		want := []int{1, -1}
		if len(values) != len(want) {
			t.Fatalf("got values %v, want %v", values, want)
		}
		if errCount != 1 {
			t.Errorf("got %d errors, want 1", errCount)
		}
	})
}

func TestRecover(t *testing.T) {
	ctx := context.Background()

	t.Run("recover from error", func(t *testing.T) {
		emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			out := make(chan core.Result[int])
			go func() {
				defer close(out)
				out <- core.Ok(1)
				out <- core.Err[int](errors.New("recoverable"))
				out <- core.Ok(3)
			}()
			return out
		})
		stream := emitter

		recoverFn := func(err error) (int, error) {
			return 999, nil
		}

		result := flowerrors.Recover(recoverFn).Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []int{1, 999, 3}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})

	t.Run("recovery fails", func(t *testing.T) {
		emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			out := make(chan core.Result[int])
			go func() {
				defer close(out)
				out <- core.Err[int](errors.New("original"))
			}()
			return out
		})
		stream := emitter

		recoverFn := func(err error) (int, error) {
			return 0, errors.New("recovery failed")
		}

		result := flowerrors.Recover(recoverFn).Apply(stream)

		var errCount int
		for r := range result.Emit(ctx) {
			if r.IsError() {
				errCount++
				if r.Error().Error() != "recovery failed" {
					t.Errorf("got error %v, want 'recovery failed'", r.Error())
				}
			}
		}

		if errCount != 1 {
			t.Errorf("got %d errors, want 1", errCount)
		}
	})
}

func TestRecoverPanic(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](core.ErrPanic{Value: "panic!"})
			out <- core.Err[int](errors.New("normal error"))
			out <- core.Ok(4)
		}()
		return out
	})
	stream := emitter

	recoverFn := func(panicValue any) (int, error) {
		return -1, nil
	}

	result := flowerrors.RecoverPanic(recoverFn).Apply(stream)

	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	want := []int{1, -1, 4}
	if len(values) != len(want) {
		t.Fatalf("got values %v, want %v", values, want)
	}
	for i := range values {
		if values[i] != want[i] {
			t.Errorf("values[%d] = %d, want %d", i, values[i], want[i])
		}
	}
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var attempts int32
	operation := func(v int) (int, error) {
		atomic.AddInt32(&attempts, 1)
		time.Sleep(50 * time.Millisecond)
		return 0, errors.New("always fail")
	}

	stream := flow.FromSlice([]int{1, 2, 3})
	result := flowerrors.RetryWithBackoff(10, flowerrors.ConstantBackoff(10*time.Millisecond), operation).Apply(stream)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	for range result.Emit(ctx) {
	}

	if atomic.LoadInt32(&attempts) > 5 {
		t.Errorf("expected fewer attempts due to cancellation, got %d", attempts)
	}
}

func TestBackoffStrategies(t *testing.T) {
	t.Run("constant backoff", func(t *testing.T) {
		strategy := flowerrors.ConstantBackoff(100 * time.Millisecond)
		for i := 0; i < 5; i++ {
			delay := strategy(i)
			if delay != 100*time.Millisecond {
				t.Errorf("attempt %d: got %v, want 100ms", i, delay)
			}
		}
	})

	t.Run("linear backoff", func(t *testing.T) {
		strategy := flowerrors.LinearBackoff(10 * time.Millisecond)
		expected := []time.Duration{10, 20, 30, 40, 50}
		for i, want := range expected {
			got := strategy(i)
			wantDuration := want * time.Millisecond
			if got != wantDuration {
				t.Errorf("attempt %d: got %v, want %v", i, got, wantDuration)
			}
		}
	})

	t.Run("exponential backoff with cap", func(t *testing.T) {
		strategy := flowerrors.ExponentialBackoff(10*time.Millisecond, 100*time.Millisecond)
		expected := []time.Duration{10, 20, 40, 80, 100, 100}
		for i, want := range expected {
			got := strategy(i)
			wantDuration := want * time.Millisecond
			if got != wantDuration {
				t.Errorf("attempt %d: got %v, want %v", i, got, wantDuration)
			}
		}
	})

	t.Run("exponential backoff without cap", func(t *testing.T) {
		strategy := flowerrors.ExponentialBackoff(10*time.Millisecond, 0)
		expected := []time.Duration{10, 20, 40, 80, 160}
		for i, want := range expected {
			got := strategy(i)
			wantDuration := want * time.Millisecond
			if got != wantDuration {
				t.Errorf("attempt %d: got %v, want %v", i, got, wantDuration)
			}
		}
	})
}
