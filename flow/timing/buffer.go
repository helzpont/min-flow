package timing

import (
	"context"
	"sync"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// TimingConfig provides configuration for timing transformers.
type TimingConfig struct {
	// BufferSize specifies the default buffer size for buffering operations.
	// A value of 0 or negative will use the function-level default (1).
	BufferSize int
}

// WithBufferSize returns a functional option that sets the buffer size.
func WithBufferSize(size int) func(*TimingConfig) {
	return func(c *TimingConfig) {
		c.BufferSize = size
	}
}

// effectiveBufferSize returns the buffer size to use, considering
// context config and the explicitly provided value.
// If size > 0, it takes precedence. Otherwise, config from context is used.
// If neither provides a valid value, returns 1.
func effectiveBufferSize(ctx context.Context, size int) int {
	if size > 0 {
		return size
	}
	if cfg, ok := core.GetConfig[*TimingConfig](ctx); ok && cfg.BufferSize > 0 {
		return cfg.BufferSize
	}
	return 1
}

// Buffer creates a Transformer with an internal buffer of the specified size.
// This decouples producer and consumer speeds, allowing the producer to continue
// while the consumer processes items.
// If size <= 0, the buffer size is determined from context config or defaults to 1.
func Buffer[T any](size int) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		bufSize := effectiveBufferSize(ctx, size)
		out := make(chan core.Result[T], bufSize)

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

// Debounce creates a Transformer that only emits an item after a specified duration
// has passed without another item arriving. Useful for handling bursts of events
// where only the last event in a burst matters.
func Debounce[T any](duration time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var timer *time.Timer
			var pending core.Result[T]
			var hasPending bool
			var mu sync.Mutex

			timerChan := make(chan struct{})

			for {
				select {
				case <-ctx.Done():
					if timer != nil {
						timer.Stop()
					}
					return

				case res, ok := <-in:
					if !ok {
						// Emit any pending item before closing
						mu.Lock()
						if hasPending {
							select {
							case out <- pending:
							case <-ctx.Done():
							}
						}
						if timer != nil {
							timer.Stop()
						}
						mu.Unlock()
						return
					}

					// Pass through errors and sentinels immediately
					if !res.IsValue() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}

					mu.Lock()
					pending = res
					hasPending = true
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(duration, func() {
						select {
						case timerChan <- struct{}{}:
						default:
						}
					})
					mu.Unlock()

				case <-timerChan:
					mu.Lock()
					if hasPending {
						select {
						case out <- pending:
						case <-ctx.Done():
							mu.Unlock()
							return
						}
						hasPending = false
					}
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// Throttle creates a Transformer that limits emissions to at most one per duration.
// The first item passes through immediately, then subsequent items are dropped
// until the duration has elapsed.
func Throttle[T any](duration time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var lastEmit time.Time

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

				now := time.Now()
				if now.Sub(lastEmit) >= duration {
					lastEmit = now
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
				// Drop items that arrive too quickly
			}
		}()

		return out
	})
}

// ThrottleLatest is like Throttle but keeps the latest item during the throttle period.
// When the period ends, it emits the most recent item received during that period.
func ThrottleLatest[T any](duration time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var pending core.Result[T]
			var hasPending bool
			var mu sync.Mutex
			ticker := time.NewTicker(duration)
			defer ticker.Stop()

			// First value passes through immediately
			firstDone := false

			for {
				select {
				case <-ctx.Done():
					return

				case res, ok := <-in:
					if !ok {
						// Emit any pending before closing
						mu.Lock()
						if hasPending {
							select {
							case out <- pending:
							case <-ctx.Done():
							}
						}
						mu.Unlock()
						return
					}

					if !res.IsValue() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}

					if !firstDone {
						firstDone = true
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					} else {
						mu.Lock()
						pending = res
						hasPending = true
						mu.Unlock()
					}

				case <-ticker.C:
					mu.Lock()
					if hasPending {
						select {
						case out <- pending:
						case <-ctx.Done():
							mu.Unlock()
							return
						}
						hasPending = false
					}
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// RateLimit creates a Transformer that limits the rate of emissions to n items per duration.
// Uses a token bucket algorithm. Items that would exceed the rate are delayed.
func RateLimit[T any](n int, per time.Duration) core.Transformer[T, T] {
	if n <= 0 {
		n = 1
	}

	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			tokens := n
			var mu sync.Mutex

			// Refill tokens periodically
			ticker := time.NewTicker(per)
			defer ticker.Stop()

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						mu.Lock()
						tokens = n
						mu.Unlock()
					}
				}
			}()

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

				// Wait for a token
				for {
					mu.Lock()
					if tokens > 0 {
						tokens--
						mu.Unlock()
						break
					}
					mu.Unlock()

					select {
					case <-ctx.Done():
						return
					case <-time.After(per / time.Duration(n)):
						// Check again after a small delay
					}
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

// Delay creates a Transformer that delays each item by the specified duration.
func Delay[T any](duration time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				case <-time.After(duration):
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

// Sample creates a Transformer that emits the most recent item at regular intervals.
// Items received between intervals are discarded except for the latest.
func Sample[T any](interval time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var latest core.Result[T]
			var hasLatest bool
			var mu sync.Mutex
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			done := make(chan struct{})

			go func() {
				for res := range in {
					if !res.IsValue() {
						select {
						case out <- res:
						case <-ctx.Done():
							return
						}
						continue
					}

					mu.Lock()
					latest = res
					hasLatest = true
					mu.Unlock()
				}
				close(done)
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case <-done:
					// Emit final sample if any
					mu.Lock()
					if hasLatest {
						select {
						case out <- latest:
						case <-ctx.Done():
						}
					}
					mu.Unlock()
					return
				case <-ticker.C:
					mu.Lock()
					if hasLatest {
						select {
						case out <- latest:
						case <-ctx.Done():
							mu.Unlock()
							return
						}
						hasLatest = false
					}
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// After creates a Transformer that errors if no item is received within the duration.
// The timeout resets after each item. If the timeout expires, an error is emitted.
func After[T any](duration time.Duration) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			timer := time.NewTimer(duration)
			defer timer.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-timer.C:
					select {
					case out <- core.Err[T](ErrTimeout):
					case <-ctx.Done():
					}
					return

				case res, ok := <-in:
					if !ok {
						return
					}

					timer.Reset(duration)

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

// ErrTimeout is returned when a Timeout transformer's deadline expires.
var ErrTimeout = timeoutError{}

type timeoutError struct{}

func (timeoutError) Error() string   { return "stream timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
