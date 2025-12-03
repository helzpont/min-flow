# Any-Based Result POC: Results

**Date**: December 3, 2025  
**System**: Apple M4 Pro (arm64), Go 1.23+

## Hypothesis

Storing values as `any` instead of typed generics might reduce allocations by allowing zero-copy type assertions when retrieving values.

## Results Summary

**Verdict: The hypothesis is INCORRECT. Any-based storage is significantly WORSE.**

The `any` interface type causes boxing overhead for value types, resulting in:

- More allocations (interface boxing)
- Higher memory usage
- Slower performance

## Benchmark Results

### Result Creation

| Type          | Result[T]         | AnyResult           | Difference          |
| ------------- | ----------------- | ------------------- | ------------------- |
| `int`         | 0.23 ns, 0 allocs | 5.24 ns, 0 allocs\* | **+22x time**       |
| `string`      | 0.22 ns, 0 allocs | 0.22 ns, 0 allocs   | Same                |
| `LargeStruct` | 3.77 ns, 0 allocs | 15.79 ns, 1 alloc   | **+4x time, +80 B** |

\*Note: The int benchmark shows 0 allocs but the interface boxing happens at the call site

### Channel Throughput (10K items)

| Type               | Time     | Memory   | Allocs      |
| ------------------ | -------- | -------- | ----------- |
| `chan Result[int]` | 267 µs   | 2.4 KB   | 3           |
| `chan AnyResult`   | 363 µs   | 80.8 KB  | 9,747       |
| **Difference**     | **+36%** | **+33x** | **+3,249x** |

### Transformation Pipeline (10K items, 2 transforms + filter)

| Type              | Time     | Memory    | Allocs       |
| ----------------- | -------- | --------- | ------------ |
| `Result[int]`     | 12.5 ms  | 164 KB    | 1            |
| `AnyResult`       | 178.8 ms | 441 KB    | 29,489       |
| `direct int`      | 5.1 ms   | 0 B       | 0            |
| **Any vs Result** | **+14x** | **+2.7x** | **+29,488x** |

### Slice Collection (10K items)

| Type           | Time    | Memory   | Allocs      |
| -------------- | ------- | -------- | ----------- |
| `Result[int]`  | 20.7 ms | 328 KB   | 1           |
| `AnyResult`    | 81.4 ms | 479 KB   | 9,745       |
| **Difference** | **+4x** | **+46%** | **+9,744x** |

## Why AnyResult is Worse

1. **Interface Boxing**: When you store a value type (like `int`) in an `any` interface, Go must allocate memory to store the type descriptor and value. This is called "boxing."

2. **Per-Item Allocation**: For value types, every `AnyOk(value)` causes an allocation, whereas `Ok[int](value)` stores the int directly in the struct (zero allocation).

3. **Strings are Special**: Strings showed similar performance because a string is already a 2-word value (pointer + length) that fits the interface representation.

4. **Generics Win**: Go's generics compile to specialized code for each type, avoiding interface overhead entirely.

## Can We Leverage the String/Pointer Observation?

Additional benchmarks revealed that **pointer types don't pay boxing overhead**:

| Type             | Result[T]         | AnyResult         | Difference |
| ---------------- | ----------------- | ----------------- | ---------- |
| `int`            | 0.23 ns, 0 allocs | 5.20 ns, 0 allocs | **+22x**   |
| `string`         | 0.22 ns, 0 allocs | 0.23 ns, 0 allocs | Same       |
| `*int`           | 0.22 ns, 0 allocs | 0.23 ns, 0 allocs | Same       |
| `[]int`          | 0.22 ns, 0 allocs | 10.6 ns, 1 alloc  | **+47x**   |
| `LargeStruct`    | 3.81 ns, 0 allocs | 16.1 ns, 1 alloc  | **+4x**    |
| `*LargeStruct`   | 0.22 ns, 0 allocs | 0.23 ns, 0 allocs | Same       |

**Types that avoid boxing:**
- Pointers (`*T`) - already fit in interface slot
- Strings - header (ptr+len) representation matches interface layout
- Other interface types

**But this doesn't help because:**

| Approach | Time | Memory | Allocs |
|----------|------|--------|--------|
| `Result[int]` direct | 0.23 ns | 0 B | 0 |
| `Result[*int]` allocate each | 4.78 ns | 8 B | 1 |
| `AnyResult` *int allocate each | 4.88 ns | 8 B | 1 |

Using pointers to avoid boxing just **moves the allocation** to pointer creation. You'd still pay 1 allocation per item - the same cost as boxing!

The only scenarios where this could help:
1. **Data already exists as pointers** (rare in stream processing)
2. **Pointer pooling** (complex, error-prone, marginal benefit)
3. **Users explicitly choose pointer types** (bad API ergonomics)

## Conclusion

The current `Result[T]` implementation using Go generics is the correct approach. The any-based storage is not a viable optimization path for min-flow.

### Files Created

- `flow/core/result_any.go` - POC implementation
- `flow/core/result_any_test.go` - Benchmark tests

These files can be deleted as the POC has concluded.

## Next Steps

Based on these results, we should investigate other optimization opportunities:

1. **Capacity-based context checking** - Batch context.Done() checks
2. **Channel buffer tuning** - Optimize DefaultBufferSize
3. **Fusion improvements** - Reduce intermediate allocations in fused mappers
4. **Pool allocations** - Use sync.Pool for frequently created objects
