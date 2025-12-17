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
    }).Apply(stream)

    // Double them
    doubled := flow.Map(func(n int) (int, error) {
        return n * 2, nil
    }).Apply(evens)

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
filtered := filter.Where(isEven).Apply(stream)

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
protected := flowerrors.WithCircuitBreaker(cb, operation).Apply(stream)

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
}).Apply(stream)
// Stream continues: Ok(2), Err("skip 2"), Ok(6)

// Log errors without stopping
logged := flowerrors.OnError(func(err error) {
    log.Printf("Error: %v", err)
}).Apply(result)

// Catch and recover from specific errors
recovered := flowerrors.CatchError(
    func(err error) bool { return errors.Is(err, ErrNotFound) },
    func(err error) (int, error) { return 0, nil },  // Replace with default
).Apply(result)

// Retry transient failures
retried := flowerrors.Retry(3, fetchFromAPI).Apply(stream)

// With exponential backoff
retried := flowerrors.RetryWithBackoff(
    3,
    flowerrors.ExponentialBackoff(100*time.Millisecond, 2.0),
    fetchFromAPI,
).Apply(stream)
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

## Stream Observation with Typed Hooks

Observe stream processing with type-safe hooks attached to context:

```go
import (
    "github.com/lguimbarda/min-flow/flow"
    "github.com/lguimbarda/min-flow/flow/core"
    "github.com/lguimbarda/min-flow/flow/observe"
    "github.com/lguimbarda/min-flow/flow/flowerrors"
)

func main() {
    ctx := context.Background()

    // Attach typed hooks to context
    ctx, counter := observe.WithCounter[int](ctx)           // Count items
    ctx = observe.WithValueHook(ctx, func(v int) {          // Observe values
        fmt.Printf("Value: %d\n", v)
    })
    ctx = observe.WithErrorHook(ctx, func(err error) {      // Observe errors
        log.Printf("Error: %v", err)
    })

    // Process stream - hooks fire automatically during Emit()
    stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
    doubled := flow.Map(func(n int) (int, error) {
        return n * 2, nil
    }).Apply(stream)

    _, _ = flow.Slice(ctx, doubled)
    fmt.Printf("Processed %d items\n", counter.Load())
}
```

### Available Hooks

```go
// Observation (observe package)
observe.WithCounter[T](ctx)              // Count processed items
observe.WithValueCounter[T](ctx, fn)     // Count values matching condition
observe.WithValueHook[T](ctx, fn)        // Callback per value
observe.WithErrorHook[T](ctx, fn)        // Callback per error
observe.WithLogging[T](ctx, logFn)       // Log stream events

// Error tracking (flowerrors package)
flowerrors.WithErrorCounter[T](ctx, fn)     // Count errors
flowerrors.WithErrorCollector[T](ctx)       // Collect all errors
flowerrors.WithCircuitBreakerMonitor[T](ctx, threshold, fn)  // Monitor error rate

// Low-level (core package)
core.WithHooks(ctx, core.Hooks[T]{
    OnValue:       func(v T) { ... },
    OnError:       func(err error) { ... },
    OnStreamStart: func() { ... },
    OnStreamEnd:   func() { ... },
})
```

### Hook Composition

Multiple hooks of different types can coexist:

```go
ctx := context.Background()
ctx, intCounter := observe.WithCounter[int](ctx)
ctx, strCounter := observe.WithCounter[string](ctx)
ctx, errCollector := flowerrors.WithErrorCollector[int](ctx)

// All hooks fire during their respective stream processing
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

## Documentation

- **[Package READMEs](flow/)** - Each package has detailed documentation
- **[Architecture Decision Records](docs/adr/)** - Key design decisions and rationale
- **[Examples](examples/)** - Working examples for common use cases
- **[Benchmarks](docs/BENCHMARKS.md)** - Performance analysis

## Project Status & Stability

**ALPHA** – No users yet. Backward compatibility is not guaranteed. Expect rapid iteration and breaking changes. Go version: **1.23+**.

## Quality & CI (recommended gates)

Run these locally and in CI before merging:

- `go vet ./...`
- `staticcheck ./...`
- `go test ./... -count=1`
- `go test ./... -race -short`
- (targeted fuzz) `go test ./flow/... -run=Fuzz -fuzz=.`
- Bench smoke: `go test ./... -run TestBuffer_ContextCancellation -count=5` (timing flake detector)

## Performance & Soak Guidance

Targets (define per deployment):

- Throughput: <TBD> items/sec
- Latency: <TBD> p99 end-to-end
- Allocations: <TBD> allocs/op, <TBD> B/op (steady-state)

Benchmarks exist under `benchmarks/` and `flow/**/` tests. For confidence:

- Throughput/allocs: `go test ./benchmarks -bench=. -benchmem`
- Concurrency & cancellation: soak with slow consumers + `context.WithTimeout`
- Backpressure: exercise buffered vs unbuffered channels in timing/aggregate paths
- GC pressure: run benchmarks with `GODEBUG=gctrace=1` and inspect pauses/p99

Scenarios to cover:

- Fast producer + slow consumer (backpressure)
- Context cancellation mid-stream
- Panic in mapper/flatmapper (recovery to Err)
- Buffered vs unbuffered channels
- Bursty input vs steady input

### Soak & Observability Quick Runs

- Soak harness: `go test ./flow -run TestSoak_ -count=1 -timeout=2s`
- Buffer cancel smoke: `go test ./flow/timing -run TestBuffer_ContextCancellation -count=3`
- Otel hooks sample (noop meter by default): `go test ./flow/observe -run TestOtelHooksIntegration -count=1`

Swap the noop meter in `flow/observe/otel_example_test.go` with your `sdk/metric` provider to emit real metrics; hooks fire synchronously, so keep callbacks lightweight.

## Observability Defaults (typed hooks)

Use hooks rather than interceptors:

```go
ctx := context.Background()
ctx, counter := observe.WithCounter[int](ctx)
ctx = observe.WithLogging[int](ctx, log.Printf)
ctx, errs := flowerrors.WithErrorCollector[int](ctx)

stream := flow.Map(func(n int) (int, error) { return n * 2, nil }).Apply(flow.FromSlice([]int{1,2,3}))
_, _ = flow.Slice(ctx, stream)

fmt.Println("count", counter.Load())
fmt.Println("errors", errs.Errors())
```

For OpenTelemetry/Prometheus, wrap hooks to emit metrics/logs inside the callbacks; hooks run synchronously, so keep callbacks fast.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions welcome! Please read the [contribution guidelines](.github/CONTRIBUTING.md) first.
