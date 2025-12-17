package flow_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
)

// Integration: cancellation should stop the pipeline early.
func TestIntegrationCancellationStopsPipeline(t *testing.T) {
	testCtx, testCancel := context.WithTimeout(context.Background(), time.Second)
	defer testCancel()

	ctx, cancel := context.WithCancel(testCtx)
	defer cancel()

	stream := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int], 32)
		go func() {
			defer close(out)
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	mapped := flow.Map(func(n int) (int, error) { return n * 2, nil }).Apply(stream)

	count := 0
	for res := range mapped.Emit(ctx) {
		if res.IsValue() {
			count++
			if count == 20 {
				cancel()
			}
		}
	}

	if ctx.Err() != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", ctx.Err())
	}
	if count < 20 {
		t.Fatalf("expected at least 20 values before cancellation, got %d", count)
	}
	if count > 200 {
		t.Fatalf("expected early stop, got %d items", count)
	}
}

// Integration: panic in mapper should be recovered and surfaced as error Result.
func TestIntegrationPanicRecoveryInMapper(t *testing.T) {
	ctx := context.Background()

	mapper := flow.Map(func(n int) (int, error) {
		if n == 1 {
			panic("boom")
		}
		return n, nil
	})

	stream := flow.FromSlice([]int{1})
	results := flow.Collect(ctx, mapper.Apply(stream))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsError() {
		t.Fatalf("expected error result, got %v", results[0])
	}
}

// Integration: slow consumer with unbuffered output should not deadlock and should process all items.
func TestIntegrationSlowConsumerBackpressure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 5; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	slowSink := flow.Sink[int, int](func(ctx context.Context, s flow.Stream[int]) (int, error) {
		count := 0
		for res := range s.Emit(ctx) {
			if res.IsValue() {
				count++
				time.Sleep(10 * time.Millisecond)
			}
		}
		return count, nil
	})

	resultStream := slowSink.Apply(stream)
	values, err := flow.Slice(ctx, resultStream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(values) != 1 || values[0] != 5 {
		t.Fatalf("expected single result 5, got %v", values)
	}
}
