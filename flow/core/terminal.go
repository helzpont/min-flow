package core

import (
	"context"
	"fmt"
)

// SinkFunc is the function signature for consuming streams.
type SinkFunc[IN, OUT any] func(context.Context, Stream[IN]) (OUT, error)

// Sink is a struct that consumes a Stream and produces a terminal result.
// It implements Transformer, allowing sinks to be composed in pipelines.
type Sink[IN, OUT any] struct {
	fn SinkFunc[IN, OUT]
}

// NewSink creates a Sink from a function.
func NewSink[IN, OUT any](fn SinkFunc[IN, OUT]) *Sink[IN, OUT] {
	return &Sink[IN, OUT]{fn: fn}
}

// From consumes a Stream and produces a terminal result.
// This is the primary method for using a Sink, mirroring Transformer.Apply.
func (s *Sink[IN, OUT]) From(ctx context.Context, stream Stream[IN]) (OUT, error) {
	return s.fn(ctx, stream)
}

// Apply implements Transformer, producing a single-element Stream containing the result.
// This allows Sinks to be composed with other Transformers in a pipeline.
func (s *Sink[IN, OUT]) Apply(ctx context.Context, stream Stream[IN]) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		out := make(chan Result[OUT], 1)
		go func() {
			defer close(out)
			result, err := s.fn(ctx, stream)
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
func ToSlice[T any]() *Sink[T, []T] {
	return NewSink(Slice[T])
}

// ToFirst returns a Sink that returns the first value from the stream.
// Returns an error if the stream is empty or the first result is an error.
func ToFirst[T any]() *Sink[T, T] {
	return NewSink(First[T])
}

// Sink constructors for void results need a wrapper type since Run returns only error.

// ToRun returns a Sink that executes the stream for side effects.
// The result type is struct{} since Run only returns an error.
func ToRun[T any]() *Sink[T, struct{}] {
	return NewSink(func(ctx context.Context, stream Stream[T]) (struct{}, error) {
		return struct{}{}, Run(ctx, stream)
	})
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
