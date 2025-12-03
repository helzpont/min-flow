// Package operators provides common stream transformation operators
// built on top of the core flow abstractions.
package filter

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Where creates a Transformer that only passes through items matching the predicate.
// Items that don't match are silently dropped. Errors are passed through unchanged.
func Where[T any](predicate func(T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			for res := range in {
				// Pass through errors and sentinels unchanged
				if !res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}

				// Apply predicate to values
				if predicate(res.Value()) {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// MapWhere creates a Transformer that both filters and maps in a single pass.
// The function returns (value, true) to include the transformed value,
// or (_, false) to filter out the item. Errors in the input are passed through.
func MapWhere[IN, OUT any](fn func(IN) (OUT, bool)) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[IN]) <-chan *core.Result[OUT] {
		out := make(chan *core.Result[OUT])
		go func() {
			defer close(out)
			for res := range in {
				// Pass through errors as converted type
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[OUT](res.Error()):
					}
					continue
				}

				// Pass through sentinels as converted type
				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[OUT](res.Error()):
					}
					continue
				}

				// Apply filter-map function
				if mapped, ok := fn(res.Value()); ok {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(mapped):
					}
				}
			}
		}()
		return out
	})
}

// Errors creates a Transformer that filters out errors from the stream.
// Only values (non-error, non-sentinel results) pass through.
func Errors[T any]() core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)
			for res := range in {
				// Only pass through values
				if res.IsValue() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// Exclude creates a Transformer that filters out items matching the predicate.
// This is the inverse of Where. Items matching the predicate are dropped.
func Exclude[T any](predicate func(T) bool) core.Transformer[T, T] {
	return Where(func(v T) bool { return !predicate(v) })
}
