# flow/fast

⚠️ **USE AT YOUR OWN RISK** ⚠️

High-performance stream processing that sacrifices min-flow's safety features for raw speed.

## What's Different?

| Feature         | `flow/core`              | `flow/fast`            |
| --------------- | ------------------------ | ---------------------- |
| Result wrapping | ✅ `Result[T]`           | ❌ Direct values       |
| Panic recovery  | ✅ Wrapped in `ErrPanic` | ❌ Panics propagate    |
| Error handling  | ✅ `Err[T]` results      | ❌ None                |
| Context checks  | ✅ Per-item              | ⚠️ At channel ops only |
| Interceptors    | ✅ Full delegate system  | ❌ None                |
| Sentinels       | ✅ `Sentinel[T]`         | ❌ None                |

## When to Use

✅ **Good candidates:**

- CPU-bound transformations where channel overhead dominates
- Trusted, well-tested transformation functions
- Benchmarking min-flow feature overhead
- Hot paths in production after extensive testing

❌ **Avoid when:**

- Processing untrusted or user-provided data
- Error recovery is needed
- Observability (metrics, logging) is required
- I/O-bound operations where safety matters more

## Usage

```go
import "github.com/lguimbarda/min-flow/flow/fast"

ctx := context.Background()

// Create stream
stream := fast.FromSlice([]int{1, 2, 3, 4, 5})

// Transform (no error signature!)
doubled := fast.Map(func(n int) int {
    return n * 2
}).Apply(ctx, stream)

// Filter
evens := fast.Filter(func(n int) bool {
    return n%2 == 0
}).Apply(ctx, doubled)

// Collect (no error return!)
result := fast.Slice(ctx, evens)
// result: [4, 8]
```

## API

### Sources

```go
fast.FromSlice([]T)       // From slice
fast.FromChannel(ch)      // From channel
fast.FromIter(seq)        // From Go 1.23+ iterator
fast.Range(start, end)    // Integer range
```

### Transformers

```go
fast.Map(fn)              // 1:1 transformation
fast.FlatMap(fn)          // 1:N transformation
fast.Filter(pred)         // Filtering
fast.Fuse(m1, m2)         // Combine mappers
```

### Aggregations

```go
fast.Reduce(fn)           // Reduce to single value
fast.Fold(initial, fn)    // Fold with initial value
fast.Take[T](n)           // First n items
fast.Skip[T](n)           // Skip first n
fast.Batch[T](size)       // Group into batches
```

### Terminal Operations

```go
fast.Slice(ctx, s)        // Collect all values
fast.First(ctx, s)        // First value
fast.Run(ctx, s)          // Consume for side effects
fast.Count(ctx, s)        // Count items
fast.ForEach(ctx, s, fn)  // Apply function to each
```

## Benchmarking Purpose

This package helps measure the overhead of min-flow's features:

```bash
go test -bench=. ./benchmarks/...
```

Compare results between:

- `BenchmarkPipeline_MinFlow_*` - Full feature set
- `BenchmarkPipeline_Fast_*` - Minimal overhead

The difference reveals the cost of:

1. **Result wrapping** - Allocating `Result[T]` vs direct values
2. **Panic recovery** - `defer/recover` in every mapper
3. **Error handling** - Checking `IsError()` on each item
4. **Context checks** - Per-item `ctx.Done()` select
5. **Interceptors** - Registry lookups and event dispatch

## Design Notes

- Matches `core.DefaultBufferSize` (64) for fair comparisons
- Same channel-per-stage architecture as core
- Minimal API surface - only essential operations
- No attempt to be a complete streaming library
