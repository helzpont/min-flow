package filter

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Take creates a Transformer that passes through only the first n items.
// After n items have been emitted, the stream completes.
// If n <= 0, an empty stream is returned.
func Take[T any](n int) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			if n <= 0 {
				return
			}

			count := 0
			for res := range in {
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}

				// Only count values, not errors/sentinels
				if res.IsValue() {
					count++
					if count >= n {
						return
					}
				}
			}
		}()
		return out
	})
}

// TakeWhile creates a Transformer that passes through items while the predicate returns true.
// Once the predicate returns false, the stream completes (remaining items are not consumed).
// Errors are passed through but don't affect the predicate evaluation.
func TakeWhile[T any](predicate func(T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			for res := range in {
				// Pass through errors and sentinels
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Check predicate for values
				if !predicate(res.Value()) {
					return
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

// Skip creates a Transformer that skips the first n items, then passes through the rest.
// If n <= 0, all items are passed through.
func Skip[T any](n int) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			skipped := 0
			for res := range in {
				// Always pass through errors and sentinels
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Skip values until we've skipped n
				if skipped < n {
					skipped++
					continue
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

// SkipWhile creates a Transformer that skips items while the predicate returns true.
// Once the predicate returns false, all subsequent items (including the first false one) are passed through.
// Errors are passed through but don't affect the predicate evaluation.
func SkipWhile[T any](predicate func(T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			skipping := true
			for res := range in {
				// Always pass through errors and sentinels
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Check if we should stop skipping
				if skipping && !predicate(res.Value()) {
					skipping = false
				}

				if !skipping {
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

// Last creates a Transformer that only emits the last n items from the stream.
// This requires buffering up to n items, so it waits until the stream completes.
// If n <= 0, an empty stream is returned.
func Last[T any](n int) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			if n <= 0 {
				// Drain input
				for range in {
				}
				return
			}

			// Ring buffer for last n items
			buffer := make([]core.Result[T], 0, n)

			for res := range in {
				// Pass through errors and sentinels immediately
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Add to buffer, evicting oldest if full
				if len(buffer) < n {
					buffer = append(buffer, res)
				} else {
					// Shift left and add at end
					copy(buffer, buffer[1:])
					buffer[n-1] = res
				}
			}

			// Emit buffered items
			for _, res := range buffer {
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

// First creates a Transformer that only emits the first item from the stream.
// This is equivalent to Take(1).
func First[T any]() core.Transformer[T, T] {
	return Take[T](1)
}

// Nth creates a Transformer that only emits the nth item (0-indexed) from the stream.
// If the stream has fewer than n+1 items, nothing is emitted.
func Nth[T any](n int) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			if n < 0 {
				return
			}

			count := 0
			for res := range in {
				// Pass through errors and sentinels
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if count == n {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					return
				}
				count++
			}
		}()
		return out
	})
}
