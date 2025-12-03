package core

import (
	"context"
	"fmt"
)

// DefaultBufferSize is the default buffer size for internal channels.
// A small buffer reduces goroutine synchronization overhead without
// consuming excessive memory.
const DefaultBufferSize = 64

// TransformConfig holds configuration options for transform operations.
type TransformConfig struct {
	BufferSize int
}

// TransformOption is a functional option for configuring transforms.
type TransformOption func(*TransformConfig)

// WithBufferSize sets the buffer size for the transform's output channel.
// A larger buffer can improve throughput for CPU-bound operations by reducing
// goroutine synchronization, while a smaller buffer reduces memory usage.
// Use 0 for unbuffered (synchronous) operation.
func WithBufferSize(size int) TransformOption {
	return func(c *TransformConfig) {
		c.BufferSize = size
	}
}

// defaultConfig returns a TransformConfig with default values.
func defaultConfig() TransformConfig {
	return TransformConfig{
		BufferSize: DefaultBufferSize,
	}
}

// applyOptions applies functional options to a config.
func applyOptions(opts ...TransformOption) TransformConfig {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Mapper defines a function that maps a Result of type IN to a Result of type OUT. It represents a transformation
// that maintains the cardinality of the flow (one input item produces one output item).
// The mapper function is at the lowest level of abstraction in the flow processing pipeline.
// It answers the question: "What is done to each item in the flow?"
type Mapper[IN, OUT any] func(Result[IN]) (Result[OUT], error)

// Map creates a Mapper from a transformation function. The returned Mapper
// uses DefaultBufferSize for its output channel. Use MapWith for custom buffer sizes.
func Map[IN, OUT any](mapFunc func(IN) (OUT, error)) Mapper[IN, OUT] {
	return func(res Result[IN]) (out Result[OUT], err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in Map function: %v", r)
			}
		}()

		if res.IsError() {
			return Err[OUT](res.Error()), nil
		}
		mappedValue, err := mapFunc(res.Value())
		if err != nil {
			return Err[OUT](err), nil
		}
		return Ok(mappedValue), nil
	}
}

// Apply transforms a stream using this Mapper with default configuration.
func (m Mapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT] {
	return m.ApplyWith(ctx, s)
}

// ApplyWith transforms a stream using this Mapper with custom options.
func (m Mapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], opts ...TransformOption) Stream[OUT] {
	cfg := applyOptions(opts...)
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], cfg.BufferSize)
		go func() {
			defer close(outChan)
			for resIn := range s.Emit(ctx) {
				resOut, err := m(resIn)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case outChan <- Err[OUT](err):
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case outChan <- resOut:
				}
			}
		}()
		return outChan
	})
}

// FlatMapper defines a function that maps a Result of type IN to a Result containing a slice of Results of type OUT.
// It represents a transformation that can change the cardinality of the flow (one input item can produce zero or more output items).
// The flat mapper function is at the lowest level of abstraction in the flow processing pipeline.
// It answers the question: "How are items in the flow reduced or expanded?"
type FlatMapper[IN, OUT any] func(Result[IN]) ([]Result[OUT], error)

// FlatMap creates a FlatMapper from a transformation function. The returned FlatMapper
// uses DefaultBufferSize for its output channel. Use ApplyWith for custom buffer sizes.
func FlatMap[IN, OUT any](flatMapFunc func(IN) ([]OUT, error)) FlatMapper[IN, OUT] {
	return func(res Result[IN]) (outs []Result[OUT], err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in FlatMap function: %v", r)
			}
		}()

		if res.IsError() {
			return []Result[OUT]{Err[OUT](res.Error())}, nil
		}
		mappedValues, err := flatMapFunc(res.Value())
		if err != nil {
			return []Result[OUT]{Err[OUT](err)}, nil
		}
		results := make([]Result[OUT], len(mappedValues))
		for i, v := range mappedValues {
			results[i] = Ok(v)
		}
		return results, nil
	}
}

// Apply transforms a stream using this FlatMapper with default configuration.
func (fm FlatMapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT] {
	return fm.ApplyWith(ctx, s)
}

// ApplyWith transforms a stream using this FlatMapper with custom options.
func (fm FlatMapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], opts ...TransformOption) Stream[OUT] {
	cfg := applyOptions(opts...)
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], cfg.BufferSize)
		go func() {
			defer close(outChan)
			for resIn := range s.Emit(ctx) {
				resOuts, err := fm(resIn)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case outChan <- Err[OUT](err):
					}
					continue
				}
				for _, resOut := range resOuts {
					select {
					case <-ctx.Done():
						return
					case outChan <- resOut:
					}
				}
			}
		}()
		return outChan
	})
}
