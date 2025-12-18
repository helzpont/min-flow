package core

import (
	"context"
	"fmt"
)

// Sink is a function type that consumes a Stream and produces a terminal result.
// It implements Transformer, allowing sinks to be composed in pipelines.
// Sink mirrors Emitter/Transmitter/Mapper/FlatMapper as a function-based abstraction.
//
// From(ctx, stream) consumes the stream and returns the result (mirrors Apply).
// Apply(ctx, stream) wraps the result in a single-element Stream for composition.
type Sink[IN, OUT any] func(context.Context, Stream[IN]) (OUT, error)

// From consumes a Stream and produces a terminal result.
// This is the primary method for using a Sink, mirroring Transformer.Apply.
func (s Sink[IN, OUT]) From(ctx context.Context, stream Stream[IN]) (OUT, error) {
	return s(ctx, stream)
}

// Defer returns a thunk that invokes the sink later. This keeps stream definition
// separate from execution, enabling lazy evaluation without losing stream-level
// behaviors (hooks, cancellation propagation, registry lookup).
func (s Sink[IN, OUT]) Defer(stream Stream[IN]) func(context.Context) (OUT, error) {
	return func(ctx context.Context) (OUT, error) {
		return s(ctx, stream)
	}
}

// Apply implements Transformer, producing a single-element Stream containing the result.
// This allows Sinks to be composed with other Transformers in a pipeline.
func (s Sink[IN, OUT]) Apply(stream Stream[IN]) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		out := make(chan Result[OUT], 1)
		go func() {
			defer close(out)
			result, err := s(ctx, stream)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- Err[OUT](err):
				}
			} else {
				select {
				case <-ctx.Done():
				case out <- Ok(result):
				}
			}
		}()
		return out
	})
}

// ToSlice returns a Sink that collects all stream values into a slice.
// Stops on the first error and returns it.
func ToSlice[T any]() Sink[T, []T] {
	return Sink[T, []T](Slice[T])
}

// ToFirst returns a Sink that returns the first value from the stream.
// Returns an error if the stream is empty or the first result is an error.
func ToFirst[T any]() Sink[T, T] {
	return Sink[T, T](First[T])
}

// Sink constructors for void results need a wrapper type since Run returns only error.

// ToRun returns a Sink that executes the stream for side effects.
// The result type is struct{} since Run only returns an error.
func ToRun[T any]() Sink[T, struct{}] {
	return func(ctx context.Context, stream Stream[T]) (struct{}, error) {
		return struct{}{}, Run(ctx, stream)
	}
}

// Terminal functions are sinks that consume stream data and produce
// a final result, such as a slice of values, the first value, or just
// run the stream for its side effects.

func Slice[OUT any](ctx context.Context, in Stream[OUT]) ([]OUT, error) {
	// Create a cancellable context to ensure we can stop processing
	// in case of an error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var result []OUT
	for res := range in.Emit(ctx) {
		if res.IsError() {
			return nil, res.Error()
		}
		result = append(result, res.Value())
	}
	return result, nil
}

func First[OUT any](ctx context.Context, in Stream[OUT]) (OUT, error) {
	var zero OUT

	// Create a cancellable context to ensure we can stop after
	// retrieving the first item.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	res, ok := <-in.Emit(ctx)
	switch {
	case !ok || res.IsSentinel():
		return zero, fmt.Errorf("stream is empty")
	case res.IsError():
		return zero, res.Error()
	default:
		return res.Value(), nil
	}
}

func Run[OUT any](ctx context.Context, in Stream[OUT]) error {
	// Create a cancellable context to ensure we can stop processing
	// in case of an error.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for res := range in.Emit(ctx) {
		if res.IsError() {
			return res.Error()
		}
	}
	return nil
}
