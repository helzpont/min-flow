# Capacity-Based Context Checking POC: Results

**Date**: December 3, 2025  
**System**: Apple M4 Pro (arm64), Go 1.23+

## Hypothesis

Instead of checking `ctx.Done()` on every item send, we can batch context checks based on channel buffer capacity. When the buffer has space, we skip the check; when full, we check.

## Results Summary

**Verdict: POSITIVE - Capacity-based checking provides 25-40% speedup.**

The optimization is effective because:
1. `select { case <-ctx.Done(): default: }` costs ~1.4ns per call
2. For 10K items, that's 14µs saved per stage just on the check
3. The real savings come from reduced scheduler/channel coordination

## Benchmark Results

### Single Stage (10K items)

| Strategy | Time | vs Every-Item |
|----------|------|---------------|
| Every-item check | 2.03 ms | baseline |
| Capacity-based | 1.53 ms | **-25%** |
| No check (baseline) | 266 µs | -87% |

### 3-Stage Pipeline (10K items)

| Strategy | Time | vs Every-Item |
|----------|------|---------------|
| Every-item check | 3.25 ms | baseline |
| Capacity-based | 2.16 ms | **-34%** |

### Impact of Buffer Size

| Buffer Size | Every-Item | Capacity-Based | Improvement |
|-------------|------------|----------------|-------------|
| 1 | 3.08 ms | 2.75 ms | -11% |
| 8 | 2.25 ms | 1.73 ms | -23% |
| 32 | 1.93 ms | 1.22 ms | -37% |
| 64 | 1.84 ms | 1.23 ms | -33% |
| 128 | 1.68 ms | 1.09 ms | -35% |
| 256 | 1.54 ms | 929 µs | **-40%** |

**Key insight**: Larger buffers amplify the benefit because we skip more checks.

### Raw Context Check Cost

| Operation | Time |
|-----------|------|
| `select { case <-ctx.Done(): default: }` | 1.45 ns |
| No-op | 0.23 ns |

The check itself is ~6x more expensive than a no-op, but the real cost is in the scheduler overhead when combined with channel operations.

## Trade-offs

### Pros
- 25-40% speedup depending on buffer size
- No additional allocations
- Works with existing buffer infrastructure

### Cons
- **Delayed cancellation response**: Up to `bufferSize` items may be processed after cancel
- **More complex code path**: Extra logic in hot path
- **Less predictable cancel timing**: Hard to reason about exact cancel point

## Cancellation Latency

With capacity-based checking, after `ctx.Cancel()`:
- Worst case: `bufferSize` more items processed
- Observed in tests: ~28 items after cancel with buffer size 64

For most use cases, this is acceptable. For strict cancellation requirements, per-item checking is still appropriate.

## Implementation Notes

The POC uses a two-part strategy:
1. **Periodic check**: After processing `bufferSize` items, do a context check
2. **Blocking check**: When channel is full, check before blocking

```go
// Try non-blocking send first
select {
case out <- value:
    itemsSinceCheck++
default:
    // Channel full - must block, check context first
    select {
    case <-ctx.Done():
        return
    case out <- value:
        itemsSinceCheck = 0
    }
}
```

## Recommendation

**Consider adopting capacity-based checking as the default** for:
- High-throughput pipelines
- CPU-bound transformations
- Scenarios where 64-item cancellation latency is acceptable

**Keep per-item checking for**:
- Low-latency cancellation requirements
- Small buffer sizes (< 8)
- Interactive/real-time processing

## Files Created

- `flow/core/capacity_check.go` - POC implementation
- `flow/core/capacity_check_test.go` - Benchmarks and tests

## Next Steps

If adopting:
1. Add configuration option for check strategy
2. Consider making capacity-based the default for `Map`, `FlatMap`
3. Document cancellation latency trade-off in API docs
4. Consider hybrid approach: capacity-based + periodic time check
