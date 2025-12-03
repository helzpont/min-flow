# sync.Pool for Slice Allocations POC

**Status**: NEGATIVE (not recommended)
**Date**: Session ongoing
**Goal**: Evaluate whether sync.Pool can reduce allocation overhead in FlatMapper

## Summary

**sync.Pool does NOT provide performance benefits for FlatMapper slice allocations.** In fact, it makes performance worse in realistic scenarios. The pooling overhead exceeds any allocation savings.

## Investigation

### Hypothesis

FlatMapper allocates a `[]Result[OUT]` slice for every input item:
```go
results := make([]Result[OUT], len(mappedValues))
```

sync.Pool could potentially reduce GC pressure by reusing these slices.

### Benchmark Results

#### Micro-benchmark: Slice Allocation Only
```
BenchmarkSliceAllocation/make_each_time-14      70422018    14.31 ns/op    80 B/op    1 allocs/op
BenchmarkSliceAllocation/sync_pool-14           51535506    22.81 ns/op     0 B/op    0 allocs/op
BenchmarkSliceAllocation/reuse_single_slice-14 445413631     2.69 ns/op     0 B/op    0 allocs/op
```

For simple int slices, sync.Pool is actually **slower** than make() due to pool overhead.

#### Micro-benchmark: Result[int] Slices
```
BenchmarkResultSliceAllocation/make_Result_slice-14    31627893    37.37 ns/op   320 B/op   1 allocs/op
BenchmarkResultSliceAllocation/pool_Result_slice-14    69997885    16.94 ns/op     0 B/op   0 allocs/op
```

For larger Result slices, sync.Pool is 2.2x faster in isolation.

#### Realistic FlatMap with Channels
```
BenchmarkRealisticFlatMap/current_approach-14       8888   137.1 µs   2465 B/op   3 allocs/op
BenchmarkRealisticFlatMap/pooled_approach-14        8234   153.4 µs   2467 B/op   3 allocs/op
BenchmarkRealisticFlatMap/no_slice_direct_send-14   8967   130.5 µs   2464 B/op   3 allocs/op
BenchmarkRealisticFlatMap/iterator_approach-14      9088   132.1 µs   2464 B/op   3 allocs/op
```

In a realistic scenario with channels:
- **Pooled: 12% SLOWER than current**
- Direct send (no slice): 5% faster
- Iterator (iter.Seq): 4% faster

### Why sync.Pool Doesn't Help

1. **Short-lived allocations**: The slices are allocated, used immediately, and discarded. Go's allocator handles this efficiently.

2. **Pool overhead**: Get/Put operations add latency that exceeds allocation savings.

3. **No cross-goroutine sharing**: FlatMapper runs in a single goroutine. sync.Pool shines with concurrent access where it reduces contention on the allocator.

4. **GC efficiency**: Modern Go's GC is highly optimized for small, short-lived allocations that don't escape function scope.

### Alternative Optimizations Found

The benchmarks revealed that **avoiding the intermediate slice** is the real opportunity:

| Approach | Time | vs Current |
|----------|------|------------|
| Current (slice) | 137.1 µs | baseline |
| No slice (direct send) | 130.5 µs | **+5% faster** |
| Iterator (iter.Seq) | 132.1 µs | **+4% faster** |

This suggests a potential API enhancement:

```go
// Current FlatMapper returns a slice
type FlatMapper[IN, OUT any] func(Result[IN]) ([]Result[OUT], error)

// Alternative: Iterator-based FlatMapper (future consideration)
type IterFlatMapper[IN, OUT any] func(Result[IN]) iter.Seq[Result[OUT]]
```

## Recommendations

1. **Do NOT use sync.Pool for FlatMapper slices** - it makes things worse.

2. **Consider iterator-based FlatMapper** as a future enhancement for the fast package - potential 4-5% improvement.

3. **Focus optimization efforts elsewhere** - the current slice allocation approach is efficient enough.

## Conclusion

sync.Pool is designed for scenarios with:
- Long-lived allocations
- Concurrent access patterns
- Large allocation sizes

FlatMapper has none of these characteristics. The investigation confirms that Go's allocator is highly efficient for our use case, and adding pooling would be a net negative for performance.
