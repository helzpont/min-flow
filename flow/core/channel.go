package core

import (
	"context"
	"iter"
)

// EmitFunc is the function signature for producing a stream of results.
type EmitFunc[OUT any] func(context.Context) <-chan Result[OUT]

// Emitter represents a producer of stream data. It implements Stream.
// Unlike the function-type approach, this struct-based implementation
// is more idiomatic Go and allows for future extensibility.
type Emitter[OUT any] struct {
	fn EmitFunc[OUT]
}

// Emit creates an Emitter from a channel-producing function.
func Emit[OUT any](emitter func(context.Context) <-chan Result[OUT]) *Emitter[OUT] {
	return &Emitter[OUT]{fn: emitter}
}

// Emit produces the channel of results. Implements Stream interface.
func (e *Emitter[OUT]) Emit(ctx context.Context) <-chan Result[OUT] {
	return e.fn(ctx)
}

// Collect gathers all results into a slice.
func (e *Emitter[OUT]) Collect(ctx context.Context) []Result[OUT] {
	return Collect(ctx, e)
}

// All returns an iterator over all results.
func (e *Emitter[OUT]) All(ctx context.Context) iter.Seq[Result[OUT]] {
	return All(ctx, e)
}

// TransmitFunc is the function signature for transforming a channel.
type TransmitFunc[IN, OUT any] func(context.Context, <-chan Result[IN]) <-chan Result[OUT]

// Transmitter represents a channel-level transformation. It implements Transformer.
type Transmitter[IN, OUT any] struct {
	fn TransmitFunc[IN, OUT]
}

// Transmit creates a Transmitter from a channel transformation function.
func Transmit[IN, OUT any](transmitter func(context.Context, <-chan Result[IN]) <-chan Result[OUT]) *Transmitter[IN, OUT] {
	return &Transmitter[IN, OUT]{fn: transmitter}
}

// Apply transforms a stream. Implements Transformer interface.
func (t *Transmitter[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		return t.fn(ctx, in.Emit(ctx))
	})
}

// Intercept creates a Transmitter that invokes interceptors for each item.
// This is useful for adding explicit observation points in pipelines that
// don't use transformers (which auto-invoke interceptors), or for observing
// the raw output of stream sources.
//
// Note: Mapper, FlatMapper, and IterFlatMapper automatically invoke interceptors,
// so Intercept is typically only needed at the start of a pipeline or between
// non-transformer operations.
//
// Uses DefaultBufferSize (64) to balance throughput and backpressure.
// For strict backpressure (unbuffered), use InterceptBuffered(0).
func Intercept[T any]() *Transmitter[T, T] {
	return InterceptBuffered[T](DefaultBufferSize)
}

// InterceptBuffered creates an Intercept transmitter with buffered output channel.
// A buffer size of 0 uses an unbuffered channel for strict backpressure.
// Larger buffers reduce goroutine synchronization overhead at the cost of memory.
//
// Recommended buffer sizes:
//   - 0: Strict backpressure, highest latency
//   - 16-64: Good balance for most use cases (Intercept uses 64 by default)
//   - 256+: High-throughput scenarios
func InterceptBuffered[T any](bufferSize int) *Transmitter[T, T] {
	return Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T], bufferSize)

		go func() {
			defer close(out)

			dispatch := newInterceptorDispatch(ctx)

			// Early exit: if no interceptors, just pass through without overhead
			if !dispatch.hasAny {
				for res := range in {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
				return
			}

			// Invoke stream start interceptors
			dispatch.invokeNoArg(ctx, StreamStart)
			defer dispatch.invokeNoArg(ctx, StreamEnd)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Invoke item-level interceptors
				dispatch.invokeOneArg(ctx, ItemReceived, res)
				dispatch.invokeResult(ctx, toAnyResult(res))

				select {
				case <-ctx.Done():
					return
				case out <- res:
					dispatch.invokeOneArg(ctx, ItemEmitted, res)
				}
			}
		}()

		return out
	})
}
