// Package flow provides a stream processing framework for building
// scalable, observable, and resilient data pipelines in Go.
//
// This package is the primary user-facing API. Most users should only
// need to import this package. The flow/core subpackage contains
// low-level abstractions that are rarely needed directly.
package flow

import (
	"context"
	"iter"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Type aliases for core stream abstractions.
// These allow users to work with the framework without importing core directly.
type (
	// Result represents the outcome of processing an item in the stream.
	// It exists in one of three states: Value, Error, or Sentinel.
	Result[T any] = core.Result[T]

	// Stream represents a flow of data with methods for processing.
	Stream[T any] = core.Stream[T]

	// Transformer transforms a Stream of type IN into a Stream of type OUT.
	Transformer[IN, OUT any] = core.Transformer[IN, OUT]

	// Emitter produces a channel of Results and implements Stream.
	Emitter[T any] = core.Emitter[T]

	// Transmitter transforms one channel of Results into another and implements Transformer.
	Transmitter[IN, OUT any] = core.Transmitter[IN, OUT]

	// Mapper transforms individual items (1:1 cardinality) and implements Transformer.
	Mapper[IN, OUT any] = core.Mapper[IN, OUT]

	// FlatMapper transforms individual items (1:N cardinality) and implements Transformer.
	FlatMapper[IN, OUT any] = core.FlatMapper[IN, OUT]
)

// ErrEndOfStream is the sentinel error indicating normal stream termination.
var ErrEndOfStream = core.ErrEndOfStream

// Result constructors - wrappers around core functions.

// Ok creates a successful Result containing the given value.
func Ok[T any](value T) *Result[T] {
	return core.Ok(value)
}

// Err creates an error Result for recoverable processing failures.
func Err[T any](err error) *Result[T] {
	return core.Err[T](err)
}

// Sentinel creates a sentinel Result for stream control signals.
func Sentinel[T any](err error) *Result[T] {
	return core.Sentinel[T](err)
}

// EndOfStream creates a sentinel indicating normal stream termination.
func EndOfStream[T any]() *Result[T] {
	return core.EndOfStream[T]()
}

// NewResult creates a Result with explicit control over all fields.
func NewResult[T any](value T, err error, isSentinel bool) *Result[T] {
	return core.NewResult(value, err, isSentinel)
}

// Mapper/FlatMapper constructors.

// Map creates a Mapper from a simple transformation function.
func Map[IN, OUT any](mapFunc func(IN) (OUT, error)) Mapper[IN, OUT] {
	return core.Map(mapFunc)
}

// FlatMap creates a FlatMapper from a function returning a slice.
func FlatMap[IN, OUT any](flatMapFunc func(IN) ([]OUT, error)) FlatMapper[IN, OUT] {
	return core.FlatMap(flatMapFunc)
}

// Terminal operations.

// Slice collects all stream values into a slice.
func Slice[T any](ctx context.Context, in Stream[T]) ([]T, error) {
	return core.Slice(ctx, in)
}

// First returns the first value from the stream.
func First[T any](ctx context.Context, in Stream[T]) (T, error) {
	return core.First(ctx, in)
}

// Run executes the stream for side effects only.
func Run[T any](ctx context.Context, in Stream[T]) error {
	return core.Run(ctx, in)
}

// Collect gathers all Results (including errors) into a slice.
func Collect[T any](ctx context.Context, stream Stream[T]) []*Result[T] {
	return core.Collect(ctx, stream)
}

// All returns an iterator over all Results in the stream.
func All[T any](ctx context.Context, stream Stream[T]) iter.Seq[*Result[T]] {
	return core.All(ctx, stream)
}

// Emitter/Transmitter constructors.

// Emit creates an Emitter from a channel-producing function.
func Emit[T any](emitter func(context.Context) <-chan *Result[T]) Emitter[T] {
	return core.Emit(emitter)
}

// Transmit creates a Transmitter from a channel transformation function.
func Transmit[IN, OUT any](transmitter func(context.Context, <-chan *Result[IN]) <-chan *Result[OUT]) Transmitter[IN, OUT] {
	return core.Transmit(transmitter)
}
