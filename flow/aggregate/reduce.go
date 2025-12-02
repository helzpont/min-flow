package aggregate

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Reduce creates a Transformer that reduces all items in the stream to a single value
// using the provided reducer function. The reducer takes the accumulated value and the
// current item, returning the new accumulated value.
// The first item becomes the initial accumulator value.
// If the stream is empty, nothing is emitted.
func Reduce[T any](reducer func(acc, item T) T) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			var acc T
			hasAcc := false

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if !hasAcc {
					acc = res.Value()
					hasAcc = true
				} else {
					acc = reducer(acc, res.Value())
				}
			}

			if hasAcc {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(acc):
				}
			}
		}()
		return out
	})
}

// Fold creates a Transformer that folds all items in the stream into a single value
// using the provided folder function and initial value.
// Unlike Reduce, Fold always emits a value (the initial value if stream is empty).
func Fold[T, R any](initial R, folder func(acc R, item T) R) core.Transformer[T, R] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[R] {
		out := make(chan *core.Result[R])
		go func() {
			defer close(out)
			acc := initial

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[R](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[R](res.Error()):
					}
					continue
				}

				acc = folder(acc, res.Value())
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(acc):
			}
		}()
		return out
	})
}

// Scan creates a Transformer that emits each intermediate accumulated value.
// Like Fold, but emits after each item rather than only at the end.
// The initial value is NOT emitted - only values after processing items.
func Scan[T, R any](initial R, scanner func(acc R, item T) R) core.Transformer[T, R] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[R] {
		out := make(chan *core.Result[R])
		go func() {
			defer close(out)
			acc := initial

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[R](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[R](res.Error()):
					}
					continue
				}

				acc = scanner(acc, res.Value())
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(acc):
				}
			}
		}()
		return out
	})
}

// Count creates a Transformer that counts the number of items in the stream.
// Emits a single int value when the stream completes.
func Count[T any]() core.Transformer[T, int] {
	return Fold[T, int](0, func(acc int, _ T) int {
		return acc + 1
	})
}

// Sum creates a Transformer that sums numeric values in the stream.
// Works with any numeric type that supports addition.
// Emits a single value when the stream completes.
func Sum[T Numeric]() core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			var sum T

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				sum += res.Value()
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(sum):
			}
		}()
		return out
	})
}

// Numeric is a constraint for numeric types that support arithmetic operations.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Average creates a Transformer that calculates the average of numeric values.
// Emits a single float64 value when the stream completes.
// If the stream is empty, emits 0.
func Average[T Numeric]() core.Transformer[T, float64] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[float64] {
		out := make(chan *core.Result[float64])
		go func() {
			defer close(out)
			var sum float64
			count := 0

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[float64](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[float64](res.Error()):
					}
					continue
				}

				sum += float64(res.Value())
				count++
			}

			var avg float64
			if count > 0 {
				avg = sum / float64(count)
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(avg):
			}
		}()
		return out
	})
}

// Min creates a Transformer that finds the minimum value in the stream.
// Uses the provided less function to compare values.
// If the stream is empty, nothing is emitted.
func Min[T any](less func(a, b T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			var min T
			hasMin := false

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if !hasMin || less(res.Value(), min) {
					min = res.Value()
					hasMin = true
				}
			}

			if hasMin {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(min):
				}
			}
		}()
		return out
	})
}

// Max creates a Transformer that finds the maximum value in the stream.
// Uses the provided less function to compare values (finds the value where !less(result, x) for all x).
// If the stream is empty, nothing is emitted.
func Max[T any](less func(a, b T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			var max T
			hasMax := false

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if !hasMax || less(max, res.Value()) {
					max = res.Value()
					hasMax = true
				}
			}

			if hasMax {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(max):
				}
			}
		}()
		return out
	})
}

// All creates a Transformer that checks if all items satisfy the predicate.
// Emits true if all items match (or if stream is empty), false otherwise.
// Short-circuits on first non-matching item.
func All[T any](predicate func(T) bool) core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)
			result := true

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[bool](res.Error()):
					}
					continue
				}

				if !predicate(res.Value()) {
					result = false
					// Drain remaining input
					for range in {
					}
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(result):
			}
		}()
		return out
	})
}

// Any creates a Transformer that checks if any item satisfies the predicate.
// Emits true if at least one item matches, false otherwise.
// Short-circuits on first matching item.
func Any[T any](predicate func(T) bool) core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)
			result := false

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[bool](res.Error()):
					}
					continue
				}

				if predicate(res.Value()) {
					result = true
					// Drain remaining input
					for range in {
					}
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(result):
			}
		}()
		return out
	})
}

// None creates a Transformer that checks if no items satisfy the predicate.
// Emits true if no items match (or if stream is empty), false otherwise.
// Short-circuits on first matching item.
func None[T any](predicate func(T) bool) core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)
			result := true

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[bool](res.Error()):
					}
					continue
				}

				if predicate(res.Value()) {
					result = false
					// Drain remaining input
					for range in {
					}
					break
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(result):
			}
		}()
		return out
	})
}
