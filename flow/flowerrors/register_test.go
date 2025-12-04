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

// testStreamWithErrors creates a stream with mixed values and errors
func testStreamWithErrors[T any](values []T, errIndices map[int]error) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		ch := make(chan core.Result[T], len(values)+len(errIndices))
		idx := 0
		for i, v := range values {
			if err, hasErr := errIndices[i]; hasErr {
				ch <- core.Err[T](err)
				idx++
			}
			ch <- core.Ok(v)
			idx++
		}
		// Any trailing errors
		for i := len(values); ; i++ {
			if err, hasErr := errIndices[i]; hasErr {
				ch <- core.Err[T](err)
			} else {
				break
			}
		}
		close(ch)
		return ch
	})
}

func TestOnErrorDo(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var receivedErrors []error
	err := OnErrorDo(registry, func(err error) {
		receivedErrors = append(receivedErrors, err)
	})
	if err != nil {
		t.Fatalf("OnErrorDo registration failed: %v", err)
	}

	// Create a mapper that produces some errors
	mapper := core.Map(func(x int) (int, error) {
		if x%2 == 0 {
			return 0, errors.New("even number error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have received 2 errors (for 2 and 4)
	if len(receivedErrors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(receivedErrors))
	}
}

func TestWithErrorCounter(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	counter, err := WithErrorCounter(registry, nil)
	if err != nil {
		t.Fatalf("WithErrorCounter registration failed: %v", err)
	}

	// Create a mapper that produces some errors
	mapper := core.Map(func(x int) (int, error) {
		if x%2 == 0 {
			return 0, errors.New("even number error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have counted 2 errors
	if counter.Count() != 2 {
		t.Errorf("expected count of 2, got %d", counter.Count())
	}
}

func TestWithErrorCounterWithPredicate(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	specificErr := errors.New("specific error")
	counter, err := WithErrorCounter(registry, func(err error) bool {
		return err.Error() == "specific error"
	})
	if err != nil {
		t.Fatalf("WithErrorCounter registration failed: %v", err)
	}

	// Create a mapper that produces different errors
	mapper := core.Map(func(x int) (int, error) {
		if x == 2 {
			return 0, specificErr
		}
		if x == 4 {
			return 0, errors.New("other error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have counted only 1 error (the specific one)
	if counter.Count() != 1 {
		t.Errorf("expected count of 1, got %d", counter.Count())
	}
}

func TestWithErrorCollector(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	collector, err := WithErrorCollector(registry)
	if err != nil {
		t.Fatalf("WithErrorCollector registration failed: %v", err)
	}

	// Create a mapper that produces some errors
	mapper := core.Map(func(x int) (int, error) {
		if x%2 == 0 {
			return 0, errors.New("even number error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have collected 2 errors
	errs := collector.Errors()
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
	if collector.Count() != 2 {
		t.Errorf("expected count of 2, got %d", collector.Count())
	}
}

func TestWithErrorCollectorMaxErrors(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	collector, err := WithErrorCollector(registry, WithMaxErrors(1))
	if err != nil {
		t.Fatalf("WithErrorCollector registration failed: %v", err)
	}

	// Create a mapper that produces many errors
	mapper := core.Map(func(x int) (int, error) {
		if x%2 == 0 {
			return 0, errors.New("even number error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Should have collected only 1 error due to max
	errs := collector.Errors()
	if len(errs) != 1 {
		t.Errorf("expected 1 error (max), got %d", len(errs))
	}
}

func TestWithCircuitBreakerMonitor(t *testing.T) {
	ctx, registry := core.WithRegistry(context.Background())

	var thresholdHit bool
	cb, err := WithCircuitBreakerMonitor(registry, 2, func() {
		thresholdHit = true
	})
	if err != nil {
		t.Fatalf("WithCircuitBreakerMonitor registration failed: %v", err)
	}

	// Create a mapper that produces errors
	mapper := core.Map(func(x int) (int, error) {
		if x%2 == 0 {
			return 0, errors.New("even number error")
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output := mapper.Apply(ctx, input)
	_, _ = core.Slice(ctx, output)

	// Threshold should be hit after 2 consecutive errors (2 and 4 aren't consecutive though)
	// Actually the CB resets on success, so it depends on the order
	// With 1,2,3,4,5 we get: success(1), error(2), success(3), error(4), success(5)
	// So it never reaches threshold of 2 consecutive
	if thresholdHit {
		t.Error("expected threshold NOT to be hit due to interleaved successes")
	}

	// Now test with consecutive errors
	ctx2, registry2 := core.WithRegistry(context.Background())
	var threshold2Hit bool
	cb2, _ := WithCircuitBreakerMonitor(registry2, 2, func() {
		threshold2Hit = true
	})

	// Create a mapper that produces consecutive errors
	mapper2 := core.Map(func(x int) (int, error) {
		if x > 1 { // Errors for 2,3,4,5
			return 0, errors.New("error")
		}
		return x, nil
	})

	input2 := testStreamFromSlice([]int{1, 2, 3, 4, 5})
	output2 := mapper2.Apply(ctx2, input2)
	_, _ = core.Slice(ctx2, output2)

	if !threshold2Hit {
		t.Error("expected threshold to be hit with consecutive errors")
	}
	if !cb2.IsOpen() {
		t.Error("expected circuit breaker to be open")
	}

	// Test reset
	cb.Reset()
	if cb.IsOpen() {
		t.Error("expected circuit breaker to be closed after reset")
	}
}
