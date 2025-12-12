// Package operators provides stream transformation operators for the min-flow framework.
// This file contains additional time-based operators for temporal control of streams.
// Note: Basic time operators (Delay, Debounce, Throttle, Timeout, RateLimit, Sample)
// are in buffer.go.
package timing

import (
	"context"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// ThrottleWithTrailing creates a Transformer that limits emissions to at most one per duration,
// but also emits the last item when the throttle window closes if items were dropped.
func ThrottleWithTrailing[T any](d time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			var (
				lastEmit    time.Time
				trailing    core.Result[T]
				hasTrailing bool
				timer       *time.Timer
				timerC      <-chan time.Time
				inputClosed bool
			)

			for {
				select {
				case <-ctx.Done():
					if timer != nil {
						timer.Stop()
					}
					return

				case res, ok := <-in:
					if !ok {
						inputClosed = true
						// Emit trailing item if pending
						if hasTrailing {
							select {
							case <-ctx.Done():
							case out <- trailing:
							}
						}
						if timer != nil {
							timer.Stop()
						}
						return
					}

					now := time.Now()
					if now.Sub(lastEmit) >= d {
						// Window expired - emit immediately
						lastEmit = now
						hasTrailing = false
						if timer != nil {
							timer.Stop()
							timerC = nil
						}
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					} else {
						// Within window - save as trailing
						trailing = res
						hasTrailing = true
						if timer == nil {
							remaining := d - now.Sub(lastEmit)
							timer = time.NewTimer(remaining)
							timerC = timer.C
						}
					}

				case <-timerC:
					if inputClosed {
						return
					}
					// Window expired - emit trailing if exists
					if hasTrailing {
						lastEmit = time.Now()
						select {
						case <-ctx.Done():
							return
						case out <- trailing:
						}
						hasTrailing = false
					}
					timer = nil
					timerC = nil
				}
			}
		}()
		return out
	})
}

// AfterWithError creates a Transformer that emits a custom error if no item
// is received within the specified duration. This extends After by allowing
// a custom error type.
func AfterWithError[T any](d time.Duration, timeoutErr error) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)

			timer := time.NewTimer(d)
			defer timer.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case res, ok := <-in:
					if !ok {
						return
					}
					// Reset the timer on each item
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(d)

					select {
					case <-ctx.Done():
						return
					case out <- res:
					}

				case <-timer.C:
					// Timeout exceeded - emit error and close
					select {
					case <-ctx.Done():
					case out <- core.Err[T](timeoutErr):
					}
					return
				}
			}
		}()
		return out
	})
}

// Interval creates an Emitter that emits sequential integers at fixed time intervals.
// The first value (0) is emitted after the first interval.
// The stream continues until context cancellation.
func Interval(d time.Duration) *core.Emitter[int] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			ticker := time.NewTicker(d)
			defer ticker.Stop()

			i := 0
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(i):
						i++
					}
				}
			}
		}()
		return out
	})
}

// Once creates an Emitter that emits a single value (0) after the specified duration,
// then completes.
func Once(d time.Duration) *core.Emitter[int] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			timer := time.NewTimer(d)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(0):
				}
			}
		}()
		return out
	})
}

// OnceWith creates an Emitter that emits the specified value after the
// specified duration, then completes.
func OnceWith[T any](d time.Duration, value T) *core.Emitter[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])
		go func() {
			defer close(out)
			timer := time.NewTimer(d)
			defer timer.Stop()

			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}
		}()
		return out
	})
}

// Timestamped wraps each item with its emission timestamp.
type Timestamped[T any] struct {
	Value     T
	Timestamp time.Time
}

// Stamped creates a Transformer that wraps each item with the time it was received.
func Stamped[T any]() core.Transformer[T, Timestamped[T]] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[Timestamped[T]] {
		out := make(chan core.Result[Timestamped[T]])
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

					now := time.Now()
					var outRes core.Result[Timestamped[T]]
					if res.IsError() {
						outRes = core.Err[Timestamped[T]](res.Error())
					} else if res.IsSentinel() {
						outRes = core.Sentinel[Timestamped[T]](res.Error())
					} else {
						outRes = core.Ok(Timestamped[T]{
							Value:     res.Value(),
							Timestamp: now,
						})
					}

					select {
					case <-ctx.Done():
						return
					case out <- outRes:
					}
				}
			}
		}()
		return out
	})
}

// TimeInterval represents the interval between consecutive emissions.
type TimeInterval[T any] struct {
	Value    T
	Interval time.Duration
}

// Elapsed creates a Transformer that wraps each item with the duration since
// the previous emission (or since stream start for the first item).
func Elapsed[T any]() core.Transformer[T, TimeInterval[T]] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[TimeInterval[T]] {
		out := make(chan core.Result[TimeInterval[T]])
		go func() {
			defer close(out)

			lastTime := time.Now()

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					now := time.Now()
					interval := now.Sub(lastTime)
					lastTime = now

					var outRes core.Result[TimeInterval[T]]
					if res.IsError() {
						outRes = core.Err[TimeInterval[T]](res.Error())
					} else if res.IsSentinel() {
						outRes = core.Sentinel[TimeInterval[T]](res.Error())
					} else {
						outRes = core.Ok(TimeInterval[T]{
							Value:    res.Value(),
							Interval: interval,
						})
					}

					select {
					case <-ctx.Done():
						return
					case out <- outRes:
					}
				}
			}
		}()
		return out
	})
}

// DelayWhen creates a Transformer that delays each item by a duration determined
// by the provided function. This allows dynamic delay based on item value.
func DelayWhen[T any](delayFn func(T) time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
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

					// Pass through errors without delay
					if res.IsError() || res.IsSentinel() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}

					// Calculate delay based on value
					delay := delayFn(res.Value())
					if delay > 0 {
						timer := time.NewTimer(delay)
						select {
						case <-ctx.Done():
							timer.Stop()
							return
						case <-timer.C:
						}
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
