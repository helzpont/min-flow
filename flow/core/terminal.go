package core

import (
	"context"
	"fmt"
)

// Teminal functions are basically sinks that consume the stream data
// and produce a final result, such as a slice of values, the first value,
// or just run the stream for its side effects.

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
