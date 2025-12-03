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

func (t Transmitter[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		return t(ctx, in.Emit(ctx))
	})
}

// Intercept creates a Transmitter that invokes interceptors for each item.
// This is a low-level primitive used to enable interceptor-based observation
// and error handling without creating explicit transformer stages.
func Intercept[T any]() Transmitter[T, T] {
	return Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T])

		go func() {
			defer close(out)

			// Invoke stream start interceptors
			_ = InvokeInterceptors(ctx, StreamStart)

			defer func() {
				// Invoke stream end interceptors
				_ = InvokeInterceptors(ctx, StreamEnd)
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Invoke item-level interceptors
				_ = InvokeInterceptors(ctx, ItemReceived, res)

				if res.IsValue() {
					_ = InvokeInterceptors(ctx, ValueReceived, res.Value())
				} else if res.IsError() {
					_ = InvokeInterceptors(ctx, ErrorOccurred, res.Error())
				} else if res.IsSentinel() {
					_ = InvokeInterceptors(ctx, SentinelReceived, res.Error())
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
					_ = InvokeInterceptors(ctx, ItemEmitted, res)
				}
			}
		}()

		return out
	})
}
