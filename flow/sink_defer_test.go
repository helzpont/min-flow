package flow_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
)

func TestSinkDeferIsLazy(t *testing.T) {
	var started atomic.Bool

	stream := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		started.Store(true)
		out := make(chan flow.Result[int], 1)
		out <- flow.Ok(42)
		close(out)
		return out
	})

	sink := flow.Sink[int, int](func(ctx context.Context, s flow.Stream[int]) (int, error) {
		vals, err := flow.Slice(ctx, s)
		if err != nil {
			return 0, err
		}
		return len(vals), nil
	})

	thunk := sink.Defer(stream)

	if started.Load() {
		t.Fatalf("expected stream to be lazy until thunk is invoked")
	}

	got, err := thunk(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1 {
		t.Fatalf("expected 1 value, got %d", got)
	}
	if !started.Load() {
		t.Fatalf("expected stream to start when thunk is invoked")
	}
}
