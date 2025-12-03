# Min-Flow Benchmark Results

This document captures baseline benchmark results for min-flow's internal operations.

**System**: Apple M4 Pro (arm64), macOS, Go 1.23+

**Date**: December 2, 2025

---

## Summary

| Operation                  | Time     | Allocs | Notes                       |
| -------------------------- | -------- | ------ | --------------------------- |
| Stream creation (1K items) | 65μs     | 19     | FromSlice                   |
| Result creation (Ok)       | ~5ns     | 1      | Zero-cost wrapper           |
| Event matching             | 1-4ns    | 0      | Pattern matching            |
| Registry lookup            | 2.4ns    | 0      | Context value extraction    |
| Fusion speedup             | 1.5-2.1x | -      | Reduces goroutines/channels |

---

## Stream Creation & Collection

FromSlice is the most common stream source. Performance scales linearly with size.

| Source        | Size   | Time    | Allocs |
| ------------- | ------ | ------- | ------ |
| `FromSlice`   | 100    | 2.2μs   | 13     |
| `FromSlice`   | 1,000  | 65.6μs  | 19     |
| `FromSlice`   | 10,000 | 652.6μs | 26     |
| `FromChannel` | 100    | 24.4μs  | 16     |
| `FromChannel` | 1,000  | 236.7μs | 20     |
| `FromChannel` | 10,000 | 2.37ms  | 27     |
| `Range`       | 100    | 18.5μs  | 14     |
| `Range`       | 1,000  | 180.6μs | 18     |
| `Range`       | 10,000 | 1.8ms   | 25     |
| `Once`        | 1      | 543ns   | 7      |
| `Empty`       | 0      | 85ns    | 4      |

**Observations**:

- `FromSlice` is significantly faster than `FromChannel` or `Range` for the same data size
- `FromChannel` has goroutine coordination overhead
- `Range` generates values on-the-fly (no pre-allocation)

---

## Result Type Operations

The `Result[T]` type is the core wrapper for stream items. Near zero-cost.

| Operation              | Time   | Allocs |
| ---------------------- | ------ | ------ |
| `Ok(42)` + `Value()`   | ~5ns   | 1      |
| `Err(err)` + `Error()` | ~5ns   | 1      |
| `IsValue()`            | ~0.5ns | 0      |
| `IsError()`            | ~0.5ns | 0      |

**Observations**:

- Result type is effectively free for value checking
- Single allocation per Result creation

---

## Transformer Chain Performance

Each transformer in an unfused chain creates a goroutine and channel.

| Chain Length   | Time (1K items) | Allocs |
| -------------- | --------------- | ------ |
| 1 transformer  | ~177μs          | 25     |
| 2 transformers | ~300μs          | 31     |
| 3 transformers | ~420μs          | 37     |
| 5 transformers | ~650μs          | 49     |

**Per-transformer overhead**: ~110μs and 6 allocations

---

## Transformer Fusion

Fusion combines multiple mappers into a single stage, eliminating intermediate channels.

| Configuration | Time (10K items) | Allocs | Speedup   |
| ------------- | ---------------- | ------ | --------- |
| 3 Unfused     | 3.69ms           | 41     | 1.0x      |
| 3 Fused       | 2.36ms           | 31     | **1.56x** |
| 5 Unfused     | 5.92ms           | 52     | 1.0x      |
| 5 Fused       | 2.77ms           | 31     | **2.14x** |

**Key insight**: Fusion provides significant speedup by:

- Reducing goroutine creation
- Eliminating channel coordination
- Reducing allocations

---

## Interceptor Overhead

Interceptors enable cross-cutting concerns but add overhead per item.

### Stream Processing Overhead (1K items)

| Configuration           | Time   | Allocs | Overhead |
| ----------------------- | ------ | ------ | -------- |
| Baseline (no intercept) | 177μs  | 25     | 1.0x     |
| Intercept (no registry) | 706μs  | 5,773  | 4.0x     |
| Empty registry          | 735μs  | 5,774  | 4.2x     |
| 1 Counter               | 987μs  | 11,785 | 5.6x     |
| 3 Interceptors          | 1.31ms | 23,801 | 7.4x     |
| Metrics (mutex)         | 1.05ms | 11,785 | 5.9x     |
| 5 Interceptors          | 1.70ms | 32,813 | 9.6x     |

### Per-Item Cost (10K items)

| Configuration      | Time   | Per-Item | Notes                        |
| ------------------ | ------ | -------- | ---------------------------- |
| Baseline           | 650μs  | ~65ns    |                              |
| With 1 interceptor | 5.9ms  | ~590ns   | Default (buffered, size=64)  |
| Unbuffered         | 7.8ms  | ~780ns   | Strict backpressure mode     |

**Observations**:

- Interceptor dispatch adds ~590ns per item with 1 interceptor (default buffer=64)
- Cost scales approximately linearly with interceptor count
- 78% of overhead is goroutine scheduling, not interceptor logic
- Default buffer size of 64 provides 41% speedup over unbuffered
- Consider batching or sampling for ultra-high-throughput scenarios

### Event Matching Performance

Pattern matching is extremely fast (nanosecond-level).

| Pattern Type    | Time   | Notes             |
| --------------- | ------ | ----------------- |
| Exact match     | 3.5ns  | `"item:received"` |
| Wildcard suffix | 4.3ns  | `"stream:*"`      |
| Wildcard prefix | 2.8ns  | `"*:start"`       |
| Match all       | 0.98ns | `"*"`             |

---

## Registry Operations

| Operation               | Time  | Allocs |
| ----------------------- | ----- | ------ |
| `WithRegistry()`        | 40ns  | 3      |
| `Register(interceptor)` | 141ns | 7      |
| `Interceptors()` (1)    | 20ns  | 1      |
| `Interceptors()` (5)    | 103ns | 4      |
| `GetRegistry()`         | 2.4ns | 0      |

**Observations**:

- Registry creation is fast (40ns)
- Context lookup is extremely fast (2.4ns)
- Interceptor list retrieval allocates a new slice

---

## Parallel Processing Scalability

Testing with 1,000 items, each with 1μs simulated work.

### min-flow Parallel Map

| Workers | Time   | Speedup vs 1 Worker |
| ------- | ------ | ------------------- |
| 1       | 4.43ms | 1.0x                |
| 2       | 2.65ms | 1.67x               |
| 4       | 1.48ms | 2.99x               |
| 8       | 1.67ms | 2.65x               |
| 16      | 1.79ms | 2.47x               |

### Comparison with Rill

| Workers | min-flow | Rill   | Winner             |
| ------- | -------- | ------ | ------------------ |
| 1       | 4.43ms   | 4.41ms | ~tie               |
| 2       | 2.65ms   | 2.55ms | Rill (4% faster)   |
| 4       | 1.48ms   | 1.33ms | Rill (10% faster)  |
| 8       | 1.67ms   | 0.63ms | Rill (2.6x faster) |
| 16      | 1.79ms   | 0.69ms | Rill (2.6x faster) |

**Observations**:

- min-flow scales well up to 4 workers on this workload
- Beyond 4 workers, diminishing returns (likely M4 Pro's efficiency cores)
- Rill has better high-concurrency scaling (different worker pool implementation)
- For typical I/O-bound work, both are adequate

---

## Recommendations

### When to Use Fusion

- Chain of 3+ mappers that don't need intermediate values
- High-throughput scenarios where every μs matters
- Memory-constrained environments

### When to Use Interceptors

- Development/debugging (always acceptable)
- Production with moderate throughput (<10K items/sec)
- Sampling: intercept every Nth item
- Batch metrics: collect counts, flush periodically

### Parallel Processing

- Use 2-4 workers for balanced performance
- For I/O-bound work, more workers may help
- Consider ordered vs unordered based on requirements

---

## Running Benchmarks

```bash
# All benchmarks
cd benchmarks && go test -bench=. -benchmem -run=^$

# Specific categories
go test -bench=BenchmarkStream -benchmem -run=^$
go test -bench=BenchmarkInterceptor -benchmem -run=^$
go test -bench=BenchmarkFusion -benchmem -run=^$
go test -bench=BenchmarkParallel -benchmem -run=^$

# With CPU profiling
go test -bench=BenchmarkPipeline -benchmem -run=^$ -cpuprofile=cpu.prof
go tool pprof cpu.prof
```
