# parallel

The `parallel` package provides operators for concurrent stream processing using worker pools.

## Overview

Parallel processing enables high throughput for CPU-bound or I/O-bound operations by distributing work across multiple goroutines.

```mermaid
graph TB
    Source[Source Stream] --> Dispatcher[Dispatcher]

    Dispatcher --> W1[Worker 1]
    Dispatcher --> W2[Worker 2]
    Dispatcher --> W3[Worker 3]
    Dispatcher --> W4[Worker 4]

    W1 --> Collector[Collector]
    W2 --> Collector
    W3 --> Collector
    W4 --> Collector

    Collector --> Output[Output Stream]
```

## Operators

### Parallel Map

Process items concurrently with multiple workers:

```go
// Process with 4 workers (results may be out of order)
processed := parallel.Map(4, func(item Request) Response {
    return callAPI(item)
}).Apply(ctx, stream)

// With error handling
processed := parallel.MapErr(4, func(item Request) (Response, error) {
    return callAPIWithRetry(item)
}).Apply(ctx, stream)
```

### Ordered Parallel Map

Preserve input order while processing in parallel:

```go
// Process in parallel but emit in original order
ordered := parallel.MapOrdered(4, func(item Request) Response {
    return process(item)
}).Apply(ctx, stream)
```

### Parallel FlatMap

Expand items concurrently:

```go
// Each item produces multiple outputs, processed in parallel
expanded := parallel.FlatMap(4, func(userId string) []Order {
    return fetchOrders(userId)
}).Apply(ctx, stream)
```

### Parallel ForEach

Execute side effects concurrently:

```go
// Process items for side effects with worker pool
parallel.ForEach(4, func(item Event) {
    sendToAnalytics(item)
}).Apply(ctx, stream)
```

## Ordered vs Unordered

```mermaid
sequenceDiagram
    participant S as Source [A,B,C,D]
    participant U as Unordered
    participant O as Ordered

    Note over S,O: Items take different processing times
    S->>U: A (slow)
    S->>U: B (fast)
    S->>U: C (medium)
    S->>U: D (fast)

    U-->>U: B done
    U-->>U: D done
    U-->>U: C done
    U-->>U: A done

    Note over U: Output: B, D, C, A

    S->>O: A (slow)
    S->>O: B (fast)
    S->>O: C (medium)
    S->>O: D (fast)

    O-->>O: Wait for A...
    O-->>O: A done, emit A
    O-->>O: B already done, emit B
    O-->>O: C done, emit C
    O-->>O: D already done, emit D

    Note over O: Output: A, B, C, D
```

### When to Use Each

| Pattern           | Use Case                          | Tradeoff                          |
| ----------------- | --------------------------------- | --------------------------------- |
| `Map` (unordered) | Independent items, max throughput | Items may reorder                 |
| `MapOrdered`      | Order matters                     | Buffers until slow items complete |
| Single worker     | Order required, simple            | No parallelism                    |

## Configuration

Use `ParallelConfig` for default worker counts:

```go
ctx, registry := core.WithRegistry(ctx)
registry.Register(&parallel.ParallelConfig{
    Workers: 8,
})

// Now Map(0, ...) uses 8 workers
processed := parallel.Map(0, process).Apply(ctx, stream)

// Explicit worker count overrides config
processed := parallel.Map(4, process).Apply(ctx, stream)
```

## Backpressure

Parallel operators respect backpressure:

```mermaid
graph LR
    subgraph "Fast Producer"
        P[1000 items/sec]
    end

    subgraph "Parallel Workers (4)"
        W1[Worker 1]
        W2[Worker 2]
        W3[Worker 3]
        W4[Worker 4]
    end

    subgraph "Slow Consumer"
        C[100 items/sec]
    end

    P --> |backpressure| W1
    P --> |backpressure| W2
    P --> |backpressure| W3
    P --> |backpressure| W4

    W1 --> |backpressure| C
    W2 --> |backpressure| C
    W3 --> |backpressure| C
    W4 --> |backpressure| C
```

When the consumer is slow, workers block on output, which blocks the dispatcher, which slows consumption from the source.

## Concurrency Patterns

### CPU-Bound Work

```go
// Match worker count to CPU cores
workers := runtime.NumCPU()
result := parallel.Map(workers, cpuIntensiveTask).Apply(ctx, stream)
```

### I/O-Bound Work

```go
// More workers than cores for I/O-bound tasks
workers := 50 // or based on connection pool size
result := parallel.Map(workers, fetchFromAPI).Apply(ctx, stream)
```

### Mixed Workloads

```go
// Pipeline with different parallelism at each stage
stream.
    Apply(ctx, parallel.Map(runtime.NumCPU(), cpuBoundParse)).
    Apply(ctx, parallel.Map(50, ioBoundEnrich)).
    Apply(ctx, parallel.Map(runtime.NumCPU(), cpuBoundTransform))
```

## Error Handling

Errors from any worker are emitted to the output stream:

```go
processed := parallel.MapErr(4, func(item Request) (Response, error) {
    resp, err := callAPI(item)
    if err != nil {
        return Response{}, err // Becomes error Result
    }
    return resp, nil
}).Apply(ctx, stream)

// Handle errors downstream
result := processed.Apply(ctx, flowerrors.OnError(logError))
```

## When to Use

| Scenario              | Recommendation                 |
| --------------------- | ------------------------------ |
| CPU-bound processing  | Workers = NumCPU               |
| I/O-bound (API calls) | Workers = connection pool size |
| Order required        | `MapOrdered`                   |
| Max throughput        | `Map` (unordered)              |
| Side effects          | `ForEach`                      |
| Item expansion        | `FlatMap`                      |
