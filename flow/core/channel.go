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
//
// For high-throughput scenarios, consider using InterceptBuffered which
// reduces channel synchronization overhead.
func Intercept[T any]() Transmitter[T, T] {
	return InterceptBuffered[T](0)
}

// InterceptBuffered creates an Intercept transmitter with buffered output channel.
// A buffer size of 0 uses an unbuffered channel (same as Intercept).
// Larger buffers reduce goroutine synchronization overhead at the cost of memory.
//
// Recommended buffer sizes:
//   - 0: Strict backpressure, highest latency
//   - 16-64: Good balance for most use cases
//   - 256+: High-throughput scenarios
func InterceptBuffered[T any](bufferSize int) Transmitter[T, T] {
	return Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T], bufferSize)

		go func() {
			defer close(out)

			// Cache registry lookup - this is the key optimization
			registry, hasRegistry := GetRegistry(ctx)
			var interceptors []Interceptor
			if hasRegistry {
				interceptors = registry.Interceptors()
			}

			// Helper to invoke matching interceptors without repeated lookups
			invoke := func(event Event, args ...any) {
				if !hasRegistry {
					return
				}
				for _, interceptor := range interceptors {
					for _, pattern := range interceptor.Events() {
						if event.Matches(string(pattern)) {
							_ = interceptor.Do(ctx, event, args...)
							break
						}
					}
				}
			}

			// Invoke stream start interceptors
			invoke(StreamStart)

			defer func() {
				// Invoke stream end interceptors
				invoke(StreamEnd)
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Invoke item-level interceptors
				invoke(ItemReceived, res)

				if res.IsValue() {
					invoke(ValueReceived, res.Value())
				} else if res.IsError() {
					invoke(ErrorOccurred, res.Error())
				} else if res.IsSentinel() {
					invoke(SentinelReceived, res.Error())
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
					invoke(ItemEmitted, res)
				}
			}
		}()

		return out
	})
}
