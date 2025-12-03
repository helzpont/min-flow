package fast

import "context"

// Mapper is a simple transformation function.
// Unlike core.Mapper, it operates directly on values without Result wrapping.
// Panics are NOT recovered - use only with trusted transformation functions.
type Mapper[IN, OUT any] func(IN) OUT

// Map creates a Mapper from a function.
// This is the fastest possible mapping - no error handling, no Result wrapping.
func Map[IN, OUT any](fn func(IN) OUT) Mapper[IN, OUT] {
	return fn
}

// Apply transforms a stream using this Mapper.
func (m Mapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT] {
	return Emitter[OUT](func(ctx context.Context) <-chan OUT {
		out := make(chan OUT, DefaultBufferSize)
		go func() {
			defer close(out)
			for v := range s.Emit(ctx) {
				result := m(v)
				select {
				case <-ctx.Done():
					return
				case out <- result:
				}
			}
		}()
		return out
	})
}

// FlatMapper transforms one input into zero or more outputs.
type FlatMapper[IN, OUT any] func(IN) []OUT

// FlatMap creates a FlatMapper from a function.
func FlatMap[IN, OUT any](fn func(IN) []OUT) FlatMapper[IN, OUT] {
	return fn
}

// Apply transforms a stream using this FlatMapper.
func (m FlatMapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT] {
	return Emitter[OUT](func(ctx context.Context) <-chan OUT {
		out := make(chan OUT, DefaultBufferSize)
		go func() {
			defer close(out)
			for v := range s.Emit(ctx) {
				for _, result := range m(v) {
					select {
					case <-ctx.Done():
						return
					case out <- result:
					}
				}
			}
		}()
		return out
	})
}

// Fuse combines two Mappers into one without intermediate channels.
func Fuse[IN, MID, OUT any](first Mapper[IN, MID], second Mapper[MID, OUT]) Mapper[IN, OUT] {
	return func(in IN) OUT {
		return second(first(in))
	}
}

// Predicate is a filter function.
type Predicate[T any] func(T) bool

// Filter creates a filtering transformer.
func Filter[T any](pred Predicate[T]) Transformer[T, T] {
	return filterTransformer[T]{pred: pred}
}

type filterTransformer[T any] struct {
	pred Predicate[T]
}

func (f filterTransformer[T]) Apply(ctx context.Context, s Stream[T]) Stream[T] {
	return Emitter[T](func(ctx context.Context) <-chan T {
		out := make(chan T, DefaultBufferSize)
		go func() {
			defer close(out)
			for v := range s.Emit(ctx) {
				if f.pred(v) {
					select {
					case <-ctx.Done():
						return
					case out <- v:
					}
				}
			}
		}()
		return out
	})
}
