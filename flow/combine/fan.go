package combine

import (
	"context"
	"fmt"
	"sync"

	"github.com/lguimbarda/min-flow/flow/core"
)

// safeMap wraps a mapper function with panic recovery
func safeMap[IN, OUT any](mapper func(IN) OUT, value IN) (result core.Result[OUT]) {
	defer func() {
		if r := recover(); r != nil {
			result = core.Err[OUT](fmt.Errorf("panic in mapper: %v", r))
		}
	}()
	return core.Ok(mapper(value))
}

// FanOut splits a stream into n identical output streams.
// Each output stream receives all items from the input.
// Returns a slice of n streams that can be consumed independently.
// Uses internal buffering to prevent slow consumers from blocking others.
func FanOut[T any](n int, stream core.Stream[T]) []core.Stream[T] {
	if n <= 0 {
		return nil
	}

	outputs := make([]core.Stream[T], n)
	channels := make([]chan core.Result[T], n)

	for i := 0; i < n; i++ {
		ch := make(chan core.Result[T], 16) // Buffer to reduce blocking
		channels[i] = ch
		outputs[i] = core.Emit(func(ctx context.Context) <-chan core.Result[T] {
			return ch
		})
	}

	// Start distributor that sends to all output channels
	go func() {
		defer func() {
			for _, ch := range channels {
				close(ch)
			}
		}()

		ctx := context.Background()
		for res := range stream.Emit(ctx) {
			for _, ch := range channels {
				ch <- res
			}
		}
	}()

	return outputs
}

// FanOutWith splits a stream using a router function to determine which output receives each item.
// The router returns an index (0 to n-1) for each item. Items with out-of-range indices are dropped.
func FanOutWith[T any](n int, router func(T) int, stream core.Stream[T]) []core.Stream[T] {
	if n <= 0 {
		return nil
	}

	outputs := make([]core.Stream[T], n)
	channels := make([]chan core.Result[T], n)

	for i := 0; i < n; i++ {
		ch := make(chan core.Result[T], 16)
		channels[i] = ch
		outputs[i] = core.Emit(func(ctx context.Context) <-chan core.Result[T] {
			return ch
		})
	}

	go func() {
		defer func() {
			for _, ch := range channels {
				close(ch)
			}
		}()

		ctx := context.Background()
		for res := range stream.Emit(ctx) {
			if res.IsValue() {
				idx := router(res.Value())
				if idx >= 0 && idx < n {
					channels[idx] <- res
				}
			} else {
				// Broadcast errors and sentinels to all
				for _, ch := range channels {
					ch <- res
				}
			}
		}
	}()

	return outputs
}

// FanIn merges multiple streams into a single stream.
// This is an alias for Merge for semantic clarity in fan-out/fan-in patterns.
func FanIn[T any](streams ...core.Stream[T]) core.Stream[T] {
	return Merge(streams...)
}

// Broadcast sends each item to all provided handler functions concurrently.
// Returns a stream of the original items after broadcasting.
// Useful for side effects like logging, metrics, or caching.
func Broadcast[T any](handlers ...func(T)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				if res.IsValue() {
					// Broadcast to all handlers concurrently
					var wg sync.WaitGroup
					wg.Add(len(handlers))
					for _, handler := range handlers {
						go func(h func(T)) {
							defer wg.Done()
							h(res.Value())
						}(handler)
					}
					wg.Wait()
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
}

// RoundRobin distributes items across n worker functions in round-robin fashion.
// Each worker is called sequentially for its assigned items.
// Returns the results from all workers merged into a single stream.
func RoundRobin[IN, OUT any](n int, worker func(IN) OUT) core.Transformer[IN, OUT] {
	if n <= 0 {
		n = 1
	}

	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])

		// Create worker channels
		workerChans := make([]chan core.Result[IN], n)
		for i := 0; i < n; i++ {
			workerChans[i] = make(chan core.Result[IN], 16)
		}

		go func() {
			defer close(out)

			var wg sync.WaitGroup

			// Start workers
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(workerChan <-chan core.Result[IN]) {
					defer wg.Done()
					for res := range workerChan {
						if res.IsError() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Err[OUT](res.Error()):
							}
							continue
						}

						if res.IsSentinel() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Sentinel[OUT](res.Error()):
							}
							continue
						}

						result := safeMap(worker, res.Value())
						select {
						case <-ctx.Done():
							return
						case out <- result:
						}
					}
				}(workerChans[i])
			}

			// Distribute items round-robin
			idx := 0
			for res := range in {
				select {
				case <-ctx.Done():
					goto cleanup
				case workerChans[idx] <- res:
				}
				idx = (idx + 1) % n
			}

		cleanup:
			for _, ch := range workerChans {
				close(ch)
			}
			wg.Wait()
		}()

		return out
	})
}

// Tee creates two identical streams from one input stream.
// Both output streams receive all items. Equivalent to FanOut(2, stream).
func Tee[T any](stream core.Stream[T]) (core.Stream[T], core.Stream[T]) {
	outputs := FanOut(2, stream)
	return outputs[0], outputs[1]
}

// Partition creates two streams based on a predicate.
// Items matching the predicate go to the first stream, others to the second.
// Unlike the Partition transformer which collects all items, this streams items immediately.
func PartitionStream[T any](predicate func(T) bool, stream core.Stream[T]) (matched core.Stream[T], unmatched core.Stream[T]) {
	outputs := FanOutWith(2, func(v T) int {
		if predicate(v) {
			return 0
		}
		return 1
	}, stream)
	return outputs[0], outputs[1]
}

// Balance distributes items across n output streams for load balancing.
// Unlike RoundRobin which uses a fixed order, Balance sends to whichever output
// has buffer space available (using select with multiple cases).
func Balance[T any](n int, stream core.Stream[T]) []core.Stream[T] {
	if n <= 0 {
		return nil
	}

	outputs := make([]core.Stream[T], n)
	channels := make([]chan core.Result[T], n)

	for i := 0; i < n; i++ {
		ch := make(chan core.Result[T], 1) // Small buffer for balancing
		channels[i] = ch
		outputs[i] = core.Emit(func(ctx context.Context) <-chan core.Result[T] {
			return ch
		})
	}

	go func() {
		defer func() {
			for _, ch := range channels {
				close(ch)
			}
		}()

		ctx := context.Background()
		for res := range stream.Emit(ctx) {
			// Try to send to any available channel
			sent := false
			for !sent {
				select {
				case <-ctx.Done():
					return
				case channels[0] <- res:
					sent = true
				case channels[1%n] <- res:
					sent = true
				default:
					// If no channel is immediately available, block on first
					channels[0] <- res
					sent = true
				}
			}
		}
	}()

	return outputs
}
