# ADR-007: Buffered Channels by Default

## Status

Accepted

## Context

Go channels can be buffered or unbuffered:

- **Unbuffered (size 0)**: synchronous, sender blocks until receiver reads
- **Buffered (size N)**: sender can write N items without blocking

In stream processing, the choice affects:

- **Throughput**: buffering reduces goroutine synchronization overhead
- **Latency**: buffering can increase latency for individual items
- **Backpressure**: unbuffered channels provide natural backpressure
- **Memory**: buffers consume memory proportional to item size × buffer size

We needed to choose default channel behavior for internal pipeline channels.

## Decision

**Internal channels use a default buffer size of 64.**

```go
const DefaultBufferSize = 64
```

This applies to:

- `Mapper.Apply()` output channels
- `FlatMapper.Apply()` output channels
- `Intercept()` passthrough channels
- Other internal transformer channels

Users can override with buffered variants:

```go
mapper.ApplyBuffered(ctx, stream, 128)  // custom buffer
core.InterceptBuffered[T](0)            // unbuffered for strict backpressure
```

## Consequences

### Positive

1. **Higher throughput** - reduces goroutine context switches:

   ```
   Benchmark results (example):
   Unbuffered: 150,000 items/sec
   Buffered(64): 450,000 items/sec (3x faster)
   ```

2. **Smoother processing** - buffers absorb temporary speed mismatches between stages

3. **Reasonable memory** - 64 items is small enough to not matter for most types:

   ```
   64 × 8 bytes (int64) = 512 bytes
   64 × 100 bytes (struct) = 6.4 KB
   ```

4. **Customizable** - users can tune per-use-case

### Negative

1. **Delayed backpressure** - slow consumers don't immediately slow producers

2. **Memory for large items** - 64 large structs or slices can consume significant memory

3. **Latency for single items** - items may sit in buffer before processing

### Why 64?

We chose 64 based on:

1. **CPU cache lines** - 64 bytes is a common cache line size; 64 items provides good prefetching
2. **Benchmarks** - diminishing returns above 64 for most workloads
3. **Memory balance** - small enough to be negligible, large enough to matter
4. **Powers of 2** - compiler/runtime may optimize

Comparison of buffer sizes (synthetic benchmark):

```
Buffer Size | Throughput  | Memory per channel
0           | 1.0x        | 0 bytes
16          | 2.2x        | minimal
64          | 3.1x        | ~512 bytes (for int)
256         | 3.3x        | ~2 KB
1024        | 3.4x        | ~8 KB
```

### Backpressure Considerations

For strict backpressure (e.g., rate limiting, resource protection):

```go
// Use unbuffered for strict backpressure
core.InterceptBuffered[T](0)

// Or small buffer for some smoothing
core.InterceptBuffered[T](4)
```

For high-throughput batch processing:

```go
// Larger buffer for bulk operations
mapper.ApplyBuffered(ctx, stream, 256)
```
