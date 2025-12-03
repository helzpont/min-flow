# min-flow

A unified stream processing framework for Go that combines the best patterns from reactive programming with Go's powerful concurrency primitives. Build scalable, observable, and resilient data pipelines with a fluent API.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **Type-Safe Streams**: Fully generic streams with compile-time type checking
- **Composable Transformers**: Chain operations naturally with a fluent API
- **Error Handling**: Built-in `Result[T]` type distinguishes between values, errors, and sentinels
- **Resilience**: Retry, backoff, circuit breaker, and timeout patterns
- **Concurrency**: Leverages Go channels for efficient parallel processing
- **Extensible**: Delegate system for interceptors, factories, and resource pools
- **Observable**: Built-in metrics, tracing, and debugging support
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

    // Filter even numbers
    evens := filter.Where(func(n int) bool {
        return n%2 == 0
    }).Apply(ctx, stream)

    // Double them
    doubled := flow.Map(func(n int) (int, error) {
        return n * 2, nil
    }).Apply(ctx, evens)

    // Collect results
    values, err := flow.Slice(ctx, doubled)
    if err != nil {
        panic(err)
    }

    fmt.Println(values) // [4 8 12 16 20]
}
```

## Package Structure

```
flow/
├── core/           # Core abstractions (stdlib only, no external deps)
├── aggregate/      # Batching, reduction, windowing
├── combine/        # Stream merging, splitting, fan-out/fan-in
├── filter/         # Filtering, taking, skipping, sampling
├── flowerrors/     # Error handling, retry, circuit breaker
├── observe/        # Metrics, tracing, debugging
├── parallel/       # Concurrent processing with worker pools
├── timing/         # Delays, throttling, debouncing
└── transform/      # Utility transformations (distinct, pairwise, etc.)
```

Each package has its own README with detailed documentation and examples.

## Core Concepts

### Streams

A `Stream[T]` represents a lazy sequence of values:

```go
// Create streams from various sources
stream := flow.FromSlice([]int{1, 2, 3})
stream := flow.FromChannel(ch)
stream := flow.Range(1, 100)
stream := flow.FromIter(myIterator)  // Go 1.23+ iter.Seq
```

### Results

Every item in a stream is wrapped in a `Result[T]`:

| State        | Description                          | Constructor             |
| ------------ | ------------------------------------ | ----------------------- |
| **Value**    | Successful result                    | `flow.Ok(value)`        |
| **Error**    | Recoverable error (stream continues) | `flow.Err[T](err)`      |
| **Sentinel** | Control signal (end of stream)       | `flow.Sentinel[T](err)` |

### Transformers

Transformers convert streams from one type to another:

```go
// Apply a single transformer
filtered := filter.Where(isEven).Apply(ctx, stream)

// Chain multiple transformers
result := flow.Pipe(ctx, stream,
    filter.Where(isValid),
    filter.Take[int](10),
    transform.Distinct[int](),
)
```

### Mappers

Low-level transformation functions with panic recovery:

```go
// 1:1 mapping
doubler := flow.Map(func(n int) (int, error) {
    return n * 2, nil
})

// 1:N mapping
exploder := flow.FlatMap(func(n int) ([]int, error) {
    return []int{n, n * 2, n * 3}, nil
})

// Fusion for performance (eliminates intermediate channels)
fused := flow.Fuse(mapper1, mapper2)
```

## Key Operators

### Filtering (`flow/filter`)

```go
filter.Where(predicate)      // Keep matching items
filter.Take[T](n)            // First n items
filter.Skip[T](n)            // Skip first n
filter.TakeWhile(pred)       // While predicate true
filter.Distinct[T]()         // Remove consecutive duplicates
filter.First[T]()            // First item only
filter.Last[T]()             // Last item only
filter.SampleWith(sampler)   // Sample when sampler emits
```

### Aggregation (`flow/aggregate`)

```go
aggregate.Reduce(reducer)           // Reduce to single value
aggregate.Fold(initial, folder)     // Fold with initial value
aggregate.Scan(initial, scanner)    // Running accumulation
aggregate.Batch[T](size)            // Fixed-size batches
aggregate.BatchTimeout(size, dur)   // Batches with timeout
aggregate.WindowTime[T](duration)   // Time-based windows
aggregate.GroupBy(keyFunc)          // Group by key
```

### Combining (`flow/combine`)

```go
combine.Merge(streams...)           // Interleaved merge
combine.Concat(streams...)          // Sequential concat
combine.Zip(s1, s2)                 // Pair elements
combine.FanOut(n, stream)           // Split to n streams
combine.FanIn(streams...)           // Merge streams
combine.CombineLatest(streams...)   // Latest from each
combine.Race(streams...)            // First to emit wins
```

### Parallel Processing (`flow/parallel`)

```go
parallel.Map(n, mapper)             // Parallel map (unordered)
parallel.MapOrdered(n, mapper)      // Parallel map (ordered)
parallel.FlatMap(n, flatMapper)     // Parallel flat map
parallel.ForEach(n, action)         // Parallel side effects
```

### Timing (`flow/timing`)

```go
timing.Delay[T](duration)           // Delay all items
timing.Debounce[T](duration)        // Wait for silence
timing.Throttle[T](duration)        // Rate limiting
timing.Timeout[T](duration)         // Timeout per item
timing.Sample[T](duration)          // Periodic sampling
timing.RateLimit[T](rate, burst)    // Token bucket limiting
```

### Error Handling & Resilience (`flow/flowerrors`)

```go
// Error observation and handling
flowerrors.OnError(handler)                    // Side effect on error
flowerrors.CatchError(predicate, handler)      // Catch specific errors
flowerrors.OnErrorReturn(defaultValue)         // Replace error with value
flowerrors.OnErrorResumeNext(fallbackStream)   // Switch to fallback

// Retry patterns
flowerrors.Retry(maxAttempts, operation)       // Simple retry
flowerrors.RetryWithBackoff(max, backoff, op)  // With backoff strategy

// Backoff strategies
flowerrors.ConstantBackoff(delay)
flowerrors.ExponentialBackoff(initial, multiplier)
flowerrors.ExponentialJitterBackoff(initial, multiplier, jitter)

// Circuit breaker
cb := flowerrors.NewCircuitBreaker(flowerrors.CircuitBreakerConfig{
    FailureThreshold: 5,
    SuccessThreshold: 2,
    Timeout:          30 * time.Second,
})
protected := flowerrors.WithCircuitBreaker(cb, operation).Apply(ctx, stream)

// Timeout
flowerrors.Timeout(duration, operation)
```

### Observability (`flow/observe`)

```go
observe.Tap(action)            // Side effect per item
observe.TapError(handler)      // Side effect on error
observe.Trace[T](prefix)       // Debug logging
observe.Meter(onComplete)      // Collect stream metrics
observe.DoOnComplete(action)   // On completion callback
observe.DoFinally(action)      // Always called (complete/error/cancel)
```

## Terminal Operations

Consume streams to produce final results:

```go
values, err := flow.Slice(ctx, stream)   // Collect all values
first, err := flow.First(ctx, stream)    // First value only
err := flow.Run(ctx, stream)             // Run for side effects
results := flow.Collect(ctx, stream)     // Collect all Results

// Using Go 1.23+ range
for result := range flow.All(ctx, stream) {
    if result.IsValue() {
        process(result.Value())
    }
}
```

## Error Handling

Errors in min-flow are values, not exceptions. They flow through the stream and can be handled at any point:

```go
// Errors from mappers become error Results
stream := flow.FromSlice([]int{1, 2, 3})
result := flow.Map(func(n int) (int, error) {
    if n == 2 {
        return 0, errors.New("skip 2")
    }
    return n * 2, nil
}).Apply(ctx, stream)
// Stream continues: Ok(2), Err("skip 2"), Ok(6)

// Log errors without stopping
logged := flowerrors.OnError(func(err error) {
    log.Printf("Error: %v", err)
}).Apply(ctx, result)

// Catch and recover from specific errors
recovered := flowerrors.CatchError(
    func(err error) bool { return errors.Is(err, ErrNotFound) },
    func(err error) (int, error) { return 0, nil },  // Replace with default
).Apply(ctx, result)

// Retry transient failures
retried := flowerrors.Retry(3, fetchFromAPI).Apply(ctx, stream)

// With exponential backoff
retried := flowerrors.RetryWithBackoff(
    3,
    flowerrors.ExponentialBackoff(100*time.Millisecond, 2.0),
    fetchFromAPI,
).Apply(ctx, stream)
```

## Context and Cancellation

All operations respect context cancellation:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

values, err := flow.Slice(ctx, stream)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Operation timed out")
}
```

## Delegate System

Extend functionality with the delegate registry:

```go
// Create registry and add to context
ctx, registry := core.WithRegistry(context.Background())

// Register interceptors for cross-cutting concerns
registry.Register(&MetricsInterceptor{})
registry.Register(&LoggingInterceptor{})

// Register configuration
registry.Register(&parallel.ParallelConfig{Workers: 8})
registry.Register(&aggregate.AggregateConfig{BatchSize: 100})

// Interceptor example
type MetricsInterceptor struct {
    count atomic.Int64
}

func (m *MetricsInterceptor) Events() []core.Event {
    return []core.Event{core.ValueReceived}
}

func (m *MetricsInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
    m.count.Add(1)
    return nil
}
```

## Development

```bash
# Build
make build

# Run tests with race detector
make test

# Run benchmarks
make bench

# Format, lint, and test
make check

# Run an example
make run EXAMPLE=basic
```

## Project Status

**ALPHA** - This project is under active development with no backward compatibility guarantees. Breaking changes are expected as the API evolves toward a stable release.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the [contribution guidelines](.github/CONTRIBUTING.md) first.
