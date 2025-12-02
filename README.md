# min-flow

A unified stream processing framework for Go that combines the best patterns from reactive programming with Go's powerful concurrency primitives. Build scalable, observable, and resilient data pipelines with a fluent API.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **Type-Safe Streams**: Fully generic streams with compile-time type checking
- **Composable Transformers**: Chain operations naturally with a fluent API
- **Error Handling**: Built-in `Result[T]` type distinguishes between values, errors, and sentinels
- **Concurrency**: Leverages Go channels for efficient parallel processing
- **Extensible**: Delegate system for interceptors, factories, and resource pools
- **Zero Dependencies**: Core package uses only the standard library

## Installation

```bash
go get github.com/lguimbarda/min-flow
```

Requires Go 1.23 or later.

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/lguimbarda/min-flow/flow"
    "github.com/lguimbarda/min-flow/flow/filter"
)

func main() {
    ctx := context.Background()

    // Create a stream from a slice
    stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

    // Apply transformations
    result := filter.Filter(func(n int) bool {
        return n%2 == 0 // Keep even numbers
    }).Apply(ctx, stream)

    // Map to squares
    doubled := flow.Map(func(n int) (int, error) {
        return n * 2, nil
    }).Apply(ctx, result)

    // Collect results
    values, err := flow.Slice(ctx, doubled)
    if err != nil {
        panic(err)
    }

    fmt.Println(values) // [4 8 12 16 20]
}
```

## Package Structure

The framework is organized into focused packages:

```
flow/
  core/                    # Core abstractions (stdlib only)
  aggregate/               # Batching, reduction, windowing
  combine/                 # Stream merging, fan-out/fan-in
  filter/                  # Filtering, taking, skipping, selection
  timing/                  # Time-based operators (delay, debounce, throttle)
  flowerrors/              # Error handling and resilience
  parallel/                # Parallel/async processing
  observe/                 # Observability, metrics, side-effects
  transform/               # General transformations (distinct, indexed, etc.)
```

## Core Concepts

### Streams

A `Stream[T]` represents a lazy sequence of values that can be consumed exactly once:

```go
// Create streams from various sources
stream := flow.FromSlice([]int{1, 2, 3})
stream := flow.FromChannel(ch)
stream := flow.Range(1, 100)
stream := flow.FromIter(myIterator)
```

### Results

Every item in a stream is wrapped in a `Result[T]` which can be:

- **Value**: A successful processing result (`flow.Ok(value)`)
- **Error**: A recoverable error (`flow.Err[T](err)`) - stream continues
- **Sentinel**: A stream control signal (`flow.Sentinel[T](err)`) - e.g., end of stream

### Transformers

Transformers convert one stream to another:

```go
// Using the Transformer interface
filtered := filter.Filter(predicate).Apply(ctx, stream)

// Chaining transformers
filtered := filter.Filter(isEven).Apply(ctx, stream)
taken := filter.Take[int](10).Apply(ctx, filtered)
distinct := transform.Distinct[int]().Apply(ctx, taken)
```

### Mappers and FlatMappers

Low-level 1:1 and 1:N transformations:

```go
// 1:1 mapping
doubler := flow.Map(func(n int) (int, error) {
    return n * 2, nil
})

// 1:N mapping (can emit 0 or more items per input)
exploder := flow.FlatMap(func(n int) ([]int, error) {
    return []int{n, n * 2, n * 3}, nil
})
```

## Stream Sources

```go
// Slice and Iterator sources
flow.FromSlice([]T{...})
flow.FromIter(seq iter.Seq[T])
flow.FromIter2(seq iter.Seq2[int, T])
flow.FromMap(map[K]V{...})

// Channel sources
flow.FromChannel(ch <-chan T)
flow.FromChannelWithErrors(ch <-chan T, errCh <-chan error)

// Numeric ranges
flow.Range(start, end)
flow.RangeStep(start, end, step)

// Time-based sources
flow.Timer(duration)           // Emits once after duration
flow.TimerValue(duration, val) // Emits value after duration
flow.Interval(duration)        // Emits incrementing values periodically
flow.IntervalWithDelay(d, init)

// Factory functions
flow.Just(value)               // Single value
flow.Repeat(value, count)      // Repeated value
flow.Empty[T]()                // Empty stream
flow.Never[T]()                // Never emits, never completes
flow.FromError[T](err)         // Single error
flow.Defer(func() Stream[T])   // Lazy stream creation
flow.Create(func(ctx, emit, emitError) error) // Imperative creation
flow.Unfold(seed, unfolder)    // State-based generation
flow.Iterate(seed, fn)         // Infinite iteration
flow.IterateN(seed, fn, n)     // Limited iteration
```

## Operators

### Filtering (`flow/filter`)

```go
filter.Filter(predicate)        // Keep matching items
filter.FilterWithIndex(pred)    // With index
filter.Take[T](n)               // First n items
filter.TakeLast[T](n)           // Last n items
filter.Skip[T](n)               // Skip first n
filter.SkipLast[T](n)           // Skip last n
filter.TakeWhile(pred)          // While predicate true
filter.SkipWhile(pred)          // Until predicate false
filter.TakeUntil(notifier)      // Until notifier emits
filter.SkipUntil(notifier)      // After notifier emits
filter.Find(predicate)          // First matching
filter.Contains(value)          // Contains value?
filter.ElementAt[T](index)      // Element at index
filter.Single(predicate)        // Exactly one matching
```

### Aggregation (`flow/aggregate`)

```go
aggregate.Reduce(initial, reducer) // Single accumulated value
aggregate.Scan(initial, reducer)   // Running accumulation
aggregate.Count[T]()               // Count items
aggregate.Sum[T]()                 // Numeric sum
aggregate.Average[T]()             // Numeric average
aggregate.Min[T]()                 // Minimum value
aggregate.Max[T]()                 // Maximum value
aggregate.All(predicate)           // All match?
aggregate.Any(predicate)           // Any match?
aggregate.None(predicate)          // None match?
aggregate.Batch[T](size)           // Fixed-size batches
aggregate.BatchTimeout(size, dur)  // With timeout
aggregate.Window[T](size)          // Sliding window
aggregate.WindowTime[T](duration)  // Time-based window
aggregate.GroupBy(keyFunc)         // Group by key
```

### Timing (`flow/timing`)

```go
timing.Delay[T](duration)          // Delay all items
timing.Debounce[T](duration)       // Debounce emissions
timing.Throttle[T](duration)       // Rate limiting
timing.Timeout[T](duration)        // Timeout per item
timing.Sample[T](duration)         // Periodic sampling
timing.Buffer[T](size)             // Rolling buffer
timing.BufferTime[T](duration)     // Time-based buffer
```

### Parallel Processing (`flow/parallel`)

```go
parallel.Parallel(n, mapper)       // Unordered parallel map
parallel.ParallelOrdered(n, mapper) // Ordered results
parallel.ParallelMap(n, mapper)    // Parallel mapping
parallel.ParallelFlatMap(n, fm)    // Parallel flat map
parallel.AsyncMap(mapper)          // Async mapping
parallel.AsyncMapOrdered(mapper)   // Ordered async
```

### Error Handling & Resilience (`flow/flowerrors`)

```go
flowerrors.Retry[T](maxAttempts)   // Retry on error
flowerrors.RetryWithBackoff(conf)  // Exponential backoff
flowerrors.CircuitBreaker[T](conf) // Circuit breaker pattern
flowerrors.OnError(handler)        // Error handling
flowerrors.CatchError(handler)     // Catch and handle errors
flowerrors.MapErrors(fn)           // Transform errors
flowerrors.FilterErrors(pred)      // Filter errors
flowerrors.Fallback(value)         // Fallback on error
```

### Fan-out/Fan-in & Combination (`flow/combine`)

```go
combine.FanOut(n, stream)          // Split to n streams
combine.FanIn(streams...)          // Merge streams
combine.Broadcast(n, stream)       // Broadcast to all
combine.RoundRobin(n, stream)      // Round-robin distribution
combine.Balance(n, stream)         // Load balancing
combine.Tee(n, stream)             // Tee to multiple consumers
combine.PartitionStream(pred)      // Split by predicate
combine.Merge(streams...)          // Interleaved merge
combine.Concat(streams...)         // Sequential concat
combine.Zip(s1, s2)                // Pair elements
combine.ZipWith(s1, s2, fn)        // Combine with function
combine.Interleave(streams...)     // Round-robin interleave
combine.CombineLatest(streams...)  // Latest from each
combine.Race(streams...)           // First to emit wins
```

### Transform & Utility (`flow/transform`)

```go
transform.Distinct[T]()            // Remove duplicates
transform.DistinctBy(keyFunc)      // By key
transform.WithIndex[T]()           // Add index to items
transform.Pairwise[T]()            // Emit consecutive pairs
transform.DefaultIfEmpty(value)    // Default if empty
transform.Collect[T]()             // Collect to slice
transform.CollectMap(keyFunc)      // Collect to map
```

### Observability (`flow/observe`)

```go
observe.Do(action)                 // Side effect per item
observe.DoOnError(handler)         // On error side effect
observe.Tap(observer)              // Observe without changing
observe.Log[T](prefix)             // Debug logging
observe.Metrics[T]()               // Collect stream metrics
observe.Progress(total, handler)   // Progress reporting
```

operators.FindIndex(predicate) // Index of first match
operators.FindLast(predicate) // Last matching
operators.FindLastIndex(predicate) // Index of last match
operators.Contains(value) // Contains value?
operators.ContainsBy(predicate) // Contains matching?
operators.IsEmpty[T]() // Stream empty?
operators.IsNotEmpty[T]() // Stream not empty?
operators.ElementAt[T](index) // Element at index
operators.Single(predicate) // Exactly one matching
operators.SequenceEqual(other) // Streams equal?

````

### Utility

```go
operators.Do(action)               // Side effect per item
operators.DoOnError(handler)       // On error side effect
operators.Tap(observer)            // Observe without changing
operators.Log[T](prefix)           // Debug logging
operators.Materialize[T]()         // Wrap in Results
operators.Dematerialize[T]()       // Unwrap Results
operators.DefaultIfEmpty(value)    // Default if empty
operators.SwitchIfEmpty(alt)       // Alternative if empty
operators.StartWith(values...)     // Prepend values
operators.EndWith(values...)       // Append values
````

## Composition

Chain transformers elegantly:

```go
// Using Pipe
pipeline := flow.Pipe(
    operators.Filter(isValid),
    operators.Take[User](100),
    operators.Distinct[User](),
)
result := pipeline.Apply(ctx, users)

// Using Chain (for same types)
pipeline := flow.Chain(
    operators.Filter(isAdult),
    operators.Skip[Person](10),
    operators.Take[Person](20),
)

// Using Apply for sequential application
result := flow.Apply(
    ctx,
    stream,
    operators.Filter(pred1),
    operators.Filter(pred2),
)
```

## Error Handling

```go
// Errors are wrapped in Results
stream := flow.FromSlice([]int{1, 2, 3})
result := flow.Map(func(n int) (int, error) {
    if n == 2 {
        return 0, errors.New("skip 2")
    }
    return n * 2, nil
}).Apply(ctx, stream)

// Handle errors
handled := operators.OnError(func(err error) {
    log.Printf("Error: %v", err)
}).Apply(ctx, result)

// Or use MapErrors
remapped := operators.MapErrors(func(err error) error {
    return fmt.Errorf("wrapped: %w", err)
}).Apply(ctx, result)
```

## Terminal Operations

Consume streams to get final results:

```go
// Collect all values
values, err := flow.Slice(ctx, stream)

// Get first value
value, err := flow.First(ctx, stream)

// Run for side effects only
err := flow.Run(ctx, stream)
```

## Context and Cancellation

All operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Stream will terminate when context is cancelled
values, err := flow.Slice(ctx, operators.Filter(slowPredicate).Apply(ctx, stream))
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Operation timed out")
}
```

## Delegate System

Extend functionality with delegates:

```go
// Register an interceptor
registry := core.NewRegistry()
registry.Register(myInterceptor)

// Propagate via context
ctx := core.WithRegistry(ctx, registry)

// Interceptors receive events
type MyInterceptor struct{}

func (i *MyInterceptor) Init() error { return nil }
func (i *MyInterceptor) Close() error { return nil }
func (i *MyInterceptor) OnEvent(event core.Event, data any) {
    switch event {
    case core.StreamStart:
        log.Println("Stream started")
    case core.StreamEnd:
        log.Println("Stream ended")
    }
}
```

## Project Status

**ALPHA** - This project is under active development with no backward compatibility guarantees. Breaking changes are expected as the API evolves.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the [contribution guidelines](.github/CONTRIBUTING.md) first.
