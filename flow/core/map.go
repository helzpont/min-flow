package core

import (
	"context"
	"fmt"
	"iter"
)

// DefaultBufferSize is the default buffer size for internal channels.
// A small buffer reduces goroutine synchronization overhead without
// consuming excessive memory.
const DefaultBufferSize = 64

// ContextCheckStrategy determines how often ctx.Done() is checked during processing.
type ContextCheckStrategy int

const (
	// CheckEveryItem checks ctx.Done() on every item send. This provides the
	// fastest cancellation response but has higher overhead.
	CheckEveryItem ContextCheckStrategy = iota

	// CheckOnCapacity only checks ctx.Done() when the output channel might block
	// or after processing bufferSize items. This reduces overhead by 25-40% but
	// may delay cancellation response by up to bufferSize items.
	CheckOnCapacity
)

// TransformConfig holds configuration options for transform operations.
// It implements the Config interface, allowing it to be registered in a
// Registry and accessed via context for stream-level configuration.
type TransformConfig struct {
	BufferSize    int
	CheckStrategy ContextCheckStrategy
}

// Init initializes the TransformConfig. Currently a no-op.
func (c *TransformConfig) Init() error {
	return nil
}

// Close cleans up the TransformConfig. Currently a no-op.
func (c *TransformConfig) Close() error {
	return nil
}

// Validate ensures the TransformConfig has valid values.
func (c *TransformConfig) Validate() error {
	if c.BufferSize < 0 {
		return fmt.Errorf("buffer size must be non-negative, got %d", c.BufferSize)
	}
	return nil
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

// WithCheckStrategy sets the context checking strategy for the transform.
// CheckEveryItem (default) provides fastest cancellation but higher overhead.
// CheckOnCapacity reduces overhead by 25-40% but may delay cancellation.
func WithCheckStrategy(strategy ContextCheckStrategy) TransformOption {
	return func(c *TransformConfig) {
		c.CheckStrategy = strategy
	}
}

// WithFastCancellation is a convenience option that sets CheckEveryItem strategy.
// Use when low-latency cancellation is more important than throughput.
func WithFastCancellation() TransformOption {
	return WithCheckStrategy(CheckEveryItem)
}

// WithHighThroughput is a convenience option that sets CheckOnCapacity strategy.
// Use when throughput is more important than immediate cancellation response.
func WithHighThroughput() TransformOption {
	return WithCheckStrategy(CheckOnCapacity)
}

// defaultConfig returns a TransformConfig with default values.
func defaultConfig() TransformConfig {
	return TransformConfig{
		BufferSize:    DefaultBufferSize,
		CheckStrategy: CheckEveryItem, // Safe default
	}
}

// applyOptions builds a config by starting with defaults, overlaying any
// context-level config, then applying functional options (highest priority).
func applyOptions(ctx context.Context, opts ...TransformOption) TransformConfig {
	cfg := defaultConfig()

	// Check for stream-level config in context
	if ctxCfg, ok := GetConfig[*TransformConfig](ctx); ok {
		cfg = *ctxCfg
	}

	// Functional options override context config
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
				err = NewPanicError(r)
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
func (m Mapper[IN, OUT]) Apply(s Stream[IN]) Stream[OUT] {
	return m.ApplyWith(context.Background(), s)
}

// ApplyWith transforms a stream using this Mapper with custom options.
func (m Mapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], opts ...TransformOption) Stream[OUT] {
	cfg := applyOptions(ctx, opts...)
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], cfg.BufferSize)
		go func() {
			defer close(outChan)

			dispatch := newInterceptorDispatch(ctx)
			dispatch.invokeNoArg(ctx, StreamStart)
			defer dispatch.invokeNoArg(ctx, StreamEnd)

			if cfg.CheckStrategy == CheckOnCapacity {
				m.runWithCapacityCheck(ctx, s, outChan, cfg.BufferSize, &dispatch)
			} else {
				m.runWithEveryItemCheck(ctx, s, outChan, &dispatch)
			}
		}()
		return outChan
	})
}

// runWithEveryItemCheck processes items checking ctx.Done() on every send.
func (m Mapper[IN, OUT]) runWithEveryItemCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], dispatch *interceptorDispatch) {
	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		resOut, err := m(resIn)
		if err != nil {
			errResult := Err[OUT](err)
			dispatch.invokeOneArg(ctx, ErrorOccurred, err)
			select {
			case <-ctx.Done():
				return
			case outChan <- errResult:
				dispatch.invokeOneArg(ctx, ItemEmitted, errResult)
			}
			continue
		}

		// Dispatch based on result type
		dispatch.invokeResult(ctx, toAnyResult(resOut))

		select {
		case <-ctx.Done():
			return
		case outChan <- resOut:
			dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
		}
	}
}

// runWithCapacityCheck processes items with batched context checks for higher throughput.
func (m Mapper[IN, OUT]) runWithCapacityCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], bufferSize int, dispatch *interceptorDispatch) {
	itemsSinceCheck := 0

	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		resOut, err := m(resIn)
		if err != nil {
			errResult := Err[OUT](err)
			dispatch.invokeOneArg(ctx, ErrorOccurred, err)
			select {
			case <-ctx.Done():
				return
			case outChan <- errResult:
				dispatch.invokeOneArg(ctx, ItemEmitted, errResult)
			}
			itemsSinceCheck = 0
			continue
		}

		// Dispatch based on result type
		dispatch.invokeResult(ctx, toAnyResult(resOut))

		// Check context periodically
		if itemsSinceCheck >= bufferSize {
			select {
			case <-ctx.Done():
				return
			default:
			}
			itemsSinceCheck = 0
		}

		// Try non-blocking send first
		select {
		case outChan <- resOut:
			dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
			itemsSinceCheck++
		default:
			// Channel full - must check context before blocking
			select {
			case <-ctx.Done():
				return
			case outChan <- resOut:
				dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
				itemsSinceCheck = 0
			}
		}
	}
}

// Fuse combines two Mappers into a single Mapper that applies both transformations
// sequentially without an intermediate channel or goroutine. This is an optimization
// for CPU-bound transformations where the overhead of channel synchronization
// outweighs the benefits of concurrency.
//
// Trade-offs:
//   - Faster: No intermediate channel allocation or goroutine synchronization
//   - Sequential: Both transformations run in the same goroutine
//   - No intermediate buffering: Can't apply backpressure between fused stages
//   - No intermediate observation: Can't intercept/log between fused mappers
//
// Use Fuse when:
//   - Both mappers are CPU-bound (no I/O)
//   - You don't need to observe intermediate values
//   - Performance profiling shows channel overhead is significant
func Fuse[IN, MID, OUT any](first Mapper[IN, MID], second Mapper[MID, OUT]) Mapper[IN, OUT] {
	return func(res Result[IN]) (Result[OUT], error) {
		mid, err := first(res)
		if err != nil {
			return Err[OUT](err), nil
		}
		return second(mid)
	}
}

// Predicate is a function that tests a value and returns true if it passes the test.
// It is used for filtering operations.
type Predicate[T any] func(T) bool

// ToFlatMapper converts a Mapper to a FlatMapper that produces exactly one output per input.
func (m Mapper[IN, OUT]) ToFlatMapper() FlatMapper[IN, OUT] {
	return func(res Result[IN]) ([]Result[OUT], error) {
		out, err := m(res)
		if err != nil {
			return []Result[OUT]{Err[OUT](err)}, nil
		}
		return []Result[OUT]{out}, nil
	}
}

// ToFlatMapper converts a Predicate to a FlatMapper that produces one output if the
// predicate passes, or zero outputs if it fails.
func (p Predicate[T]) ToFlatMapper() FlatMapper[T, T] {
	return func(res Result[T]) ([]Result[T], error) {
		if res.IsError() {
			return []Result[T]{res}, nil
		}
		if p(res.Value()) {
			return []Result[T]{res}, nil
		}
		return nil, nil // filtered out
	}
}

// FuseFlat combines two FlatMappers into a single FlatMapper.
// This is the universal fusion function - Mapper and Predicate can be converted
// to FlatMapper using their ToFlatMapper() methods before fusing.
func FuseFlat[IN, MID, OUT any](first FlatMapper[IN, MID], second FlatMapper[MID, OUT]) FlatMapper[IN, OUT] {
	return func(res Result[IN]) ([]Result[OUT], error) {
		mids, err := first(res)
		if err != nil {
			return []Result[OUT]{Err[OUT](err)}, nil
		}
		var outs []Result[OUT]
		for _, mid := range mids {
			midOuts, err := second(mid)
			if err != nil {
				outs = append(outs, Err[OUT](err))
				continue
			}
			outs = append(outs, midOuts...)
		}
		return outs, nil
	}
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
				err = NewPanicError(r)
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
func (fm FlatMapper[IN, OUT]) Apply(s Stream[IN]) Stream[OUT] {
	return fm.ApplyWith(context.Background(), s)
}

// ApplyWith transforms a stream using this FlatMapper with custom options.
func (fm FlatMapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], opts ...TransformOption) Stream[OUT] {
	cfg := applyOptions(ctx, opts...)
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], cfg.BufferSize)
		go func() {
			defer close(outChan)

			dispatch := newInterceptorDispatch(ctx)
			dispatch.invokeNoArg(ctx, StreamStart)
			defer dispatch.invokeNoArg(ctx, StreamEnd)

			if cfg.CheckStrategy == CheckOnCapacity {
				fm.runWithCapacityCheck(ctx, s, outChan, cfg.BufferSize, &dispatch)
			} else {
				fm.runWithEveryItemCheck(ctx, s, outChan, &dispatch)
			}
		}()
		return outChan
	})
}

// runWithEveryItemCheck processes items checking ctx.Done() on every send.
func (fm FlatMapper[IN, OUT]) runWithEveryItemCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], dispatch *interceptorDispatch) {
	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		resOuts, err := fm(resIn)
		if err != nil {
			errResult := Err[OUT](err)
			dispatch.invokeOneArg(ctx, ErrorOccurred, err)
			select {
			case <-ctx.Done():
				return
			case outChan <- errResult:
				dispatch.invokeOneArg(ctx, ItemEmitted, errResult)
			}
			continue
		}
		for _, resOut := range resOuts {
			// Dispatch based on result type
			dispatch.invokeResult(ctx, toAnyResult(resOut))

			select {
			case <-ctx.Done():
				return
			case outChan <- resOut:
				dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
			}
		}
	}
}

// runWithCapacityCheck processes items with batched context checks for higher throughput.
func (fm FlatMapper[IN, OUT]) runWithCapacityCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], bufferSize int, dispatch *interceptorDispatch) {
	itemsSinceCheck := 0

	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		resOuts, err := fm(resIn)
		if err != nil {
			errResult := Err[OUT](err)
			dispatch.invokeOneArg(ctx, ErrorOccurred, err)
			select {
			case <-ctx.Done():
				return
			case outChan <- errResult:
				dispatch.invokeOneArg(ctx, ItemEmitted, errResult)
			}
			itemsSinceCheck = 0
			continue
		}

		for _, resOut := range resOuts {
			// Dispatch based on result type
			dispatch.invokeResult(ctx, toAnyResult(resOut))

			// Check context periodically
			if itemsSinceCheck >= bufferSize {
				select {
				case <-ctx.Done():
					return
				default:
				}
				itemsSinceCheck = 0
			}

			// Try non-blocking send first
			select {
			case outChan <- resOut:
				dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
				itemsSinceCheck++
			default:
				select {
				case <-ctx.Done():
					return
				case outChan <- resOut:
					dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
					itemsSinceCheck = 0
				}
			}
		}
	}
}

// =============================================================================
// Iterator-based FlatMapper
// =============================================================================

// IterFlatMapper defines a function that maps a value of type IN to an iterator
// of Results of type OUT. Unlike FlatMapper, it avoids intermediate slice allocation
// by yielding results directly.
//
// The iterator function is at the lowest level of abstraction in the flow processing
// pipeline. It answers the question: "How are items in the flow reduced or expanded?"
//
// Benefits over FlatMapper:
//   - No intermediate slice allocation
//   - Lazy evaluation - can short-circuit on cancellation
//   - 4-5% faster in benchmarks
type IterFlatMapper[IN, OUT any] func(Result[IN]) iter.Seq[Result[OUT]]

// IterFlatMap creates an IterFlatMapper from a transformation function that returns
// an iterator. This is the most efficient way to implement one-to-many transformations.
func IterFlatMap[IN, OUT any](flatMapFunc func(IN) iter.Seq[OUT]) IterFlatMapper[IN, OUT] {
	return func(res Result[IN]) iter.Seq[Result[OUT]] {
		return func(yield func(Result[OUT]) bool) {
			if res.IsError() {
				yield(Err[OUT](res.Error()))
				return
			}

			// Wrap the user's iterator to convert OUT to Result[OUT]
			// and handle panics
			func() {
				defer func() {
					if r := recover(); r != nil {
						yield(Err[OUT](NewPanicError(r)))
					}
				}()

				for out := range flatMapFunc(res.Value()) {
					if !yield(Ok(out)) {
						return
					}
				}
			}()
		}
	}
}

// IterFlatMapSlice creates an IterFlatMapper from a function that returns a slice.
// This provides the convenience of slice-based logic with the efficiency of iterators.
func IterFlatMapSlice[IN, OUT any](flatMapFunc func(IN) ([]OUT, error)) IterFlatMapper[IN, OUT] {
	return func(res Result[IN]) iter.Seq[Result[OUT]] {
		return func(yield func(Result[OUT]) bool) {
			if res.IsError() {
				yield(Err[OUT](res.Error()))
				return
			}

			// Handle panics from user function
			var outs []OUT
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = NewPanicError(r)
					}
				}()
				outs, err = flatMapFunc(res.Value())
			}()

			if err != nil {
				yield(Err[OUT](err))
				return
			}

			for _, out := range outs {
				if !yield(Ok(out)) {
					return
				}
			}
		}
	}
}

// Apply transforms a stream using this IterFlatMapper with default configuration.
func (ifm IterFlatMapper[IN, OUT]) Apply(s Stream[IN]) Stream[OUT] {
	return ifm.ApplyWith(context.Background(), s)
}

// ApplyWith transforms a stream using this IterFlatMapper with custom options.
func (ifm IterFlatMapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], opts ...TransformOption) Stream[OUT] {
	cfg := applyOptions(ctx, opts...)
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], cfg.BufferSize)
		go func() {
			defer close(outChan)

			dispatch := newInterceptorDispatch(ctx)
			dispatch.invokeNoArg(ctx, StreamStart)
			defer dispatch.invokeNoArg(ctx, StreamEnd)

			if cfg.CheckStrategy == CheckOnCapacity {
				ifm.runWithCapacityCheck(ctx, s, outChan, cfg.BufferSize, &dispatch)
			} else {
				ifm.runWithEveryItemCheck(ctx, s, outChan, &dispatch)
			}
		}()
		return outChan
	})
}

// runWithEveryItemCheck processes items checking ctx.Done() on every send.
func (ifm IterFlatMapper[IN, OUT]) runWithEveryItemCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], dispatch *interceptorDispatch) {
	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		cancelled := false
		for resOut := range ifm(resIn) {
			// Dispatch based on result type
			dispatch.invokeResult(ctx, toAnyResult(resOut))

			select {
			case <-ctx.Done():
				cancelled = true
				return
			case outChan <- resOut:
				dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
			}
		}
		if cancelled {
			return
		}
	}
}

// runWithCapacityCheck processes items with batched context checks for higher throughput.
func (ifm IterFlatMapper[IN, OUT]) runWithCapacityCheck(ctx context.Context, s Stream[IN], outChan chan<- Result[OUT], bufferSize int, dispatch *interceptorDispatch) {
	itemsSinceCheck := 0

	for resIn := range s.Emit(ctx) {
		// Dispatch ItemReceived for the input
		dispatch.invokeOneArg(ctx, ItemReceived, resIn)

		for resOut := range ifm(resIn) {
			// Dispatch based on result type
			dispatch.invokeResult(ctx, toAnyResult(resOut))

			// Check context periodically
			if itemsSinceCheck >= bufferSize {
				select {
				case <-ctx.Done():
					return
				default:
				}
				itemsSinceCheck = 0
			}

			// Try non-blocking send first
			select {
			case outChan <- resOut:
				dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
				itemsSinceCheck++
			default:
				select {
				case <-ctx.Done():
					return
				case outChan <- resOut:
					dispatch.invokeOneArg(ctx, ItemEmitted, resOut)
					itemsSinceCheck = 0
				}
			}
		}
	}
}
