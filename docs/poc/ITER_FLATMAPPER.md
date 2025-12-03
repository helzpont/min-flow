# Iterator-based FlatMapper POC

**Status**: NEGATIVE (iterator overhead exceeds benefits)
**Date**: December 2024
**Goal**: Evaluate whether iter.Seq-based FlatMapper reduces allocation overhead

## Summary

**Iterator-based FlatMapper is SLOWER than slice-based FlatMapper.** The closure allocation overhead of Go's `iter.Seq` pattern exceeds any savings from avoiding intermediate slice allocation. The iterator approach has **3.5x more allocations** in practice.

## Investigation

### Hypothesis

The slice-based FlatMapper allocates a `[]Result[OUT]` slice for every input item:
```go
results := make([]Result[OUT], len(mappedValues))
```

An iterator-based approach using Go 1.23's `iter.Seq` could avoid this allocation by yielding results directly.

### Implementation

Created `IterFlatMapper[IN, OUT]` with two constructors:

```go
// Iterator-based: user provides an iter.Seq
func IterFlatMap[IN, OUT any](flatMapFunc func(IN) iter.Seq[OUT]) IterFlatMapper[IN, OUT]

// Convenience: user provides a slice, we iterate it
func IterFlatMapSlice[IN, OUT any](flatMapFunc func(IN) ([]OUT, error)) IterFlatMapper[IN, OUT]
```

### Benchmark Results

Testing with 10,000 input items, 5 outputs per item:

```
BenchmarkFlatMapperVsIterFlatMapper/FlatMapper_slice-14       160   7.43ms   4.04MB   20,038 allocs
BenchmarkFlatMapperVsIterFlatMapper/IterFlatMapper_iter-14    142   8.50ms   4.28MB   70,038 allocs
BenchmarkFlatMapperVsIterFlatMapper/IterFlatMapSlice_iter-14  146   8.15ms   3.96MB   50,037 allocs
```

| Approach | Time | Memory | Allocs | vs FlatMapper |
|----------|------|--------|--------|---------------|
| FlatMapper (slice) | 7.43ms | 4.04MB | 20,038 | baseline |
| IterFlatMapper | 8.50ms | 4.28MB | 70,038 | **14% slower, 3.5x allocs** |
| IterFlatMapSlice | 8.15ms | 3.96MB | 50,037 | **10% slower, 2.5x allocs** |

### With Check Strategies

```
FlatMapper_CheckEveryItem-14        235   5.07ms   2.37MB   20,035 allocs
FlatMapper_CheckOnCapacity-14       354   3.38ms   2.37MB   20,035 allocs
IterFlatMapper_CheckEveryItem-14    172   6.99ms   3.49MB   70,035 allocs
IterFlatMapper_CheckOnCapacity-14   258   4.67ms   3.57MB   60,036 allocs
```

Even with the high-throughput strategy, IterFlatMapper is ~38% slower than FlatMapper.

## Why Iterators Are Slower

1. **Closure allocation per call**: Each invocation of the flatMapFunc creates a new closure (the `iter.Seq` function). With 10,000 items × 5 outputs, this creates 50,000+ extra allocations.

2. **No escape analysis benefit**: The closures escape to the heap because they're passed to range loops.

3. **Iterator protocol overhead**: The yield/return dance adds function call overhead vs simple slice iteration.

4. **Synthetic benchmark was misleading**: The earlier micro-benchmark without channels/Result types showed ~4% improvement, but real-world usage patterns negate this.

### Memory Profile Hotspots

```
7,709,016  23.38%  IterFlatMapper.runWithEveryItemCheck
5,439,571  16.50%  IterFlatMapper.ApplyWith.func1.1
2,752,554   8.35%  user flatMapFunc closure
2,370,327   7.19%  IterFlatMap wrapper closures
```

The iterator machinery allocates multiple closures per input item:
1. User's `flatMapFunc` returns a new `iter.Seq` (closure)
2. `IterFlatMap` wrapper creates its own closure
3. The range loop interaction creates additional allocations

## Recommendations

1. **Keep slice-based FlatMapper as primary** - it's faster and simpler
2. **Do not promote IterFlatMapper for performance** - it's slower
3. **IterFlatMapper could have niche uses**:
   - Lazy evaluation where short-circuit is common
   - Infinite/unbounded sequences
   - Composability with other iter.Seq-based APIs

## Code Retained

The `IterFlatMapper` implementation was kept in the codebase as it:
- Provides API completeness (some users may prefer iterator style)
- Enables lazy evaluation patterns
- Demonstrates Go 1.23 iter.Seq integration

However, documentation should NOT recommend it for performance-critical code.

## Conclusion

Go's `iter.Seq` pattern is elegant for composability but has non-trivial allocation overhead. For hot paths like stream transformation, the classic slice-based approach remains more efficient. The ~10-15% performance penalty and 2.5-3.5x allocation increase make iterators unsuitable as a performance optimization for FlatMapper.

This contradicts the initial micro-benchmark finding (+4% for iterators) - a good reminder that micro-benchmarks can be misleading when they don't capture real-world allocation patterns.
