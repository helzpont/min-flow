# Min-Flow vs Competitors

This document compares min-flow against popular Go stream processing and resilience libraries, highlighting our unique value propositions.

## Quick Comparison Matrix

| Feature                 | min-flow    | RxGo        | go-linq | samber/lo | avast/retry-go | sony/gobreaker |
| ----------------------- | ----------- | ----------- | ------- | --------- | -------------- | -------------- |
| **Stars**               | —           | 5.1k        | 3.6k    | 19k+      | 2.8k           | 3.4k           |
| **Paradigm**            | Streams     | Observables | LINQ    | Utilities | Functions      | Functions      |
| **Stream Processing**   | ✅          | ✅          | ✅      | ❌        | ❌             | ❌             |
| **Channel-based**       | ✅          | ✅          | ❌      | ❌        | ❌             | ❌             |
| **iter.Seq Support**    | ✅          | ❌          | ✅      | ❌        | ❌             | ❌             |
| **Retry**               | ✅          | ✅          | ❌      | ✅        | ✅             | ❌             |
| **Exponential Backoff** | ✅          | ✅          | ❌      | ✅        | ✅             | ❌             |
| **Circuit Breaker**     | ✅          | ❌          | ❌      | ❌        | ❌             | ✅             |
| **Interceptor Pattern** | ✅          | ❌          | ❌      | ❌        | ❌             | ❌             |
| **Event System**        | ✅          | ❌          | ❌      | ❌        | ❌             | ❌             |
| **Transformer Fusion**  | ✅          | ❌          | ❌      | ❌        | ❌             | ❌             |
| **Zero Dependencies**   | ✅ (core)   | ❌          | ❌      | ❌        | ❌             | ❌             |
| **Generics**            | ✅          | ✅          | ✅      | ✅        | ✅             | ✅             |
| **Parallel Processing** | ✅          | ✅          | ❌      | ✅        | ❌             | ❌             |
| **Observability**       | ✅ Built-in | ❌          | ❌      | ❌        | ✅ Callback    | ✅ Callback    |

## Unique Value Propositions

### 1. Unified Stream + Resilience + Observability

**The Problem**: Most Go projects cobble together multiple libraries:

- Stream processing: RxGo or go-linq
- Retry logic: avast/retry-go or cenkalti/backoff
- Circuit breaker: sony/gobreaker
- Metrics: custom code

**Min-flow's Solution**: Everything in one coherent framework.

```go
// min-flow: All-in-one
stream := flow.FromSlice(urls)
results := flow.Pipe(
    flowerrors.RetryWithBackoff(flowerrors.RetryConfig{...}),
    parallel.Parallel(10, fetcher),
    observe.Metrics[Response](),
).Apply(ctx, stream)

// vs. typical Go approach: 3+ libraries, manual integration
cb := gobreaker.NewCircuitBreaker[[]byte](settings)
for _, url := range urls {
    body, err := retry.Do(func() ([]byte, error) {
        return cb.Execute(func() ([]byte, error) {
            return fetch(url)
        })
    }, retry.Attempts(3), retry.Delay(100*time.Millisecond))
    // ... manual metrics tracking
}
```

### 2. Interceptor Pattern with Event System

**The Problem**: Cross-cutting concerns (logging, metrics, error tracking) require modifying every pipeline or wrapping functions.

**Min-flow's Solution**: Register interceptors once, they apply automatically.

```go
// Register interceptors once
registry := core.NewRegistry()
registry.Register(observe.NewMetricsInterceptor(reportMetrics))
registry.Register(flowerrors.NewErrorCollectorInterceptor())
registry.Register(observe.NewLogInterceptor(logger))

ctx := core.WithRegistry(ctx, registry)

// All streams automatically get metrics, error collection, and logging
stream1 := flow.FromSlice(data1)
stream2 := flow.FromChannel(ch)
// Both streams are observed without modifying the pipeline
```

**Event Types**:

- `stream:start`, `stream:end` - Stream lifecycle
- `item:received`, `item:emitted` - Every item
- `value:received` - Successful values only
- `error:occurred` - Errors only
- `sentinel:received` - Control signals

**Pattern Matching**:

```go
// Match specific events
interceptor.Events() // ["error:occurred"]

// Match wildcards
"stream:*"  // All stream events
"*:start"   // All start events
"*"         // All events
```

### 3. Native Go Idioms

**The Problem**: RxGo brings ReactiveX patterns that feel foreign to Go developers.

**Min-flow's Solution**: Built on Go's native concurrency primitives.

```go
// RxGo: Observable pattern (familiar to RxJS/RxJava developers)
observable := rxgo.Just(1, 2, 3)().
    Map(func(_ context.Context, i interface{}) (interface{}, error) {
        return i.(int) * 2, nil
    })

// min-flow: Channels + context (familiar to Go developers)
stream := flow.FromSlice([]int{1, 2, 3})
doubled := flow.Map(func(n int) (int, error) {
    return n * 2, nil
}).Apply(ctx, stream)
```

**Go 1.23+ iter.Seq Integration**:

```go
// Seamlessly work with Go's new iterator protocol
seq := slices.Values(data)
stream := flow.FromIter(seq)

// And back to iterators
iter := stream.All()
for v := range iter {
    // ...
}
```

### 4. Result Type for Explicit Error Handling

**The Problem**: Streams that silently drop errors or require try/catch patterns.

**Min-flow's Solution**: `Result[T]` makes errors first-class citizens.

```go
// Every item is a Result: Value, Error, or Sentinel
for result := range stream.Emit(ctx) {
    switch {
    case result.IsSentinel():
        // End of stream or control signal
    case result.IsErr():
        // Handle error, stream continues
        log.Printf("Error: %v", result.Err())
    default:
        // Process value
        process(result.Value())
    }
}
```

### 5. Transformer Fusion

**The Problem**: Each transformation stage creates a new goroutine and channel, adding overhead.

**Min-flow's Solution**: Fuse compatible transformers into a single stage.

```go
// Without fusion: 3 goroutines, 3 channels
s1 := addOne.Apply(ctx, stream)
s2 := double.Apply(ctx, s1)
s3 := addTen.Apply(ctx, s2)

// With fusion: 1 goroutine, 1 channel, same result
fused := core.Fuse(core.Fuse(addOne, double), addTen)
result := fused.Apply(ctx, stream)
```

**Benchmark Impact** (10,000 items):

- Unfused: ~1.2ms
- Fused: ~0.4ms (3x faster)

### 6. Zero-Dependency Core

**The Problem**: Framework dependencies cascade through your project.

**Min-flow's Solution**: Core package uses only the standard library.

```
flow/core/        # Zero external dependencies
├── channel.go    # Emitter, Transmitter
├── delegate.go   # Delegate, Interceptor, Registry
├── map.go        # Mapper, FlatMapper
├── result.go     # Result[T]
├── stream.go     # Stream, Transformer
└── terminal.go   # Slice, First, Run
```

## Detailed Comparisons

### vs. RxGo

| Aspect                   | RxGo                       | min-flow                           |
| ------------------------ | -------------------------- | ---------------------------------- |
| **Learning Curve**       | Steep (ReactiveX concepts) | Gentle (Go idioms)                 |
| **Hot/Cold Observables** | ✅                         | Streams are cold by default        |
| **Backpressure**         | ✅ Explicit                | ✅ Channel-based (natural)         |
| **Error Model**          | OnError callback           | Result[T] in stream                |
| **Parallel**             | Pool-based                 | Worker-based with ordering options |
| **Interceptors**         | ❌                         | ✅ First-class                     |

**When to choose RxGo**: You're porting ReactiveX code from another language.

**When to choose min-flow**: You want Go-native patterns with integrated resilience.

### vs. go-linq

| Aspect              | go-linq      | min-flow                  |
| ------------------- | ------------ | ------------------------- |
| **Focus**           | Query syntax | Stream processing         |
| **Concurrency**     | ❌           | ✅ Built-in               |
| **Resilience**      | ❌           | ✅ Retry, Circuit Breaker |
| **Lazy Evaluation** | ✅           | ✅                        |
| **iter.Seq**        | ✅           | ✅                        |

**When to choose go-linq**: Simple data queries, no concurrency needed.

**When to choose min-flow**: Concurrent processing, resilience, observability.

### vs. samber/lo

| Aspect       | samber/lo                  | min-flow                   |
| ------------ | -------------------------- | -------------------------- |
| **Type**     | Utility library            | Stream framework           |
| **Lazy**     | ❌ Eager                   | ✅ Lazy                    |
| **Memory**   | Full collections in memory | Streaming                  |
| **Retry**    | ✅ Attempt()               | ✅ Retry, RetryWithBackoff |
| **Parallel** | ✅                         | ✅                         |

**When to choose samber/lo**: Small collections, utility functions.

**When to choose min-flow**: Large/infinite streams, complex pipelines.

### vs. avast/retry-go + sony/gobreaker

| Aspect            | retry-go + gobreaker | min-flow                 |
| ----------------- | -------------------- | ------------------------ |
| **Integration**   | Manual composition   | Native                   |
| **Scope**         | Function wrapping    | Stream processing        |
| **Configuration** | Separate configs     | Unified delegate configs |
| **Observability** | Callbacks            | Interceptor events       |

**When to choose separate libraries**: Simple function retry, not streams.

**When to choose min-flow**: Stream-oriented processing with resilience.

## Performance Considerations

Min-flow prioritizes **correctness and developer experience** over raw performance, but remains competitive:

- **Channel overhead**: ~10-15ns per item (Go runtime optimized)
- **Result wrapping**: ~2-3 allocations per item
- **Interceptor dispatch**: ~50ns per event with 3 interceptors
- **Fusion optimization**: Up to 3x speedup for chained mappers

For CPU-bound batch processing where every nanosecond matters, consider:

- Using `Fuse()` for chained transformations
- Increasing batch sizes in `aggregate.Batch()`
- Using `parallel.Parallel()` for I/O-bound work

## Migration Guide

### From RxGo

```go
// RxGo
rxgo.Just(1, 2, 3)().
    Map(func(_ context.Context, i interface{}) (interface{}, error) {
        return i.(int) * 2, nil
    }).
    Filter(func(i interface{}) bool {
        return i.(int) > 2
    })

// min-flow
flow.FromSlice([]int{1, 2, 3}).
    Transform(core.Map(func(n int) (int, error) { return n * 2, nil })).
    Transform(filter.Where(func(n int) bool { return n > 2 }))
```

### From go-linq

```go
// go-linq
linq.From(data).
    Where(func(i interface{}) bool { return i.(int)%2 == 0 }).
    Select(func(i interface{}) interface{} { return i.(int) * 2 })

// min-flow
flow.FromSlice(data).
    Transform(filter.Where(func(n int) bool { return n%2 == 0 })).
    Transform(core.Map(func(n int) (int, error) { return n * 2, nil }))
```

### From retry-go + gobreaker

```go
// Before: Manual composition
cb := gobreaker.NewCircuitBreaker[Result](settings)
result, err := retry.Do(func() (Result, error) {
    return cb.Execute(func() (Result, error) {
        return doWork()
    })
}, retry.Attempts(3))

// After: Integrated in stream
stream := flow.Just(input)
result := flow.Pipe(
    flowerrors.CircuitBreaker[Input](cbConfig),
    flowerrors.Retry[Input](3),
    core.Map(doWork),
).Apply(ctx, stream)
```

## Conclusion

Choose min-flow when you need:

- **Unified solution**: Stream processing + resilience + observability
- **Go-native patterns**: Channels, context, iter.Seq
- **Cross-cutting concerns**: Interceptor pattern for metrics/logging/errors
- **Type safety**: Full generics with compile-time checking
- **Minimal dependencies**: Core has zero external dependencies

Choose alternatives when:

- **RxGo**: Porting ReactiveX code, need hot observables
- **go-linq**: Simple LINQ-style queries, no concurrency
- **samber/lo**: Utility functions on small collections
- **Dedicated libraries**: Single-purpose retry or circuit breaker only
