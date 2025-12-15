package flow

import (
	"context"
	"iter"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// FromSlice creates a Stream that emits each element from the given slice.
// The stream completes after all elements have been emitted.
// Uses buffered channels to reduce goroutine synchronization overhead.
func FromSlice[T any](items []T) Stream[T] {
	const maxBufferSize = 512

	return Emit(func(ctx context.Context) <-chan Result[T] {
		// For small slices, use a fully-buffered channel (no goroutine needed)
		if len(items) <= maxBufferSize {
			out := make(chan Result[T], len(items))
			for _, item := range items {
				out <- Ok(item)
			}
			close(out)
			return out
		}

		// For larger slices, use a buffered channel with a goroutine
		out := make(chan Result[T], maxBufferSize)
		go func() {
			defer close(out)
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(item):
				}
			}
		}()
		return out
	})
}

// FromChannel creates a Stream that emits values received from the given channel.
// The stream completes when the input channel is closed.
// The caller is responsible for closing the input channel.
func FromChannel[T any](ch <-chan T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-ch:
					if !ok {
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- Ok(item):
					}
				}
			}
		}()
		return out
	})
}

// FromIter creates a Stream from a Go 1.23+ iterator sequence.
// The stream completes when the iterator is exhausted.
func FromIter[T any](seq iter.Seq[T]) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			for item := range seq {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(item):
				}
			}
		}()
		return out
	})
}

// Empty creates a Stream that emits no values and completes immediately.
func Empty[T any]() Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		close(out)
		return out
	})
}

// Once creates a Stream that emits a single value and then completes.
func Once[T any](value T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			select {
			case <-ctx.Done():
				return
			case out <- Ok(value):
			}
		}()
		return out
	})
}

// Generate creates a Stream that lazily generates values using the provided function.
// The function should return the next value and true to continue, or zero value and
// false to signal completion. If the function returns an error, it is wrapped in
// an error Result and the stream continues.
func Generate[T any](fn func() (T, bool, error)) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			for {
				value, ok, err := fn()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- Ok(value):
				}
			}
		}()
		return out
	})
}

// Repeat creates a Stream that emits the same value n times.
// If n is negative, the stream repeats indefinitely until context cancellation.
func Repeat[T any](value T, n int) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			count := 0
			for n < 0 || count < n {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(value):
					count++
				}
			}
		}()
		return out
	})
}

// Range creates a Stream that emits integers from start (inclusive) to end (exclusive).
// If start >= end, an empty stream is returned.
func Range(start, end int) Stream[int] {
	return Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int])
		go func() {
			defer close(out)
			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(i):
				}
			}
		}()
		return out
	})
}

// Timer creates a Stream that emits a single value after the specified delay.
// The value emitted is the current time when the timer fires.
func Timer(delay time.Duration) Stream[time.Time] {
	return Emit(func(ctx context.Context) <-chan Result[time.Time] {
		out := make(chan Result[time.Time])
		go func() {
			defer close(out)
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case t := <-timer.C:
				select {
				case <-ctx.Done():
					return
				case out <- Ok(t):
				}
			}
		}()
		return out
	})
}

// TimerValue creates a Stream that emits the specified value after the delay.
func TimerValue[T any](delay time.Duration, value T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				select {
				case <-ctx.Done():
					return
				case out <- Ok(value):
				}
			}
		}()
		return out
	})
}

// Interval creates a Stream that emits sequential integers at the specified interval.
// The first value (0) is emitted after the initial delay.
// The stream continues indefinitely until context cancellation.
func Interval(period time.Duration) Stream[int] {
	return IntervalWithDelay(period, period)
}

// IntervalWithDelay creates a Stream that emits sequential integers at the specified interval.
// The first value (0) is emitted after the initial delay.
// The stream continues indefinitely until context cancellation.
func IntervalWithDelay(initialDelay, period time.Duration) Stream[int] {
	return Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int])
		go func() {
			defer close(out)

			// Initial delay
			if initialDelay > 0 {
				timer := time.NewTimer(initialDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}

			// Emit first value
			select {
			case <-ctx.Done():
				return
			case out <- Ok(0):
			}

			// Continue with ticker
			ticker := time.NewTicker(period)
			defer ticker.Stop()

			count := 1
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					select {
					case <-ctx.Done():
						return
					case out <- Ok(count):
						count++
					}
				}
			}
		}()
		return out
	})
}

// KeyValue represents a key-value pair from a map.
type KeyValue[K comparable, V any] struct {
	Key   K
	Value V
}

// FromMap creates a Stream that emits key-value pairs from the given map.
// The order of emission is non-deterministic (as per Go map iteration).
func FromMap[K comparable, V any](m map[K]V) Stream[KeyValue[K, V]] {
	return Emit(func(ctx context.Context) <-chan Result[KeyValue[K, V]] {
		out := make(chan Result[KeyValue[K, V]])
		go func() {
			defer close(out)
			for k, v := range m {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(KeyValue[K, V]{Key: k, Value: v}):
				}
			}
		}()
		return out
	})
}

// FromError creates a Stream that immediately emits an error and completes.
func FromError[T any](err error) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			select {
			case <-ctx.Done():
				return
			case out <- core.Err[T](err):
			}
		}()
		return out
	})
}

// Never creates a Stream that never emits any values and never completes.
// The stream only terminates when the context is cancelled.
func Never[T any]() Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			<-ctx.Done()
		}()
		return out
	})
}

// Defer creates a Stream lazily, calling the factory function each time
// the stream is subscribed to. This allows for late binding of stream creation.
func Defer[T any](factory func() Stream[T]) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		stream := factory()
		return stream.Emit(ctx)
	})
}

// Create creates a Stream using a custom emitter function.
// The emitter function receives an emit callback to produce values.
// Return an error from the emitter to emit an error and continue,
// or call the provided done function to complete the stream.
func Create[T any](emitter func(ctx context.Context, emit func(T), emitError func(error)) error) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)

			emit := func(value T) {
				select {
				case <-ctx.Done():
				case out <- Ok(value):
				}
			}

			emitError := func(err error) {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
			}

			if err := emitter(ctx, emit, emitError); err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
			}
		}()
		return out
	})
}

// RangeStep creates a Stream that emits integers from start to end with the given step.
// If step is positive, emits start, start+step, start+2*step, ... (while < end)
// If step is negative, emits start, start+step, start+2*step, ... (while > end)
// If step is zero or the direction is invalid, an empty stream is returned.
func RangeStep(start, end, step int) Stream[int] {
	return Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int])
		go func() {
			defer close(out)

			// Validate step direction
			if step == 0 {
				return
			}
			if step > 0 && start >= end {
				return
			}
			if step < 0 && start <= end {
				return
			}

			for i := start; (step > 0 && i < end) || (step < 0 && i > end); i += step {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(i):
				}
			}
		}()
		return out
	})
}

// Concat creates a Stream that emits all values from the first stream,
// then all values from the second stream, and so on.
func Concat[T any](streams ...Stream[T]) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			for _, stream := range streams {
				for res := range stream.Emit(ctx) {
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

// StartWith creates a Transformer that prepends values before the source stream.
func StartWith[T any](values ...T) Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)

			// Emit prepended values first
			for _, v := range values {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(v):
				}
			}

			// Then pass through source values
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

// EndWith creates a Transformer that appends values after the source stream completes.
func EndWith[T any](values ...T) Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)

			// Pass through source values first
			for res := range in {
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}

			// Emit appended values after source completes
			for _, v := range values {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(v):
				}
			}
		}()
		return out
	})
}

// FromFunc creates a Stream by calling a function that returns values one at a time.
// Each call to the function should return the next value. The stream continues
// until the function returns an error, which is NOT emitted (use FromFuncWithError for that).
// Pass nil as the "done" error to indicate completion without error.
func FromFunc[T any](fn func() (T, error)) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			for {
				value, err := fn()
				if err != nil {
					return // Function signals completion
				}
				select {
				case <-ctx.Done():
					return
				case out <- Ok(value):
				}
			}
		}()
		return out
	})
}

// Unfold creates a Stream by unfolding a seed value.
// The function receives the current state and returns:
// - The value to emit
// - The next state
// - Whether to continue (false = complete)
// - An error (if any, emitted as an error result)
func Unfold[T, S any](seed S, fn func(S) (T, S, bool, error)) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			state := seed
			for {
				value, nextState, ok, err := fn(state)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					state = nextState
					continue
				}
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- Ok(value):
				}
				state = nextState
			}
		}()
		return out
	})
}

// Iterate creates a Stream by repeatedly applying a function to a value.
// Emits seed, fn(seed), fn(fn(seed)), ... indefinitely until context cancellation.
func Iterate[T any](seed T, fn func(T) T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			current := seed
			for {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(current):
					current = fn(current)
				}
			}
		}()
		return out
	})
}

// IterateN creates a Stream that emits seed, fn(seed), fn(fn(seed)), ... for n iterations.
func IterateN[T any](seed T, fn func(T) T, n int) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		out := make(chan Result[T])
		go func() {
			defer close(out)
			current := seed
			for i := 0; i < n; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(current):
					current = fn(current)
				}
			}
		}()
		return out
	})
}
