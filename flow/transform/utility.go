// Package operators provides stream transformation operators for the min-flow framework.
// This file contains utility operators for common stream transformations.
package transform

import (
	"context"
	"sync"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Pairwise creates a Transformer that emits pairs of consecutive items.
// Each emission (except the first) includes the current and previous item.
func Pairwise[T any]() core.Transformer[T, [2]T] {
	return core.Transmitter[T, [2]T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[2]T] {
		out := make(chan core.Result[[2]T])
		go func() {
			defer close(out)

			var prev T
			hasPrev := false

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[2]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					curr := res.Value()
					if hasPrev {
						select {
						case <-ctx.Done():
							return
						case out <- core.Ok([2]T{prev, curr}):
						}
					}
					prev = curr
					hasPrev = true
				}
			}
		}()
		return out
	})
}

// StartWith creates a Transformer that prepends the specified values before
// emitting items from the source stream.
func StartWith[T any](values ...T) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			// Emit prepended values first
			for _, v := range values {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(v):
				}
			}

			// Then emit source stream
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// EndWith creates a Transformer that appends the specified values after
// all items from the source stream have been emitted.
func EndWith[T any](values ...T) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			// Emit source stream first
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						// Source complete, emit appended values
						for _, v := range values {
							select {
							case <-ctx.Done():
								return
							case out <- core.Ok(v):
							}
						}
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// DefaultIfEmpty creates a Transformer that emits the specified default value
// if the source stream completes without emitting any items.
func DefaultIfEmpty[T any](defaultValue T) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			hasEmitted := false
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						if !hasEmitted {
							select {
							case <-ctx.Done():
							case out <- core.Ok(defaultValue):
							}
						}
						return
					}
					if res.IsValue() {
						hasEmitted = true
					}
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// SwitchMap creates a Transformer that projects each source value to a Stream,
// then flattens all of these inner Streams using switch: when a new inner Stream
// is created, the previous inner Stream is cancelled.
func SwitchMap[IN, OUT any](project func(IN) core.Stream[OUT]) core.Transformer[IN, OUT] {
	return core.Transmitter[IN, OUT](func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])
		go func() {
			defer close(out)

			var cancelInner context.CancelFunc
			innerDone := make(chan struct{}, 1)
			hasInner := false
			var innerWg sync.WaitGroup

			// Ensure all inner goroutines complete before returning
			defer func() {
				if cancelInner != nil {
					cancelInner()
				}
				innerWg.Wait()
			}()

			for {
				select {
				case <-ctx.Done():
					return

				case <-innerDone:
					hasInner = false

				case res, ok := <-in:
					if !ok {
						// Wait for current inner to complete
						if hasInner {
							select {
							case <-ctx.Done():
							case <-innerDone:
							}
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[OUT](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Cancel previous inner stream
					if cancelInner != nil {
						cancelInner()
						// Wait for previous inner to finish
						if hasInner {
							select {
							case <-ctx.Done():
								return
							case <-innerDone:
							}
						}
					}

					// Create new inner stream
					innerCtx, cancel := context.WithCancel(ctx)
					cancelInner = cancel
					innerStream := project(res.Value())
					innerCh := innerStream.Emit(innerCtx)
					hasInner = true
					innerWg.Add(1)

					// Drain inner stream (non-blocking, in a goroutine)
					go func(ch <-chan core.Result[OUT], done chan<- struct{}) {
						defer innerWg.Done()
						defer func() {
							select {
							case done <- struct{}{}:
							default:
							}
						}()
						for innerRes := range ch {
							select {
							case <-innerCtx.Done():
								// Drain remaining items after cancellation
								for range ch {
								}
								return
							case out <- innerRes:
							}
						}
					}(innerCh, innerDone)
				}
			}
		}()
		return out
	})
}

// ExhaustMap creates a Transformer that projects each source value to a Stream,
// but ignores new source values while the current inner Stream is still active.
func ExhaustMap[IN, OUT any](project func(IN) core.Stream[OUT]) core.Transformer[IN, OUT] {
	return core.Transmitter[IN, OUT](func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])
		go func() {
			defer close(out)

			innerActive := false
			innerDone := make(chan struct{}, 1)

			for {
				select {
				case <-ctx.Done():
					return

				case <-innerDone:
					innerActive = false

				case res, ok := <-in:
					if !ok {
						// Input closed, wait for any active inner to complete
						if innerActive {
							select {
							case <-ctx.Done():
							case <-innerDone:
							}
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[OUT](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Ignore if inner is still active
					if innerActive {
						continue
					}

					// Start new inner stream
					innerActive = true
					innerStream := project(res.Value())
					innerCh := innerStream.Emit(ctx)

					go func(ch <-chan core.Result[OUT], done chan<- struct{}) {
						defer func() {
							select {
							case done <- struct{}{}:
							default:
							}
						}()
						for innerRes := range ch {
							select {
							case <-ctx.Done():
								return
							case out <- innerRes:
							}
						}
					}(innerCh, innerDone)
				}
			}
		}()
		return out
	})
}

// ConcatMap creates a Transformer that projects each source value to a Stream,
// then flattens all inner Streams sequentially (waits for each to complete before
// subscribing to the next).
func ConcatMap[IN, OUT any](project func(IN) core.Stream[OUT]) core.Transformer[IN, OUT] {
	return core.Transmitter[IN, OUT](func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])
		go func() {
			defer close(out)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[OUT](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Project and drain inner stream completely
					innerStream := project(res.Value())
					innerCh := innerStream.Emit(ctx)
					for innerRes := range innerCh {
						select {
						case <-ctx.Done():
							return
						case out <- innerRes:
						}
					}
				}
			}
		}()
		return out
	})
}

// MergeMap creates a Transformer that projects each source value to a Stream,
// then flattens all inner Streams concurrently (all inner streams run in parallel).
// Use concurrency parameter to limit the number of concurrent inner streams.
// If concurrency is 0 or negative, defaults to 100 as a reasonable upper bound.
func MergeMap[IN, OUT any](project func(IN) core.Stream[OUT], concurrency int) core.Transformer[IN, OUT] {
	if concurrency <= 0 {
		concurrency = 100 // Reasonable default for "unlimited"
	}
	return core.Transmitter[IN, OUT](func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])
		go func() {
			defer close(out)

			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup

			for {
				select {
				case <-ctx.Done():
					wg.Wait()
					return

				case res, ok := <-in:
					if !ok {
						wg.Wait()
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							wg.Wait()
							return
						case out <- core.Err[OUT](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Acquire semaphore
					select {
					case <-ctx.Done():
						wg.Wait()
						return
					case sem <- struct{}{}:
					}

					wg.Add(1)
					innerStream := project(res.Value())
					innerCh := innerStream.Emit(ctx)

					go func(ch <-chan core.Result[OUT]) {
						defer func() {
							<-sem
							wg.Done()
						}()
						for innerRes := range ch {
							select {
							case <-ctx.Done():
								return
							case out <- innerRes:
							}
						}
					}(innerCh)
				}
			}
		}()
		return out
	})
}

// Repeat creates a Transformer that repeats the source stream the specified
// number of times. If count is 0 or negative, the stream is repeated indefinitely.
func Repeat[T any](count int) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			// Collect all items first
			var items []core.Result[T]
			for res := range in {
				items = append(items, res)
				// Also emit first iteration
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}

			// Repeat remaining iterations
			iterations := count - 1
			if count <= 0 {
				iterations = -1 // Infinite
			}

			for i := 0; iterations < 0 || i < iterations; i++ {
				for _, res := range items {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// RepeatWhen creates a Transformer that repeats the source stream when the
// notifier function returns a Stream that emits. The stream completes when
// the notifier completes.
func RepeatWhen[T, N any](notifier func() core.Stream[N]) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			// Collect all items first
			var items []core.Result[T]
			for res := range in {
				items = append(items, res)
				// Also emit first iteration
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}

			// Repeat based on notifier
			for {
				notifierStream := notifier()
				notifierCh := notifierStream.Emit(ctx)

				select {
				case <-ctx.Done():
					return
				case _, ok := <-notifierCh:
					if !ok {
						return // Notifier completed, stop repeating
					}
					// Emit all items again
					for _, res := range items {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}
			}
		}()
		return out
	})
}

// IgnoreElements creates a Transformer that ignores all emitted items,
// only forwarding errors and completion.
func IgnoreElements[T any]() core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					// Only forward errors
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}
			}
		}()
		return out
	})
}

// ToSlice creates a Transformer that collects all items into a single slice.
// The slice is emitted when the source stream completes.
func ToSlice[T any]() core.Transformer[T, []T] {
	return core.Transmitter[T, []T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			var items []T
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						select {
						case <-ctx.Done():
						case out <- core.Ok(items):
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}
					items = append(items, res.Value())
				}
			}
		}()
		return out
	})
}

// ToMap creates a Transformer that collects all items into a map using the
// provided key function. If duplicate keys exist, later values overwrite earlier ones.
func ToMap[T any, K comparable](keyFn func(T) K) core.Transformer[T, map[K]T] {
	return core.Transmitter[T, map[K]T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[map[K]T] {
		out := make(chan core.Result[map[K]T])
		go func() {
			defer close(out)

			result := make(map[K]T)
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						select {
						case <-ctx.Done():
						case out <- core.Ok(result):
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[map[K]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}
					result[keyFn(res.Value())] = res.Value()
				}
			}
		}()
		return out
	})
}

// ToSet creates a Transformer that collects all unique items into a map[T]struct{}.
// This effectively creates a set of all unique values.
func ToSet[T comparable]() core.Transformer[T, map[T]struct{}] {
	return core.Transmitter[T, map[T]struct{}](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[map[T]struct{}] {
		out := make(chan core.Result[map[T]struct{}])
		go func() {
			defer close(out)

			result := make(map[T]struct{})
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						select {
						case <-ctx.Done():
						case out <- core.Ok(result):
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[map[T]struct{}](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}
					result[res.Value()] = struct{}{}
				}
			}
		}()
		return out
	})
}

// Distinct creates a Transformer that only emits items that haven't been seen before.
// Uses a map to track seen values, so T must be comparable.
func Distinct[T comparable]() core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			seen := make(map[T]struct{})
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					val := res.Value()
					if _, exists := seen[val]; !exists {
						seen[val] = struct{}{}
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}
			}
		}()
		return out
	})
}

// DistinctBy creates a Transformer that only emits items whose key (derived
// by the keyFn) hasn't been seen before.
func DistinctBy[T any, K comparable](keyFn func(T) K) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			seen := make(map[K]struct{})
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					key := keyFn(res.Value())
					if _, exists := seen[key]; !exists {
						seen[key] = struct{}{}
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}
			}
		}()
		return out
	})
}

// WithIndex creates a Transformer that wraps each item with its 0-based index.
type Indexed[T any] struct {
	Index int
	Value T
}

func WithIndex[T any]() core.Transformer[T, Indexed[T]] {
	return core.Transmitter[T, Indexed[T]](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[Indexed[T]] {
		out := make(chan core.Result[Indexed[T]])
		go func() {
			defer close(out)

			index := 0
			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[Indexed[T]](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					indexed := Indexed[T]{
						Index: index,
						Value: res.Value(),
					}
					index++

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(indexed):
					}
				}
			}
		}()
		return out
	})
}
