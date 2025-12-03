# POC: Interceptor Overhead Optimization

**Date**: 2024-12-03  
**Status**: ✅ POSITIVE - Significant improvement achieved  
**Category**: Performance Optimization

## Objective

Investigate and optimize the per-item overhead of the interceptor system, particularly when no interceptors are registered.

## Problem Statement

The original `InterceptBuffered` implementation used a variadic `invoke()` function:

```go
invoke := func(event Event, args ...any) {
    if !hasRegistry {
        return
    }
    // ... invoke interceptors
}

// Called per-item:
invoke(ItemReceived, res)
invoke(ValueReceived, res.Value())
invoke(ItemEmitted, res)
```

**The Problem**: In Go, variadic parameters (`...any`) allocate a slice at the **call site**, not inside the function. This means every `invoke()` call allocates a slice, even when:

1. There's no registry (`hasRegistry` is false)
2. There are no interceptors registered
3. The interceptor doesn't match the event

This caused ~60,000 allocations for processing 10,000 items, even when no interceptors were registered.

## Benchmark Results

### Before Optimization (10,000 items)

| Scenario                        | Time   | Allocations | Overhead vs Baseline      |
| ------------------------------- | ------ | ----------- | ------------------------- |
| No intercept (baseline)         | 1.89ms | 31          | -                         |
| With intercept, no interceptors | 4.28ms | 59,914      | +126% time, +1933x allocs |
| With intercept, 1 counting      | 5.08ms | 59,921      | +169% time                |

### After Optimization (10,000 items)

| Scenario                        | Time   | Allocations | Overhead vs Baseline  |
| ------------------------------- | ------ | ----------- | --------------------- |
| No intercept (baseline)         | 1.89ms | 31          | -                     |
| With intercept, no interceptors | 2.77ms | 41          | +47% time, +10 allocs |
| With intercept, 1 counting      | 4.87ms | 49,921      | +158% time            |

### Improvement Summary

| Scenario                   | Time Improvement | Allocation Reduction           |
| -------------------------- | ---------------- | ------------------------------ |
| No interceptors registered | **35% faster**   | **99.93% fewer** (59,914 → 41) |
| With interceptors          | ~2% faster       | ~17% fewer                     |

## Solution

Three-part optimization:

### 1. Early Exit for No Registry

```go
if !hasRegistry {
    for res := range in {
        select {
        case <-ctx.Done():
            return
        case out <- res:
        }
    }
    return
}
```

When there's no registry in the context, bypass all interceptor logic entirely.

### 2. Early Exit for Empty Interceptors

```go
interceptors := registry.Interceptors()
if len(interceptors) == 0 {
    for res := range in {
        select {
        case <-ctx.Done():
            return
        case out <- res:
        }
    }
    return
}
```

When a registry exists but has no interceptors, use the same fast path.

### 3. Specialized Invoke Functions

Replace the variadic `invoke()` with specialized versions:

```go
// For events with no arguments (StreamStart, StreamEnd)
invokeNoArg := func(event Event) {
    for _, interceptor := range interceptors {
        for _, pattern := range interceptor.Events() {
            if event.Matches(string(pattern)) {
                _ = interceptor.Do(ctx, event)
                break
            }
        }
    }
}

// For events with one argument (most item events)
invokeOneArg := func(event Event, arg any) {
    for _, interceptor := range interceptors {
        for _, pattern := range interceptor.Events() {
            if event.Matches(string(pattern)) {
                _ = interceptor.Do(ctx, event, arg)
                break
            }
        }
    }
}
```

This eliminates the variadic slice allocation at call sites.

## Why Some Allocations Remain

When interceptors are actually present and being called, allocations still occur because:

1. **Boxing to `any`**: The `arg any` parameter in `invokeOneArg` requires boxing the value
2. **Interceptor.Do() variadic**: The interface method `Do(ctx, event, ...any)` still allocates

These are **unavoidable given the current interface design**. However, this is acceptable because:

- You're paying for functionality you're actually using
- The overhead is proportional to the number of interceptors × items processed

## Potential Future Optimizations

If the remaining allocations become problematic, consider:

1. **Typed Interceptor interfaces**: Separate interfaces for different event signatures

   ```go
   type ItemInterceptor interface {
       DoItem(ctx context.Context, event Event, item any) error
   }
   ```

2. **Sync.Pool for arguments**: Reuse argument slices (adds complexity)

3. **Code generation**: Generate specialized interceptor invokers per event type

## Conclusion

The optimization is **highly effective** for the common case (no interceptors or empty registry):

- **99.93% reduction** in allocations
- **35% faster** throughput

The remaining allocations when interceptors are present are inherent to the interface design and represent acceptable overhead for the functionality provided.

## Files Changed

- `flow/core/channel.go`: Refactored `InterceptBuffered` with specialized invoke functions and early exits
