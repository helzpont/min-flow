package flowerrors

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// OnError creates a Transformer that calls a handler function when an error occurs.
// The handler is called for side effects; the error still passes through the stream.
func OnError[T any](handler func(error)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() {
					handler(res.Error())
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

// CatchError creates a Transformer that catches errors matching a predicate and handles them.
// If the handler returns a value, it replaces the error. If the handler returns an error,
// that error propagates. Non-matching errors pass through unchanged.
func CatchError[T any](predicate func(error) bool, handler func(error) (T, error)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() && predicate(res.Error()) {
					value, err := handler(res.Error())
					if err != nil {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[T](err):
						}
					} else {
						select {
						case <-ctx.Done():
							return
						case out <- core.Ok(value):
						}
					}
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

// FilterErrors creates a Transformer that filters out errors matching a predicate.
// Matching errors are silently dropped; non-matching errors pass through.
func FilterErrors[T any](predicate func(error) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Drop matching errors
				if res.IsError() && predicate(res.Error()) {
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

// IgnoreErrors creates a Transformer that drops all error results.
// Only values and sentinels pass through.
func IgnoreErrors[T any]() core.Transformer[T, T] {
	return FilterErrors[T](func(_ error) bool {
		return true
	})
}

// MapErrors creates a Transformer that transforms errors using a mapping function.
// Values and sentinels pass through unchanged.
func MapErrors[T any](mapper func(error) error) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() {
					mappedErr := mapper(res.Error())
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](mappedErr):
					}
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

// WrapError creates a Transformer that wraps all errors with additional context.
func WrapError[T any](wrapper func(error) error) core.Transformer[T, T] {
	return MapErrors[T](wrapper)
}

// ErrorsOnly creates a Transformer that only passes through error results.
// Values and sentinels are dropped. Useful for error-focused processing.
func ErrorsOnly[T any]() core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
				// Drop values and sentinels
			}
		}()

		return out
	})
}

// Materialize creates a Transformer that converts values to a wrapper type containing
// either the value or the error. This is useful for processing both successes and errors uniformly.
type Materialized[T any] struct {
	Value   T
	Err     error
	IsValue bool
}

// Materialize creates a Transformer that wraps all results in a Materialized struct.
// This allows downstream processors to handle both values and errors uniformly.
func Materialize[T any]() core.Transformer[T, Materialized[T]] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[Materialized[T]] {
		out := make(chan *core.Result[Materialized[T]])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[Materialized[T]](res.Error()):
					}
					continue
				}

				var materialized Materialized[T]
				if res.IsError() {
					materialized = Materialized[T]{Err: res.Error(), IsValue: false}
				} else {
					materialized = Materialized[T]{Value: res.Value(), IsValue: true}
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(materialized):
				}
			}
		}()

		return out
	})
}

// Dematerialize creates a Transformer that unwraps Materialized values back to regular results.
func Dematerialize[T any]() core.Transformer[Materialized[T], T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[Materialized[T]]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[T](res.Error()):
					}
					continue
				}

				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](res.Error()):
					}
					continue
				}

				materialized := res.Value()
				if materialized.IsValue {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(materialized.Value):
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](materialized.Err):
					}
				}
			}
		}()

		return out
	})
}

// CountErrors creates a Transformer that counts errors and adds the count to a provided counter.
// Errors still pass through the stream.
func CountErrors[T any](counter *int64) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() {
					*counter++
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

// ThrowOnError creates a Transformer that stops the stream on the first error.
// This is useful when any error should halt processing.
func ThrowOnError[T any]() core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}

				// Stop after emitting the error
				if res.IsError() {
					return
				}
			}
		}()

		return out
	})
}
