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

func TestWithValueHook(t *testing.T) {
	ctx := context.Background()

	var received []int
	ctx = WithValueHook(ctx, func(v int) {
		received = append(received, v)
	})

	// Create and process a stream
	mapper := core.Map(func(x int) (int, error) {
		return x * 2, nil
	})
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(input)

	_, _ = core.Slice(ctx, output)

	// Verify values were received
	if len(received) != 3 {
		t.Errorf("expected 3 values, got %d", len(received))
	}
	expected := []int{2, 4, 6}
	for i, v := range received {
		if v != expected[i] {
			t.Errorf("received[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestWithErrorHook(t *testing.T) {
	ctx := context.Background()

	var errors []error
	ctx = WithErrorHook[int](ctx, func(err error) {
		errors = append(errors, err)
	})

	// Create a stream with errors
	input := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int], 3)
		ch <- core.Ok(1)
		ch <- core.Err[int](context.DeadlineExceeded)
		ch <- core.Ok(3)
		close(ch)
		return ch
	})

	// Process through a mapper (which auto-invokes hooks)
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	output := mapper.Apply(input)
	_, _ = core.Slice(ctx, output)

	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(errors))
	}
}

func TestWithStartAndCompleteHooks(t *testing.T) {
	ctx := context.Background()

	var startCalled, completeCalled atomic.Int32

	ctx = WithStartHook[int](ctx, func() {
		startCalled.Add(1)
	})
	ctx = WithCompleteHook[int](ctx, func() {
		completeCalled.Add(1)
	})

	// Process a stream
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(input)
	_, _ = core.Slice(ctx, output)

	if startCalled.Load() != 1 {
		t.Errorf("expected OnStart called once, got %d", startCalled.Load())
	}
	if completeCalled.Load() != 1 {
		t.Errorf("expected OnComplete called once, got %d", completeCalled.Load())
	}
}

func TestWithCounter(t *testing.T) {
	ctx := context.Background()

	ctx, counter := WithCounter[int](ctx)

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
	output := mapper.Apply(input)
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
	ctx := context.Background()

	ctx, counter := WithValueCounter[int](ctx)

	mapper := core.Map(func(x int) (int, error) { return x * 2, nil })
	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(input)
	_, _ = core.Slice(ctx, output)

	if counter.Count() != 5 {
		t.Errorf("expected 5 values, got %d", counter.Count())
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
	output := mapper.Apply(input)
	_, _ = core.Slice(ctx, output)

	if !collector.HasErrors() {
		t.Error("expected HasErrors() to be true")
	}
	if len(collector.Errors()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(collector.Errors()))
	}
}

func TestWithLogging(t *testing.T) {
	ctx := context.Background()

	var logs []string
	ctx = WithLogging[int](ctx, func(format string, args ...any) {
		logs = append(logs, format)
	})

	mapper := core.Map(func(x int) (int, error) { return x, nil })
	input := testStreamFromSlice([]int{1, 2, 3})
	output := mapper.Apply(input)
	_, _ = core.Slice(ctx, output)

	// Should have logged start, 3 values, and complete
	if len(logs) != 5 {
		t.Errorf("expected 5 log messages (start + 3 values + complete), got %d: %v", len(logs), logs)
	}
	if logs[0] != "stream started" {
		t.Errorf("first log should be 'stream started', got %s", logs[0])
	}
	if logs[4] != "stream completed" {
		t.Errorf("last log should be 'stream completed', got %s", logs[4])
	}
}
