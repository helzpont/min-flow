// Package operators provides stream transformation operators for the min-flow framework.
// This file contains selection operators for finding and testing stream elements.
package filter

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// FindResult represents the result of a Find operation.
type FindResult[T any] struct {
	Value T
	Found bool
}

// Find creates a Transformer that emits a FindResult with the first element matching the predicate.
// If no element matches, emits a FindResult with Found=false.
func Find[T any](predicate func(T) bool) core.Transformer[T, FindResult[T]] {
	if predicate == nil {
		panic("Find: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[FindResult[T]] {
		out := make(chan *core.Result[FindResult[T]])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[FindResult[T]](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate(res.Value()) {
					select {
					case <-ctx.Done():
					case out <- core.Ok(FindResult[T]{Value: res.Value(), Found: true}):
					}
					return
				}
			}
			// No match
			var zero T
			select {
			case <-ctx.Done():
			case out <- core.Ok(FindResult[T]{Value: zero, Found: false}):
			}
		}()
		return out
	})
}

// FindIndex creates a Transformer that emits the index of the first element matching the predicate.
// If no element matches, emits -1.
func FindIndex[T any](predicate func(T) bool) core.Transformer[T, int] {
	if predicate == nil {
		panic("FindIndex: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)

			index := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[int](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate(res.Value()) {
					select {
					case <-ctx.Done():
					case out <- core.Ok(index):
					}
					return
				}
				index++
			}
			// No match
			select {
			case <-ctx.Done():
			case out <- core.Ok(-1):
			}
		}()
		return out
	})
}

// FindLast creates a Transformer that emits a FindResult with the last element matching the predicate.
// If no element matches, emits a FindResult with Found=false.
func FindLast[T any](predicate func(T) bool) core.Transformer[T, FindResult[T]] {
	if predicate == nil {
		panic("FindLast: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[FindResult[T]] {
		out := make(chan *core.Result[FindResult[T]])
		go func() {
			defer close(out)

			var lastMatch T
			found := false
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[FindResult[T]](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate(res.Value()) {
					lastMatch = res.Value()
					found = true
				}
			}

			if found {
				select {
				case <-ctx.Done():
				case out <- core.Ok(FindResult[T]{Value: lastMatch, Found: true}):
				}
			} else {
				var zero T
				select {
				case <-ctx.Done():
				case out <- core.Ok(FindResult[T]{Value: zero, Found: false}):
				}
			}
		}()
		return out
	})
}

// FindLastIndex creates a Transformer that emits the index of the last element matching the predicate.
// If no element matches, emits -1.
func FindLastIndex[T any](predicate func(T) bool) core.Transformer[T, int] {
	if predicate == nil {
		panic("FindLastIndex: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)

			lastIndex := -1
			index := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[int](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate(res.Value()) {
					lastIndex = index
				}
				index++
			}

			select {
			case <-ctx.Done():
			case out <- core.Ok(lastIndex):
			}
		}()
		return out
	})
}

// Contains creates a Transformer that checks if the stream contains a specific value.
// Uses the == operator for comparison (values must be comparable).
func Contains[T comparable](value T) core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if res.Value() == value {
					select {
					case <-ctx.Done():
					case out <- core.Ok(true):
					}
					return
				}
			}
			// Not found
			select {
			case <-ctx.Done():
			case out <- core.Ok(false):
			}
		}()
		return out
	})
}

// ContainsBy creates a Transformer that checks if the stream contains an element matching the predicate.
func ContainsBy[T any](predicate func(T) bool) core.Transformer[T, bool] {
	if predicate == nil {
		panic("ContainsBy: predicate cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate(res.Value()) {
					select {
					case <-ctx.Done():
					case out <- core.Ok(true):
					}
					return
				}
			}
			// Not found
			select {
			case <-ctx.Done():
			case out <- core.Ok(false):
			}
		}()
		return out
	})
}

// IsEmpty creates a Transformer that checks if the stream contains no values.
// Emits true if empty, false otherwise.
func IsEmpty[T any]() core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				// Found a value, not empty
				select {
				case <-ctx.Done():
				case out <- core.Ok(false):
				}
				return
			}
			// Stream is empty
			select {
			case <-ctx.Done():
			case out <- core.Ok(true):
			}
		}()
		return out
	})
}

// IsNotEmpty creates a Transformer that checks if the stream contains at least one value.
// Emits true if not empty, false otherwise.
func IsNotEmpty[T any]() core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[bool](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				// Found a value
				select {
				case <-ctx.Done():
				case out <- core.Ok(true):
				}
				return
			}
			// Stream is empty
			select {
			case <-ctx.Done():
			case out <- core.Ok(false):
			}
		}()
		return out
	})
}

// SequenceEqual creates a Transformer that compares the source stream with another stream.
// Emits true if both streams have the same elements in the same order.
func SequenceEqual[T comparable](other core.Stream[T]) core.Transformer[T, bool] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			otherCh := other.Emit(ctx)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						// Source finished, check if other also finished
						for otherRes := range otherCh {
							if otherRes.IsValue() {
								// Other has more elements
								select {
								case <-ctx.Done():
								case out <- core.Ok(false):
								}
								return
							}
						}
						select {
						case <-ctx.Done():
						case out <- core.Ok(true):
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[bool](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Get corresponding element from other
					var otherVal T
					foundOther := false
					for otherRes := range otherCh {
						if otherRes.IsError() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Err[bool](otherRes.Error()):
							}
							continue
						}
						if otherRes.IsSentinel() {
							continue
						}
						otherVal = otherRes.Value()
						foundOther = true
						break
					}

					if !foundOther {
						// Other stream finished early
						select {
						case <-ctx.Done():
						case out <- core.Ok(false):
						}
						return
					}

					if res.Value() != otherVal {
						select {
						case <-ctx.Done():
						case out <- core.Ok(false):
						}
						return
					}
				}
			}
		}()
		return out
	})
}

// SequenceEqualBy creates a Transformer that compares streams using a custom comparator.
func SequenceEqualBy[T any](other core.Stream[T], equals func(T, T) bool) core.Transformer[T, bool] {
	if equals == nil {
		panic("SequenceEqualBy: equals function cannot be nil")
	}
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[bool] {
		out := make(chan *core.Result[bool])
		go func() {
			defer close(out)

			otherCh := other.Emit(ctx)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						// Source finished, check if other also finished
						for otherRes := range otherCh {
							if otherRes.IsValue() {
								select {
								case <-ctx.Done():
								case out <- core.Ok(false):
								}
								return
							}
						}
						select {
						case <-ctx.Done():
						case out <- core.Ok(true):
						}
						return
					}
					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[bool](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Get corresponding element from other
					var otherVal T
					foundOther := false
					for otherRes := range otherCh {
						if otherRes.IsError() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Err[bool](otherRes.Error()):
							}
							continue
						}
						if otherRes.IsSentinel() {
							continue
						}
						otherVal = otherRes.Value()
						foundOther = true
						break
					}

					if !foundOther {
						select {
						case <-ctx.Done():
						case out <- core.Ok(false):
						}
						return
					}

					if !equals(res.Value(), otherVal) {
						select {
						case <-ctx.Done():
						case out <- core.Ok(false):
						}
						return
					}
				}
			}
		}()
		return out
	})
}

// CountIf creates a Transformer that counts elements matching the predicate.
// If predicate is nil, counts all elements.
func CountIf[T any](predicate func(T) bool) core.Transformer[T, int] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)

			count := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[int](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if predicate == nil || predicate(res.Value()) {
					count++
				}
			}

			select {
			case <-ctx.Done():
			case out <- core.Ok(count):
			}
		}()
		return out
	})
}

// IndexOf creates a Transformer that emits the index of a specific value.
// If not found, emits -1.
func IndexOf[T comparable](value T) core.Transformer[T, int] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)

			index := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[int](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if res.Value() == value {
					select {
					case <-ctx.Done():
					case out <- core.Ok(index):
					}
					return
				}
				index++
			}
			// Not found
			select {
			case <-ctx.Done():
			case out <- core.Ok(-1):
			}
		}()
		return out
	})
}

// LastIndexOf creates a Transformer that emits the last index of a specific value.
// If not found, emits -1.
func LastIndexOf[T comparable](value T) core.Transformer[T, int] {
	return core.Transmit(func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)

			lastIndex := -1
			index := 0
			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[int](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					continue
				}
				if res.Value() == value {
					lastIndex = index
				}
				index++
			}

			select {
			case <-ctx.Done():
			case out <- core.Ok(lastIndex):
			}
		}()
		return out
	})
}
