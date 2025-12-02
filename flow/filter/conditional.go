// Package operators provides stream transformation operators for the min-flow framework.
// This file contains conditional operators for controlling stream flow based on predicates and signals.
package filter

import (
	"context"
	"errors"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Common errors for conditional operators
var (
	errNoMatch         = errors.New("no matching element found")
	errMultipleMatches = errors.New("multiple matching elements found")
)

// TakeWhileWithIndex creates a Transformer that emits values while the predicate
// (which receives both value and index) returns true.
// Once the predicate returns false, the stream completes immediately.
func TakeWhileWithIndex[T any](predicate func(T, int) bool) core.Transformer[T, T] {
	if predicate == nil {
		panic("TakeWhileWithIndex: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			index := 0
			for res := range in {
				// Pass through errors
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if !predicate(res.Value(), index) {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
				index++
			}
		}()
		return out
	})
}

// SkipWhileWithIndex creates a Transformer that skips values while the predicate
// (which receives both value and index) returns true.
// Once the predicate returns false, all subsequent values are emitted.
func SkipWhileWithIndex[T any](predicate func(T, int) bool) core.Transformer[T, T] {
	if predicate == nil {
		panic("SkipWhileWithIndex: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			skipping := true
			index := 0
			for res := range in {
				// Always pass through errors
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}

				if skipping {
					if predicate(res.Value(), index) {
						index++
						continue
					}
					skipping = false
				}
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
				index++
			}
		}()
		return out
	})
}

// TakeUntil creates a Transformer that emits values until the notifier stream emits.
// When the notifier emits any value, the source stream completes.
func TakeUntil[T, N any](notifier core.Stream[N]) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			notifierCh := notifier.Emit(ctx)

			for {
				select {
				case <-ctx.Done():
					return
				case notifierRes, ok := <-notifierCh:
					if ok && notifierRes.IsValue() {
						// Notifier emitted a value - stop
						return
					}
					if !ok {
						// Notifier completed without emitting - continue
						notifierCh = nil
					}
				case res, ok := <-in:
					if !ok {
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}
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

// SkipUntil creates a Transformer that skips values until the notifier stream emits.
// After the notifier emits, all subsequent values from the source are passed through.
func SkipUntil[T, N any](notifier core.Stream[N]) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			notifierCh := notifier.Emit(ctx)
			skipping := true

			for {
				select {
				case <-ctx.Done():
					return
				case notifierRes, ok := <-notifierCh:
					if ok && notifierRes.IsValue() {
						// Notifier emitted - stop skipping
						skipping = false
					}
					if !ok {
						notifierCh = nil
					}
				case res, ok := <-in:
					if !ok {
						return
					}
					if skipping {
						continue
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}
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

// ElementAt creates a Transformer that emits only the element at the specified index.
// If the stream has fewer elements, nothing is emitted.
func ElementAt[T any](index int) core.Transformer[T, T] {
	if index < 0 {
		panic("ElementAt: index cannot be negative")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			current := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if current == index {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					return
				}
				current++
			}
		}()
		return out
	})
}

// ElementAtOrDefault creates a Transformer that emits the element at the specified index,
// or emits the default value if the stream has fewer elements.
func ElementAtOrDefault[T any](index int, defaultValue T) core.Transformer[T, T] {
	if index < 0 {
		panic("ElementAtOrDefault: index cannot be negative")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			current := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if current == index {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					return
				}
				current++
			}
			// Stream ended without reaching index, emit default
			select {
			case <-ctx.Done():
			case out <- core.Ok(defaultValue):
			}
		}()
		return out
	})
}

// Single creates a Transformer that emits the single element matching the predicate.
// If zero or more than one element matches, an error is emitted.
// If predicate is nil, expects exactly one element in the stream.
func Single[T any](predicate func(T) bool) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			var match *core.Result[T]
			matchCount := 0

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate == nil || predicate(res.Value()) {
					matchCount++
					if matchCount == 1 {
						match = res
					} else if matchCount == 2 {
						// More than one match
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[T](errMultipleMatches):
						}
					}
				}
			}

			if matchCount == 0 {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](errNoMatch):
				}
			} else if matchCount == 1 && match != nil {
				select {
				case <-ctx.Done():
				case out <- match:
				}
			}
		}()
		return out
	})
}

// SingleOrDefault creates a Transformer that emits the single element matching the predicate,
// or the default value if no elements match. Errors if more than one element matches.
func SingleOrDefault[T any](predicate func(T) bool, defaultValue T) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			var match *core.Result[T]
			matchCount := 0

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate == nil || predicate(res.Value()) {
					matchCount++
					if matchCount == 1 {
						match = res
					} else if matchCount == 2 {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[T](errMultipleMatches):
						}
					}
				}
			}

			if matchCount == 0 {
				select {
				case <-ctx.Done():
				case out <- core.Ok(defaultValue):
				}
			} else if matchCount == 1 && match != nil {
				select {
				case <-ctx.Done():
				case out <- match:
				}
			}
		}()
		return out
	})
}

// FirstOrDefault creates a Transformer that emits the first element matching the predicate,
// or the default value if no elements match.
// If predicate is nil, returns the first element or default.
func FirstOrDefault[T any](predicate func(T) bool, defaultValue T) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate == nil || predicate(res.Value()) {
					select {
					case <-ctx.Done():
					case out <- res:
					}
					return
				}
			}
			// No match found, emit default
			select {
			case <-ctx.Done():
			case out <- core.Ok(defaultValue):
			}
		}()
		return out
	})
}

// LastOrDefault creates a Transformer that emits the last element matching the predicate,
// or the default value if no elements match.
// If predicate is nil, returns the last element or default.
func LastOrDefault[T any](predicate func(T) bool, defaultValue T) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			var lastMatch *core.Result[T]
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate == nil || predicate(res.Value()) {
					lastMatch = res
				}
			}

			if lastMatch != nil {
				select {
				case <-ctx.Done():
				case out <- lastMatch:
				}
			} else {
				select {
				case <-ctx.Done():
				case out <- core.Ok(defaultValue):
				}
			}
		}()
		return out
	})
}
