package flowerrors

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// ErrMaxRetries is returned when the maximum number of retries has been exceeded.
var ErrMaxRetries = errors.New("max retries exceeded")

// ErrCircuitOpen is returned when a circuit breaker is in the open state.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Retry creates a Transformer that retries failed items up to maxRetries times.
// If an item still fails after all retries, the error is passed through.
// Only items that result in errors are retried; successful items and sentinels pass through.
func Retry[T any](maxRetries int, operation func(T) (T, error)) core.Transformer[T, T] {
	if maxRetries < 0 {
		maxRetries = 0
	}

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

				// Pass through errors and sentinels without processing
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Try the operation with retries
				value := res.Value()
				var lastErr error
				var result T
				success := false

				for attempt := 0; attempt <= maxRetries; attempt++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					result, lastErr = operation(value)
					if lastErr == nil {
						success = true
						break
					}
				}

				if success {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(result):
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](lastErr):
					}
				}
			}
		}()

		return out
	})
}

// BackoffStrategy defines how to calculate delay between retries.
type BackoffStrategy func(attempt int) time.Duration

// ConstantBackoff returns a BackoffStrategy that always waits the same duration.
func ConstantBackoff(delay time.Duration) BackoffStrategy {
	return func(attempt int) time.Duration {
		return delay
	}
}

// LinearBackoff returns a BackoffStrategy that increases delay linearly.
func LinearBackoff(initialDelay time.Duration) BackoffStrategy {
	return func(attempt int) time.Duration {
		return time.Duration(attempt+1) * initialDelay
	}
}

// ExponentialBackoff returns a BackoffStrategy that doubles delay each attempt.
// The delay is capped at maxDelay if provided (use 0 for no cap).
func ExponentialBackoff(initialDelay, maxDelay time.Duration) BackoffStrategy {
	return func(attempt int) time.Duration {
		delay := initialDelay * time.Duration(math.Pow(2, float64(attempt)))
		if maxDelay > 0 && delay > maxDelay {
			return maxDelay
		}
		return delay
	}
}

// RetryWithBackoff creates a Transformer that retries failed items with configurable backoff.
// The backoff strategy determines the delay between retries.
func RetryWithBackoff[T any](maxRetries int, backoff BackoffStrategy, operation func(T) (T, error)) core.Transformer[T, T] {
	if maxRetries < 0 {
		maxRetries = 0
	}

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

				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				value := res.Value()
				var lastErr error
				var result T
				success := false

				for attempt := 0; attempt <= maxRetries; attempt++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					result, lastErr = operation(value)
					if lastErr == nil {
						success = true
						break
					}

					// Apply backoff delay before next retry (except for last attempt)
					if attempt < maxRetries {
						delay := backoff(attempt)
						select {
						case <-ctx.Done():
							return
						case <-time.After(delay):
						}
					}
				}

				if success {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(result):
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](lastErr):
					}
				}
			}
		}()

		return out
	})
}

// RetryWhen creates a Transformer that retries based on a predicate function.
// The predicate receives the error and attempt number (0-indexed) and returns true to retry.
// If the predicate returns false or maxRetries is exceeded, the error is passed through.
func RetryWhen[T any](maxRetries int, shouldRetry func(err error, attempt int) bool, operation func(T) (T, error)) core.Transformer[T, T] {
	if maxRetries < 0 {
		maxRetries = 0
	}

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

				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				value := res.Value()
				var lastErr error
				var result T
				success := false

				for attempt := 0; attempt <= maxRetries; attempt++ {
					select {
					case <-ctx.Done():
						return
					default:
					}

					result, lastErr = operation(value)
					if lastErr == nil {
						success = true
						break
					}

					// Check if we should retry
					if attempt < maxRetries && !shouldRetry(lastErr, attempt) {
						break
					}
				}

				if success {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(result):
					}
				} else {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](lastErr):
					}
				}
			}
		}()

		return out
	})
}

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker wraps an operation with circuit breaker pattern.
// - failureThreshold: number of failures before opening the circuit
// - resetTimeout: duration to wait before trying half-open state
// - halfOpenSuccesses: number of successes in half-open before fully closing
type CircuitBreaker[T any] struct {
	operation         func(T) (T, error)
	failureThreshold  int
	resetTimeout      time.Duration
	halfOpenSuccesses int
	state             CircuitState
	failures          int
	successes         int
	lastFailure       time.Time
	mu                sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker[T any](
	operation func(T) (T, error),
	failureThreshold int,
	resetTimeout time.Duration,
	halfOpenSuccesses int,
) *CircuitBreaker[T] {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	if halfOpenSuccesses <= 0 {
		halfOpenSuccesses = 1
	}

	return &CircuitBreaker[T]{
		operation:         operation,
		failureThreshold:  failureThreshold,
		resetTimeout:      resetTimeout,
		halfOpenSuccesses: halfOpenSuccesses,
		state:             CircuitClosed,
	}
}

// Execute runs the operation through the circuit breaker.
func (cb *CircuitBreaker[T]) Execute(value T) (T, error) {
	cb.mu.Lock()
	state := cb.state

	// Check if we should transition from open to half-open
	if state == CircuitOpen && time.Since(cb.lastFailure) >= cb.resetTimeout {
		cb.state = CircuitHalfOpen
		cb.successes = 0
		state = CircuitHalfOpen
	}

	if state == CircuitOpen {
		cb.mu.Unlock()
		var zero T
		return zero, ErrCircuitOpen
	}

	cb.mu.Unlock()

	// Execute the operation
	result, err := cb.operation(value)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.state == CircuitHalfOpen {
			// Any failure in half-open goes back to open
			cb.state = CircuitOpen
		} else if cb.failures >= cb.failureThreshold {
			cb.state = CircuitOpen
		}

		return result, err
	}

	// Success
	if cb.state == CircuitHalfOpen {
		cb.successes++
		if cb.successes >= cb.halfOpenSuccesses {
			cb.state = CircuitClosed
			cb.failures = 0
		}
	} else {
		// Reset failures on success in closed state
		cb.failures = 0
	}

	return result, nil
}

// State returns the current circuit state.
func (cb *CircuitBreaker[T]) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// WithCircuitBreaker creates a Transformer that applies a circuit breaker to operations.
func WithCircuitBreaker[T any](cb *CircuitBreaker[T]) core.Transformer[T, T] {
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

				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				result, err := cb.Execute(res.Value())
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
					case out <- core.Ok(result):
					}
				}
			}
		}()

		return out
	})
}

// Fallback creates a Transformer that provides a fallback value when an error occurs.
// The fallback function receives the original value and the error.
func Fallback[T any](fallbackFn func(T, error) T) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])

		go func() {
			defer close(out)

			var lastValue T
			hasValue := false

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
					case out <- res:
					}
					continue
				}

				if res.IsError() {
					if hasValue {
						fallbackValue := fallbackFn(lastValue, res.Error())
						select {
						case <-ctx.Done():
							return
						case out <- core.Ok(fallbackValue):
						}
					} else {
						// No previous value, can't compute fallback - pass error through
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
					continue
				}

				lastValue = res.Value()
				hasValue = true
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

// FallbackValue creates a Transformer that replaces errors with a default value.
func FallbackValue[T any](defaultValue T) core.Transformer[T, T] {
	return Fallback(func(_ T, _ error) T {
		return defaultValue
	})
}

// Recover creates a Transformer that recovers from errors using a recovery function.
// The recovery function can return a new value or return an error to propagate.
func Recover[T any](recoverFn func(error) (T, error)) core.Transformer[T, T] {
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

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				if res.IsError() {
					recovered, err := recoverFn(res.Error())
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
						case out <- core.Ok(recovered):
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

// RecoverPanic creates a Transformer that specifically recovers from panic errors.
// It checks if the error is an ErrPanic and applies the recovery function.
func RecoverPanic[T any](recoverFn func(panicValue any) (T, error)) core.Transformer[T, T] {
	return Recover(func(err error) (T, error) {
		var panicErr core.ErrPanic
		if errors.As(err, &panicErr) {
			return recoverFn(panicErr.Value)
		}
		var zero T
		return zero, err
	})
}
