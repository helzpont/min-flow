# min-flow MVP

A lightweight stream processing framework for Go with built-in observability via typed hooks. Build composable, observable data pipelines with a fluent API.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.23-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

## Features

- **Type-Safe Streams**: Fully generic streams with compile-time type checking
- **Composable Transformers**: Chain operations naturally with a fluent API
- **Error Handling**: Built-in `Result[T]` type distinguishes between values, errors, and sentinels
- **Typed Hooks**: Register type-safe callbacks for stream events (values, errors, start, complete)
- **Observable**: Built-in metrics, logging, and error collection via typed hooks
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
)

func main() {
    ctx := context.Background()

    // Create a stream from a slice
    stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

    // Double each value
    doubled := flow.Map(func(n int) (int, error) {
        return n * 2, nil
    }).Apply(stream)

    // Collect results
    values, err := flow.Slice(ctx, doubled)
    if err != nil {
        panic(err)
    }

    fmt.Println(values) // [2 4 6 8 10]
}
```

## Package Structure

```
flow/
├── core/           # Core abstractions (stdlib only, no external deps)
│   ├── result.go       # Result[T] type with Ok/Err/Sentinel
│   ├── stream.go       # Stream and Transformer interfaces
│   ├── channel.go      # Emitter and Transmitter
│   ├── map.go          # Mapper and FlatMapper
│   ├── terminal.go     # Sink type and terminal operations
│   ├── delegate.go     # Delegate system and Registry
│   └── hooks.go        # Typed hooks system
├── types.go        # Type aliases and constructors
├── source.go       # Stream sources (FromSlice, Range, etc.)
├── compose.go      # Pipeline composition (Pipe, Chain, Through)
├── observe/        # Observability via interceptors
│   ├── register.go     # Interceptor registration functions
│   ├── lifecycle.go    # Notification types
│   └── observe.go      # Metrics types
└── flowerrors/     # Error handling
    ├── register.go     # Error interceptor registration
    └── error.go        # Error transformers
```

## Core Concepts

### Streams

A `Stream[T]` represents a lazy sequence of values:

```go
// Create streams from various sources
stream := flow.FromSlice([]int{1, 2, 3})
stream := flow.FromChannel(ch)
stream := flow.Range(1, 100)
stream := flow.FromIter(myIterator)  // Go 1.23+ iter.Seq

// Other sources
stream := flow.Empty[int]()           // Empty stream
stream := flow.Once(42)               // Single value
stream := flow.Generate(seedFn, genFn) // Generator function
stream := flow.Repeat(value, count)   // Repeat value n times
stream := flow.Interval(duration)     // Tick every duration
```

### Results

Every item in a stream is wrapped in a `Result[T]`:

| State        | Description                          | Constructor             |
| ------------ | ------------------------------------ | ----------------------- |
| **Value**    | Successful result                    | `flow.Ok(value)`        |
| **Error**    | Recoverable error (stream continues) | `flow.Err[T](err)`      |
| **Sentinel** | Control signal (end of stream)       | `flow.Sentinel[T](err)` |

```go
result := flow.Ok(42)
if result.IsValue() {
    fmt.Println(result.Value()) // 42
}

result := flow.Err[int](errors.New("failed"))
if result.IsError() {
    fmt.Println(result.Error()) // failed
}
```

### Transformers

Transformers convert streams from one type to another:

```go
// Apply a single transformer
doubled := flow.Map(func(n int) (int, error) {
    return n * 2, nil
}).Apply(stream)

// Chain multiple transformers of the same type
result := flow.Pipe(ctx, stream,
    mapper1,
    mapper2,
    mapper3,
)

// Compose transformers of different types
composed := flow.Through(intToString, stringToBytes)
```

### Mappers

Low-level transformation functions with panic recovery:

```go
// 1:1 mapping
doubler := flow.Map(func(n int) (int, error) {
    return n * 2, nil
})

// 1:N mapping (one input -> multiple outputs)
exploder := flow.FlatMap(func(n int) ([]int, error) {
    return []int{n, n * 2, n * 3}, nil
})

// Fusion for performance (eliminates intermediate channels)
fused := flow.Fuse(mapper1, mapper2)
fusedFlat := flow.FuseFlat(flatMapper1, flatMapper2)
```

## Typed Hooks System

Typed hooks provide a powerful way to observe stream events without modifying the stream itself. Register type-safe callbacks that are automatically invoked as items flow through transformers.

### Setting Up Hooks

```go
ctx := context.Background()

// Attach typed hooks to context
ctx = observe.WithValueHook[int](ctx, func(value int) {
    fmt.Printf("Received value: %d\n", value)
})

ctx = observe.WithErrorHook[int](ctx, func(err error) {
    log.Printf("Error occurred: %v", err)
})

ctx = observe.WithStartHook[int](ctx, func() {
    fmt.Println("Stream started")
})

ctx = observe.WithCompleteHook[int](ctx, func() {
    fmt.Println("Stream completed")
})

// Use the context - hooks are automatically invoked during Emit()
stream := flow.FromSlice([]int{1, 2, 3})
doubled := flow.Map(func(n int) (int, error) {
    return n * 2, nil
}).Apply(stream)

values, _ := flow.Slice(ctx, doubled)
```

### Built-in Hook Helpers

#### Observability (`flow/observe`)

```go
// Value/Error callbacks
ctx = observe.WithValueHook[T](ctx, func(value T) { ... })
ctx = observe.WithErrorHook[T](ctx, func(err error) { ... })

// Lifecycle callbacks
ctx = observe.WithStartHook[T](ctx, func() { ... })
ctx = observe.WithCompleteHook[T](ctx, func() { ... })
ctx = observe.WithSentinelHook[T](ctx, func(err error) { ... })

// Counting (returns context and counter)
ctx, counter := observe.WithCounter[T](ctx)
// After stream: counter.Values(), counter.Errors(), counter.Total()

ctx, valueCounter := observe.WithValueCounter[T](ctx)
// After stream: valueCounter.Count()

// Error collection
ctx, errCollector := observe.WithErrorCollector[T](ctx)
// After stream: errCollector.Errors(), errCollector.HasErrors()

// Logging
ctx = observe.WithLogging[T](ctx, log.Printf)
```

#### Low-level Hooks (`flow/core`)

```go
// Full hooks structure for complete control
ctx = core.WithHooks(ctx, core.Hooks[int]{
    OnStart:    func() { ... },
    OnValue:    func(v int) { ... },
    OnError:    func(err error) { ... },
    OnSentinel: func(err error) { ... },
    OnComplete: func() { ... },
})
```

### Hook Composition

Multiple hooks of different types can coexist:

```go
ctx := context.Background()
ctx, intCounter := observe.WithCounter[int](ctx)
ctx, strCounter := observe.WithCounter[string](ctx)
ctx, errCollector := observe.WithErrorCollector[int](ctx)

// All hooks fire during their respective stream processing
```

## Error Handling

### Error Transformers

```go
// Catch and handle errors
handled := flowerrors.CatchError(
    func(err error) bool { return true },  // Match all errors
    func(err error) (int, error) {
        return 0, nil  // Provide fallback value
    },
).Apply(ctx, stream)

// Filter errors based on predicate
filtered := flowerrors.FilterErrors[int](func(err error) bool {
    return errors.Is(err, ErrRetryable)
}).Apply(ctx, stream)

// Ignore all errors (only emit values)
valuesOnly := flowerrors.IgnoreErrors[int]().Apply(ctx, stream)

// Transform errors
mapped := flowerrors.MapErrors[int](func(err error) error {
    return fmt.Errorf("wrapped: %w", err)
}).Apply(ctx, stream)

// Wrap all errors with additional context
wrapped := flowerrors.WrapError[int]("processing failed").Apply(ctx, stream)

// Extract only errors (for error-focused processing)
errorsOnly := flowerrors.ErrorsOnly[int]().Apply(ctx, stream)

// Convert errors to sentinel to stop stream
throwing := flowerrors.ThrowOnError[int]().Apply(ctx, stream)
```

### Materialize/Dematerialize

Convert between values and notification types:

```go
// Materialize: convert stream items to Notification objects
notifications := observe.MaterializeNotification[int]().Apply(ctx, stream)

// Dematerialize: convert Notification objects back to stream items
items := observe.DematerializeNotification[int]().Apply(ctx, notifications)
```

## Terminal Operations

Consume streams to produce final results:

```go
// Collect all values into a slice
values, err := flow.Slice(ctx, stream)

// Get first value only
first, err := flow.First(ctx, stream)

// Run for side effects (discards values)
err := flow.Run(ctx, stream)

// Collect all Results (including errors)
results := flow.Collect(ctx, stream)

// Iterate with Go 1.23 iterators
for result := range flow.All(ctx, stream) {
    if result.IsValue() {
        fmt.Println(result.Value())
    }
}
```

### Sink Type

Sinks consume streams and produce terminal results:

```go
// Create sinks
toSlice := flow.ToSlice[int]()
toFirst := flow.ToFirst[int]()
toRun := flow.ToRun[int]()

// Use directly (primary method)
values, err := toSlice.From(ctx, stream)

// Lazy evaluation with Defer - returns a thunk you call later
thunk := toSlice.Defer(stream)
// ... define more pipeline ...
values, err := thunk(ctx)  // Execute when ready

// Compose in a pipeline (wraps result in single-element stream)
resultStream := toSlice.Apply(stream)
```

## Composition

### Pipe

Apply multiple same-type transformers:

```go
result := flow.Pipe(ctx, stream,
    mapper1,
    mapper2,
    mapper3,
)
```

### Chain

Compose transformers into a single transformer:

```go
combined := flow.Chain(
    mapper1,
    mapper2,
    mapper3,
)
result := combined.Apply(stream)
```

### Through

Compose transformers of different types:

```go
// int -> string -> []byte
composed := flow.Through(
    flow.Map(strconv.Itoa),
    flow.Map(func(s string) ([]byte, error) { return []byte(s), nil }),
)
result := composed.Apply(intStream)
```

## Roadmap

This MVP provides the foundation for stream processing with interceptors. Future releases will add:

### Phase 2: Filtering & Aggregation

- `flow/filter` - Filtering operators (Where, Take, Skip, First, Last, Distinct)
- `flow/aggregate` - Aggregation operators (Reduce, Fold, Scan, Batch, Window, GroupBy)

### Phase 3: Stream Combination

- `flow/combine` - Merging and combining streams (Merge, Concat, Zip, FanOut, FanIn, Race)

### Phase 4: Parallel Processing

- `flow/parallel` - Concurrent processing (parallel Map, FlatMap, ForEach with worker pools)

### Phase 5: Timing & Rate Control

- `flow/timing` - Timing operators (Delay, Debounce, Throttle, Timeout, Sample, RateLimit)

### Phase 6: Resilience Patterns

- Retry with backoff strategies (constant, exponential, jitter)
- Circuit breaker pattern
- Timeout handling

### Phase 7: I/O Integrations

- `flow/csv` - CSV file processing
- `flow/json` - JSON streaming
- `flow/http` - HTTP request/response streams
- `flow/sql` - Database query streams
- `flow/io` - File and reader/writer streams
- `flow/glob` - File glob patterns

## Design Philosophy

1. **Viability First**: Robust and reliable stream processing users can trust
2. **Developer Experience**: Clear APIs, helpful errors, progressive learning curve
3. **Go Idiomatic**: Leverage Go's concurrency primitives (channels, goroutines)
4. **Extensibility**: Typed hooks system for observation and customization
5. **Performance**: Efficient by default, with optimization options (Fuse, buffering)

## Contributing

Contributions are welcome! Please read the contribution guidelines before submitting PRs.

## License

MIT License - see [LICENSE](LICENSE) for details.
