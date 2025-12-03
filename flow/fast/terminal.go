package fast

import "context"

// Slice collects all stream values into a slice.
// Unlike core.Slice, this never returns an error - panics propagate up.
func Slice[T any](ctx context.Context, s Stream[T]) []T {
	var result []T
	for v := range s.Emit(ctx) {
		result = append(result, v)
	}
	return result
}

// First returns the first value from a stream.
// Returns zero value if stream is empty.
func First[T any](ctx context.Context, s Stream[T]) (T, bool) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	v, ok := <-s.Emit(ctx)
	return v, ok
}

// Run consumes the stream for side effects.
func Run[T any](ctx context.Context, s Stream[T]) {
	for range s.Emit(ctx) {
		// consume
	}
}

// Count returns the number of items in the stream.
func Count[T any](ctx context.Context, s Stream[T]) int {
	count := 0
	for range s.Emit(ctx) {
		count++
	}
	return count
}

// ForEach applies a function to each element.
func ForEach[T any](ctx context.Context, s Stream[T], fn func(T)) {
	for v := range s.Emit(ctx) {
		fn(v)
	}
}
