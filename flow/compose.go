package flow

import (
	"context"
)

// Through chains two transformers together, creating a new transformer
// that first applies t1 and then t2 to the stream.
func Through[IN, MID, OUT any](t1 Transformer[IN, MID], t2 Transformer[MID, OUT]) Transformer[IN, OUT] {
	return Transmit(func(ctx context.Context, in <-chan *Result[IN]) <-chan *Result[OUT] {
		inStream := Emit(func(_ context.Context) <-chan *Result[IN] { return in })
		return t2.Apply(ctx, t1.Apply(ctx, inStream)).Emit(ctx)
	})
}

// Chain composes multiple transformers of the same type into a single transformer.
// Transformers are applied in order from left to right.
// If no transformers are provided, returns an identity transformer.
func Chain[T any](transformers ...Transformer[T, T]) Transformer[T, T] {
	return Transmit(func(ctx context.Context, in <-chan *Result[T]) <-chan *Result[T] {
		var result Stream[T] = Emit(func(_ context.Context) <-chan *Result[T] { return in })
		for _, t := range transformers {
			result = t.Apply(ctx, result)
		}
		return result.Emit(ctx)
	})
}

// Pipe applies a series of transformers to a stream, returning the final stream.
// This is a convenience function for applying multiple transformations inline.
func Pipe[T any](ctx context.Context, source Stream[T], transformers ...Transformer[T, T]) Stream[T] {
	result := source
	for _, t := range transformers {
		result = t.Apply(ctx, result)
	}
	return result
}

// Apply is a helper to apply a single transformer to a stream.
// Equivalent to transformer.Apply(ctx, stream) but reads left-to-right.
func Apply[IN, OUT any](ctx context.Context, stream Stream[IN], transformer Transformer[IN, OUT]) Stream[OUT] {
	return transformer.Apply(ctx, stream)
}
