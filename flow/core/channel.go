package core

import (
	"context"
	"iter"
)

// Emitter represents a function that produces a stream of results of type OUT.
// It is a level of abstraction over channels, just under Stream. Emitters
// answer the question: "How is the stream's data produced?".
type Emitter[OUT any] func(context.Context) <-chan Result[OUT]

func Emit[OUT any](emitter func(context.Context) <-chan Result[OUT]) Emitter[OUT] {
	return emitter
}

func (e Emitter[OUT]) Emit(ctx context.Context) <-chan Result[OUT] {
	return e(ctx)
}

func (e Emitter[OUT]) Collect(ctx context.Context) []Result[OUT] {
	return Collect(ctx, e)
}

func (e Emitter[OUT]) All(ctx context.Context) iter.Seq[Result[OUT]] {
	return All(ctx, e)
}

// Transmitter represents a function that transforms a stream of results
// of type IN into a stream of results of type OUT. It is a level of abstraction
// over channels, just under Transformer. Transmitters answer the question:
// "How is the stream's data transformed?".
type Transmitter[IN, OUT any] func(context.Context, <-chan Result[IN]) <-chan Result[OUT]

func Transmit[IN, OUT any](transmitter func(context.Context, <-chan Result[IN]) <-chan Result[OUT]) Transmitter[IN, OUT] {
	return transmitter
}

func (t Transmitter[IN, OUT]) Apply(in Stream[IN]) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		return t(ctx, in.Emit(ctx))
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
func Intercept[T any]() Transmitter[T, T] {
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
func InterceptBuffered[T any](bufferSize int) Transmitter[T, T] {
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
