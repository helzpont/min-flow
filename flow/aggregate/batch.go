package aggregate

import (
	"context"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// AggregateConfig provides configuration for aggregate transformers.
type AggregateConfig struct {
	// BatchSize specifies the default batch size for batching operations.
	// A value of 0 or negative will use the function-level default.
	BatchSize int

	// BatchTimeout specifies the default timeout for BatchTimeout operations.
	// A value of 0 or negative will use the function-level default.
	BatchTimeout time.Duration
}

// WithBatchSize returns a functional option that sets the batch size.
func WithBatchSize(size int) func(*AggregateConfig) {
	return func(c *AggregateConfig) {
		c.BatchSize = size
	}
}

// WithBatchTimeout returns a functional option that sets the batch timeout.
func WithBatchTimeout(timeout time.Duration) func(*AggregateConfig) {
	return func(c *AggregateConfig) {
		c.BatchTimeout = timeout
	}
}

// effectiveBatchSize returns the batch size to use, considering
// context config and the explicitly provided value.
// If size > 0, it takes precedence. Otherwise, config from context is used.
// Returns 0 if neither provides a valid value (caller must handle).
func effectiveBatchSize(ctx context.Context, size int) int {
	if size > 0 {
		return size
	}
	if cfg, ok := core.GetConfig[*AggregateConfig](ctx); ok && cfg.BatchSize > 0 {
		return cfg.BatchSize
	}
	return 0 // Caller must handle zero case (usually panic)
}

// effectiveBatchTimeout returns the batch timeout to use, considering
// context config and the explicitly provided value.
// If timeout > 0, it takes precedence. Otherwise, config from context is used.
// Returns 0 if neither provides a valid value (caller must handle).
func effectiveBatchTimeout(ctx context.Context, timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	if cfg, ok := core.GetConfig[*AggregateConfig](ctx); ok && cfg.BatchTimeout > 0 {
		return cfg.BatchTimeout
	}
	return 0 // Caller must handle zero case
}

// Batch creates a Transformer that collects items into batches of the specified size.
// When the batch is full, it is emitted as a slice. The final partial batch is emitted
// when the stream completes.
// If size <= 0 and no context config provides a valid size, panics.
func Batch[T any](size int) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		batchSize := effectiveBatchSize(ctx, size)
		if batchSize <= 0 {
			panic("Batch size must be > 0")
		}

		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)
			batch := make([]T, 0, batchSize)

			emit := func() {
				if len(batch) > 0 {
					batchCopy := make([]T, len(batch))
					copy(batchCopy, batch)
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(batchCopy):
					}
					batch = batch[:0]
				}
			}

			for res := range in {
				// Pass through errors as-is (wrapped in a new Result type)
				if res.IsError() {
					// Emit current batch before error
					emit()
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]T](res.Error()):
					}
					continue
				}

				// Pass through sentinels
				if res.IsSentinel() {
					emit()
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[[]T](res.Error()):
					}
					continue
				}

				batch = append(batch, res.Value())
				if len(batch) >= batchSize {
					emit()
				}
			}

			// Emit final partial batch
			emit()
		}()
		return out
	})
}

// BatchTimeout creates a Transformer that collects items into batches, emitting when
// either the batch reaches the specified size OR the timeout elapses (whichever comes first).
// This is useful for creating time-bounded batches in streaming scenarios.
// If size <= 0 and no context config provides a valid size, panics.
func BatchTimeout[T any](size int, timeout time.Duration) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		batchSize := effectiveBatchSize(ctx, size)
		if batchSize <= 0 {
			panic("BatchTimeout size must be > 0")
		}
		batchTimeout := effectiveBatchTimeout(ctx, timeout)
		if batchTimeout <= 0 {
			batchTimeout = timeout // fallback to explicit parameter even if <= 0
		}

		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)
			batch := make([]T, 0, batchSize)
			timer := time.NewTimer(batchTimeout)
			timer.Stop()

			emit := func() {
				timer.Stop()
				if len(batch) > 0 {
					batchCopy := make([]T, len(batch))
					copy(batchCopy, batch)
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(batchCopy):
					}
					batch = batch[:0]
				}
			}

			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					emit()
					timer.Reset(batchTimeout)
				case res, ok := <-in:
					if !ok {
						emit()
						return
					}

					if res.IsError() {
						emit()
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}

					if res.IsSentinel() {
						emit()
						select {
						case <-ctx.Done():
							return
						case out <- core.Sentinel[[]T](res.Error()):
						}
						continue
					}

					// Start timer on first item in batch
					if len(batch) == 0 {
						timer.Reset(batchTimeout)
					}

					batch = append(batch, res.Value())
					if len(batch) >= batchSize {
						emit()
					}
				}
			}
		}()
		return out
	})
}

// Chunk is an alias for Batch - creates fixed-size chunks from the stream.
func Chunk[T any](size int) core.Transformer[T, []T] {
	return Batch[T](size)
}

// Window creates a sliding window Transformer that emits overlapping windows of items.
// Each window contains 'size' items, and windows slide by 'step' items.
// For example, Window(3, 1) on [1,2,3,4,5] produces [[1,2,3], [2,3,4], [3,4,5]].
// If size <= 0 or step <= 0, panics.
func Window[T any](size, step int) core.Transformer[T, []T] {
	if size <= 0 {
		panic("Window size must be > 0")
	}
	if step <= 0 {
		panic("Window step must be > 0")
	}

	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)
			window := make([]T, 0, size)
			skipCount := 0

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]T](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[[]T](res.Error()):
					}
					continue
				}

				// Handle step > size case (skip items between windows)
				if skipCount > 0 {
					skipCount--
					continue
				}

				window = append(window, res.Value())

				if len(window) == size {
					// Emit window copy
					windowCopy := make([]T, size)
					copy(windowCopy, window)
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(windowCopy):
					}

					// Slide window
					if step >= size {
						// Clear window and skip additional items
						window = window[:0]
						skipCount = step - size
					} else {
						// Keep overlapping portion
						window = append(window[:0], window[step:]...)
					}
				}
			}
		}()
		return out
	})
}

// Partition creates a Transformer that splits the stream into two sub-streams based on a predicate.
// Items for which the predicate returns true go to the first slice element, others to the second.
// Both partitions are collected and emitted as a single pair when the stream completes.
// This is a collecting operation - it waits for the entire stream.
func Partition[T any](predicate func(T) bool) core.Transformer[T, [2][]T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[2][]T] {
		out := make(chan core.Result[[2][]T])
		go func() {
			defer close(out)
			var trueItems, falseItems []T

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[2][]T](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[[2][]T](res.Error()):
					}
					continue
				}

				if predicate(res.Value()) {
					trueItems = append(trueItems, res.Value())
				} else {
					falseItems = append(falseItems, res.Value())
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok([2][]T{trueItems, falseItems}):
			}
		}()
		return out
	})
}

// GroupBy creates a Transformer that groups items by a key function.
// All items with the same key are collected into a slice.
// The result is a map from keys to slices of items.
// This is a collecting operation - it waits for the entire stream.
func GroupBy[T any, K comparable](keyFn func(T) K) core.Transformer[T, map[K][]T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[map[K][]T] {
		out := make(chan core.Result[map[K][]T])
		go func() {
			defer close(out)
			groups := make(map[K][]T)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[map[K][]T](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[map[K][]T](res.Error()):
					}
					continue
				}

				key := keyFn(res.Value())
				groups[key] = append(groups[key], res.Value())
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(groups):
			}
		}()
		return out
	})
}
