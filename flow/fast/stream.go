package fast

import (
	"context"
	"iter"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize matches core.DefaultBufferSize for fair benchmarking.
const DefaultBufferSize = core.DefaultBufferSize

// Stream represents a minimal stream of values without Result wrapping.
// Values flow directly through channels without error handling overhead.
type Stream[T any] interface {
	Emit(context.Context) <-chan T
}

// Transformer converts a Stream[IN] to a Stream[OUT].
type Transformer[IN, OUT any] interface {
	Apply(Stream[IN]) Stream[OUT]
}

// Emitter is a function that produces a channel of values.
type Emitter[T any] func(context.Context) <-chan T

// Emit implements Stream.
func (e Emitter[T]) Emit(ctx context.Context) <-chan T {
	return e(ctx)
}

// FromSlice creates a stream from a slice.
// Unlike flow.FromSlice, values are sent directly without Result wrapping.
func FromSlice[T any](data []T) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			for _, v := range data {
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}()
		return out
	})
}

// FromChannel wraps an existing channel as a Stream.
func FromChannel[T any](ch <-chan T) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			for v := range ch {
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}()
		return out
	})
}

// FromIter creates a stream from a Go 1.23+ iterator.
func FromIter[T any](seq iter.Seq[T]) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			for v := range seq {
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}()
		return out
	})
}

// Range creates a stream of integers from start to end (exclusive).
func Range(start, end int) Stream[int] {
	return Emitter[int](func(ctx context.Context) <-chan int {
		out := make(chan int, DefaultBufferSize)
		go func() {
			defer close(out)
			for i := start; i < end; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- i:
				}
			}
		}()
		return out
	})
}

// All returns an iterator over stream values.
func All[T any](ctx context.Context, s Stream[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s.Emit(ctx) {
			if !yield(v) {
				return
			}
		}
	}
}
