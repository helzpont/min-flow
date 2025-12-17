package fast

import "context"

// Reduce combines all stream values into a single result.
func Reduce[T any](fn func(T, T) T) Transformer[T, T] {
	return reduceTransformer[T]{fn: fn}
}

type reduceTransformer[T any] struct {
	fn func(T, T) T
}

func (r reduceTransformer[T]) Apply(s Stream[T]) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, 1)
		go func() {
			defer close(out)
			var acc T
			first := true
			for v := range s.Emit(ctx) {
				if first {
					acc = v
					first = false
				} else {
					acc = r.fn(acc, v)
				}
			}
			if !first {
				select {
				case <-ctx.Done():
				case out <- acc:
				}
			}
		}()
		return out
	})
}

// Fold reduces with an initial value.
func Fold[T, R any](initial R, fn func(R, T) R) Transformer[T, R] {
	return foldTransformer[T, R]{initial: initial, fn: fn}
}

type foldTransformer[T, R any] struct {
	initial R
	fn      func(R, T) R
}

func (f foldTransformer[T, R]) Apply(s Stream[T]) Stream[R] {
	return Emitter[R](func(ctx context.Context) <-chan R {
		out := make(chan R, 1)
		go func() {
			defer close(out)
			acc := f.initial
			for v := range s.Emit(ctx) {
				acc = f.fn(acc, v)
			}
			select {
			case <-ctx.Done():
			case out <- acc:
			}
		}()
		return out
	})
}

// Take limits the stream to n items.
func Take[T any](n int) Transformer[T, T] {
	return takeTransformer[T]{n: n}
}

type takeTransformer[T any] struct {
	n int
}

func (t takeTransformer[T]) Apply(s Stream[T]) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			count := 0
			for v := range s.Emit(ctx) {
				if count >= t.n {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- v:
					count++
				}
			}
		}()
		return out
	})
}

// Skip skips the first n items.
func Skip[T any](n int) Transformer[T, T] {
	return skipTransformer[T]{n: n}
}

type skipTransformer[T any] struct {
	n int
}

func (t skipTransformer[T]) Apply(s Stream[T]) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			count := 0
			for v := range s.Emit(ctx) {
				if count < t.n {
					count++
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- v:
				}
			}
		}()
		return out
	})
}

// Batch groups items into fixed-size slices.
func Batch[T any](size int) Transformer[T, []T] {
	return batchTransformer[T]{size: size}
}

type batchTransformer[T any] struct {
	size int
}

func (b batchTransformer[T]) Apply(s Stream[T]) Stream[[]T] {
	return Emitter[[]T](func(ctx context.Context) <-chan []T {
		out := make(chan []T, DefaultBufferSize)
		go func() {
			defer close(out)
			batch := make([]T, 0, b.size)
			for v := range s.Emit(ctx) {
				batch = append(batch, v)
				if len(batch) >= b.size {
					select {
					case <-ctx.Done():
						return
					case out <- batch:
					}
					batch = make([]T, 0, b.size)
				}
			}
			// Emit remaining items
			if len(batch) > 0 {
				select {
				case <-ctx.Done():
				case out <- batch:
				}
			}
		}()
		return out
	})
}
