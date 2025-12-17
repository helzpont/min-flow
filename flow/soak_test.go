package flow_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/timing"
)

func runSoakScenario(t *testing.T, name string, build func(context.Context) flow.Stream[int], cancelAfter int, maxDuration time.Duration, expectMin int, expectMax int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	stream := build(ctx)

	count := 0
	for res := range stream.Emit(ctx) {
		if res.IsValue() {
			count++
			if cancelAfter > 0 && count == cancelAfter {
				cancel()
			}
		}
	}

	if ctx.Err() != nil && ctx.Err() != context.Canceled {
		t.Fatalf("%s: context ended with %v", name, ctx.Err())
	}
	if expectMin >= 0 && count < expectMin {
		t.Fatalf("%s: expected at least %d values, got %d", name, expectMin, count)
	}
	if expectMax > 0 && count > expectMax {
		t.Fatalf("%s: expected at most %d values, got %d", name, expectMax, count)
	}
}

func TestSoak_FastProducerSlowConsumer(t *testing.T) {
	build := func(ctx context.Context) flow.Stream[int] {
		producer := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
			out := make(chan flow.Result[int], 64)
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

		slow := flow.Transmit(func(ctx context.Context, in <-chan flow.Result[int]) <-chan flow.Result[int] {
			out := make(chan flow.Result[int])
			go func() {
				defer close(out)
				for res := range in {
					if res.IsValue() {
						time.Sleep(2 * time.Millisecond)
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}()
			return out
		})

		return slow.Apply(producer)
	}

	runSoakScenario(t, "fast_producer_slow_consumer", build, 25, 150*time.Millisecond, 20, 80)
}

func TestSoak_BurstyProducerBuffered(t *testing.T) {
	build := func(ctx context.Context) flow.Stream[int] {
		emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
			out := make(chan flow.Result[int], 16)
			go func() {
				defer close(out)
				for burst := 0; burst < 5; burst++ {
					for i := 0; i < 10; i++ {
						select {
						case <-ctx.Done():
							return
						case out <- flow.Ok(burst*10 + i):
						}
					}
					time.Sleep(time.Millisecond)
				}
			}()
			return out
		})

		buffered := timing.Buffer[int](8)
		return buffered.Apply(emitter)
	}

	runSoakScenario(t, "bursty_producer_buffered", build, 0, 200*time.Millisecond, 40, 60)
}

func TestSoak_CancellationPropagation(t *testing.T) {
	build := func(ctx context.Context) flow.Stream[int] {
		emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
			out := make(chan flow.Result[int])
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

		mapped := flow.Map(func(n int) (int, error) {
			if n%7 == 0 {
				return 0, nil
			}
			return n * 3, nil
		})

		return mapped.Apply(emitter)
	}

	runSoakScenario(t, "cancellation_propagation", build, 5, 100*time.Millisecond, 5, 25)
}
