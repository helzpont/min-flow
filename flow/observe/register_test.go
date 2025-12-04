package observe

import (
	"context"
	"sync/atomic"
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

func TestOnValue(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var received []any
	err := OnValue(registry, func(v any) {
		received = append(received, v)
	})
	if err != nil {
		t.Fatalf("OnValue registration failed: %v", err)
	}

	// Create and process a stream
	mapper := core.Map(func(x int) (int, error) {
		return x * 2, nil
	})
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(ctx, input)

	_, _ = core.Slice(ctx, output)

	// Verify values were received
	if len(received) != 3 {
		t.Errorf("expected 3 values, got %d", len(received))
	}
}

func TestOnError(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var errors []error
	err := OnError(registry, func(err error) {
		errors = append(errors, err)
	})
	if err != nil {
		t.Fatalf("OnError registration failed: %v", err)
	}

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 3)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Ok(3)
		close(ch)
		return ch
	})

	// Process through a mapper (which auto-invokes interceptors)
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestOnStartAndComplete(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var startCalled, completeCalled atomic.Int32

	err := OnStart(registry, func() {
		startCalled.Add(1)
	})
	if err != nil {
		t.Fatalf("OnStart registration failed: %v", err)
	}

	err = OnComplete(registry, func() {
		completeCalled.Add(1)
	})
	if err != nil {
		t.Fatalf("OnComplete registration failed: %v", err)
	}

	// Process a stream
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if startCalled.Load() != 1 {
		t.Errorf("expected OnStart called once, got %d", startCalled.Load())
	}
	if completeCalled.Load() != 1 {
		t.Errorf("expected OnComplete called once, got %d", completeCalled.Load())
	}
}

func TestWithCounter(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	counter, err := WithCounter(registry)
	if err != nil {
		t.Fatalf("WithCounter failed: %v", err)
	}

	// Create a stream with mixed results
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 4)
		ch <- core.Ok(1)
		ch <- core.Ok(2)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Ok(3)
		close(ch)
		return ch
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if counter.Values() != 3 {
		t.Errorf("expected 3 values, got %d", counter.Values())
	}
	if counter.Errors() != 1 {
		t.Errorf("expected 1 error, got %d", counter.Errors())
	}
	if counter.Total() != 4 {
		t.Errorf("expected 4 total, got %d", counter.Total())
	}
}

func TestWithValueCounter(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	counter, err := WithValueCounter(registry)
	if err != nil {
		t.Fatalf("WithValueCounter failed: %v", err)
	}

	mapper := core.Map(func(x int) (int, error) { return x * 2, nil })
	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	if counter.Count() != 5 {
		t.Errorf("expected 5 values, got %d", counter.Count())
	}
}

func TestWithErrorCollector(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	collector, err := WithErrorCollector(registry)
	if err != nil {
		t.Fatalf("WithErrorCollector failed: %v", err)
	}

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

	if !collector.HasErrors() {
		t.Error("expected HasErrors() to be true")
	}
	if len(collector.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(collector.Errors()))
	}
}

func TestWithLogging(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var logs []string
	err := WithLogging(registry, func(format string, args ...any) {
		logs = append(logs, format)
	})
	if err != nil {
		t.Fatalf("WithLogging failed: %v", err)
	}

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have logged events (StreamStart, ItemReceived*3, ValueReceived*3, ItemEmitted*3, StreamEnd)
	if len(logs) == 0 {
		t.Error("expected some log messages")
	}
}
