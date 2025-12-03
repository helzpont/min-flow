// Package operators provides stream transformation operators for the min-flow framework.
// This file contains sampling and filtering operators for data reduction.
// Note: Basic sample-by-interval is in buffer.go as Sample.
package filter

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// SampleWith creates a Transformer that emits the most recent item from the source
// whenever the sampler stream emits. Items from the source between samples are dropped.
func SampleWith[T, S any](sampler core.Stream[S]) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			samplerCh := sampler.Emit(ctx)
			var lastItem core.Result[T]
			var hasLastItem bool
			var mu sync.Mutex

			for {
				select {
				case <-ctx.Done():
					return

				case _, ok := <-samplerCh:
					if !ok {
						return
					}
					mu.Lock()
					item := lastItem
					hasItem := hasLastItem
					hasLastItem = false
					mu.Unlock()

					if hasItem {
						select {
						case <-ctx.Done():
							return
						case out <- item:
						}
					}

				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						// Forward errors immediately
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

					mu.Lock()
					lastItem = res
					hasLastItem = true
					mu.Unlock()
				}
			}
		}()
		return out
	})
}

// AuditTime creates a Transformer that, when the source emits, waits for the
// specified duration, then emits the most recent value from the source.
func AuditTime[T any](d time.Duration) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			var timer *time.Timer
			var timerC <-chan time.Time
			var lastItem core.Result[T]
			var hasLastItem bool

			for {
				select {
				case <-ctx.Done():
					if timer != nil {
						timer.Stop()
					}
					return

				case <-timerC:
					if hasLastItem {
						select {
						case <-ctx.Done():
							return
						case out <- lastItem:
						}
						hasLastItem = false
					}
					timer = nil
					timerC = nil

				case res, ok := <-in:
					if !ok {
						if timer != nil {
							timer.Stop()
						}
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

					lastItem = res
					hasLastItem = true
					if timer == nil {
						timer = time.NewTimer(d)
						timerC = timer.C
					}
				}
			}
		}()
		return out
	})
}

// ThrottleLast creates a Transformer that collects items for the specified duration,
// then emits the last item received during that period, and repeats.
// If no items are received during a period, nothing is emitted for that period.
func ThrottleLast[T any](d time.Duration) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			ticker := time.NewTicker(d)
			defer ticker.Stop()

			var lastItem core.Result[T]
			var hasLastItem bool
			var mu sync.Mutex

			for {
				select {
				case <-ctx.Done():
					return

				case <-ticker.C:
					mu.Lock()
					item := lastItem
					hasItem := hasLastItem
					hasLastItem = false
					mu.Unlock()

					if hasItem {
						select {
						case <-ctx.Done():
							return
						case out <- item:
						}
					}

				case res, ok := <-in:
					if !ok {
						// Emit last item before closing
						mu.Lock()
						item := lastItem
						hasItem := hasLastItem
						mu.Unlock()
						if hasItem {
							select {
							case <-ctx.Done():
							case out <- item:
							}
						}
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

					mu.Lock()
					lastItem = res
					hasLastItem = true
					mu.Unlock()
				}
			}
		}()
		return out
	})
}

// DistinctUntilChanged creates a Transformer that only emits when the current item
// is different from the previous item.
func DistinctUntilChanged[T comparable]() core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			var lastValue T
			first := true

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
					if first || val != lastValue {
						first = false
						lastValue = val
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

// DistinctUntilChangedBy creates a Transformer that only emits when the key
// derived from the current item is different from the key of the previous item.
func DistinctUntilChangedBy[T any, K comparable](keyFn func(T) K) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			var lastKey K
			first := true

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
					if first || key != lastKey {
						first = false
						lastKey = key
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

// RandomSample creates a Transformer that samples items with the given probability.
// Each item has a chance of being emitted based on the probability (0.0 to 1.0).
func RandomSample[T any](probability float64) core.Transformer[T, T] {
	if probability <= 0 {
		// Emit nothing
		return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
			out := make(chan core.Result[T])
			go func() {
				defer close(out)
				for range in {
				}
			}()
			return out
		})
	}
	if probability >= 1 {
		// Emit everything
		return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
			out := make(chan core.Result[T])
			go func() {
				defer close(out)
				for res := range in {
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

	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))

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

					if rng.Float64() < probability {
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

// ReservoirSample creates a Transformer that performs reservoir sampling,
// randomly selecting k items from the stream with equal probability.
// The result is emitted when the stream ends.
func ReservoirSample[T any](k int) core.Transformer[T, []T] {
	if k <= 0 {
		k = 1
	}
	return core.Transmitter[T, []T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			reservoir := make([]T, 0, k)
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			count := 0

			for {
				select {
				case <-ctx.Done():
					if len(reservoir) > 0 {
						select {
						case out <- core.Ok(reservoir):
						default:
						}
					}
					return

				case res, ok := <-in:
					if !ok {
						if len(reservoir) > 0 {
							select {
							case <-ctx.Done():
							case out <- core.Ok(reservoir):
							}
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

					count++
					if len(reservoir) < k {
						reservoir = append(reservoir, res.Value())
					} else {
						// Randomly replace with decreasing probability
						j := rng.Intn(count)
						if j < k {
							reservoir[j] = res.Value()
						}
					}
				}
			}
		}()
		return out
	})
}

// EveryNth creates a Transformer that emits every nth item, starting from the first.
func EveryNth[T any](n int) core.Transformer[T, T] {
	if n <= 0 {
		n = 1
	}
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			count := 0

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

					count++
					if count == n {
						count = 0
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

// TakeEvery is an alias for EveryNth with a more descriptive name.
func TakeEvery[T any](n int) core.Transformer[T, T] {
	return EveryNth[T](n)
}
