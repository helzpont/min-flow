package flowerrors

import (
	"context"
	"errors"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

// testStreamFromSlice creates a stream from a slice for testing
func testStreamFromSlice[T any](data []T) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		ch := make(chan core.Result[T], len(data))
		for _, v := range data {
			ch <- core.Ok(v)
		}
		close(ch)
		return ch
	})
}

func TestOnErrorDo(t *testing.T) {
	ctx := context.Background()

	var errors []error
	ctx = OnErrorDo[int](ctx, func(err error) {
		errors = append(errors, err)
	})

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Err[int](context.Canceled)
		ch <- core.Ok(2)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestWithErrorCounter(t *testing.T) {
	ctx := context.Background()

	ctx, counter := WithErrorCounter[int](ctx, nil)

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Err[int](context.Canceled)
		ch <- core.Ok(2)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if counter.Count() != 2 {
		t.Errorf("expected count of 2, got %d", counter.Count())
	}
}

func TestWithErrorCounterWithPredicate(t *testing.T) {
	ctx := context.Background()

	// Only count DeadlineExceeded errors
	ctx, counter := WithErrorCounter[int](ctx, func(err error) bool {
		return errors.Is(err, context.DeadlineExceeded)
	})

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Err[int](context.Canceled)
		ch <- core.Ok(2)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if counter.Count() != 1 {
		t.Errorf("expected count of 1, got %d", counter.Count())
	}
}

func TestWithErrorCollector(t *testing.T) {
	ctx := context.Background()

	ctx, collector := WithErrorCollector[int](ctx)

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Err[int](context.Canceled)
		ch <- core.Ok(2)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if len(collector.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(collector.Errors()))
	}
	if collector.Count() != 2 {
		t.Errorf("expected count of 2, got %d", collector.Count())
	}
}

func TestWithErrorCollectorMaxErrors(t *testing.T) {
	ctx := context.Background()

	ctx, collector := WithErrorCollector[int](ctx, WithMaxErrors(1))

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Err[int](context.Canceled)
		ch <- core.Ok(2)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if len(collector.Errors()) != 1 {
		t.Errorf("expected 1 error (max), got %d", len(collector.Errors()))
	}
}

func TestWithCircuitBreakerMonitor(t *testing.T) {
	t.Run("trips after threshold", func(t *testing.T) {
		ctx := context.Background()

		thresholdHit := false
		ctx, cb := WithCircuitBreakerMonitor[int](ctx, 2, func() {
			thresholdHit = true
		})

		// Create a stream with consecutive errors
		input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int], 3)
			ch <- core.Err[int](context.DeadlineExceeded)
			ch <- core.Err[int](context.Canceled)
			ch <- core.Ok(1)
			close(ch)
			return ch
		})

		mapper := core.Map(func(x int) (int, error) { return x, nil })
		output := mapper.Apply(ctx, input)
		_, _ = core.Slice(ctx, output)

		if !thresholdHit {
			t.Error("expected threshold to be hit")
		}
		if !cb.IsOpen() {
			t.Error("expected circuit breaker to be open")
		}
	})

	t.Run("resets on success", func(t *testing.T) {
		ctx := context.Background()

		ctx, cb := WithCircuitBreakerMonitor[int](ctx, 3, nil)

		// Create a stream with error then success
		input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
			ch := make(chan core.Result[int], 3)
			ch <- core.Err[int](context.DeadlineExceeded)
			ch <- core.Ok(1) // This should reset the count
			ch <- core.Err[int](context.Canceled)
			close(ch)
			return ch
		})

		mapper := core.Map(func(x int) (int, error) { return x, nil })
		output := mapper.Apply(ctx, input)
		_, _ = core.Slice(ctx, output)

		if cb.IsOpen() {
			t.Error("circuit breaker should not be open")
		}
		// After success reset, one more error means count should be 1
		if cb.FailureCount() != 1 {
			t.Errorf("expected failure count of 1, got %d", cb.FailureCount())
		}
	})
}
