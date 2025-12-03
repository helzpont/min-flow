// package core defines the core abstractions for data flow processing,
// including streams, transformers, emitters, and result handling.
// It provides the foundational building blocks for creating complex
// data processing pipelines in a modular and composable manner.
//
// NOTE: this package should have no dependencies outside the standard
// library, including other flow packages.
package core

import (
	"context"
	"iter"
)

// Stream represents a flow of data. It is a higher-level abstraction
// than channels, providing methods for processing the stream's data.
// Along with Transformer, it enables building complex data processing
// pipelines.
// Stream answers the question: "What operations will produce the stream's data?".
type Stream[OUT any] interface {
	Emit(context.Context) <-chan Result[OUT]

	Collect(context.Context) []Result[OUT]
	All(context.Context) iter.Seq[Result[OUT]]
}

func Collect[OUT any](ctx context.Context, stream Stream[OUT]) []Result[OUT] {
	var results []Result[OUT]
	for res := range stream.Emit(ctx) {
		results = append(results, res)
	}
	return results
}

func All[OUT any](ctx context.Context, stream Stream[OUT]) iter.Seq[Result[OUT]] {
	return func(yield func(Result[OUT]) bool) {
		for res := range stream.Emit(ctx) {
			if !yield(res) {
				return
			}
		}
	}
}

// Transformer represents a data processing unit that transforms
// a Stream of type IN into a Stream of type OUT. Transformers can
// be composed to build complex data processing pipelines.
// They answer the question: "What operations are being applied to the stream's data?".
type Transformer[IN, OUT any] interface {
	Apply(context.Context, Stream[IN]) Stream[OUT]
}
