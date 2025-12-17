# ADR-005: Typed Hooks for Stream Observation

## Status

Accepted

## Date

2024-12-16

## Context

Min-flow needed a way to observe stream processing for metrics, logging, error tracking, and debugging. The original design used an `Interceptor` interface with pattern-based event dispatch:

```go
// Old approach (removed)
type Interceptor interface {
    Delegate
    Events() []Event      // Pattern matching: "stream.*", "item.*"
    Do(ctx context.Context, event Event, args ...any) error
}
```

This approach had several problems:

1. **Type erasure**: All values passed as `any`, requiring runtime type assertions
2. **Runtime overhead**: Event pattern matching on every invocation
3. **Allocation pressure**: Variadic `args ...any` caused allocations
4. **Complex registration**: Required Registry, pattern strings, and explicit `core.Intercept()` stages
5. **Inconsistent behavior**: Generic observers couldn't distinguish between `Stream[int]` and `Stream[string]`

## Decision

Replace the interceptor dispatch system with a typed hooks system that:

1. Uses generics for type-safe callbacks: `Hooks[T]`
2. Attaches directly to context without requiring a Registry
3. Invokes only registered hooks (no pattern matching)
4. Provides zero-overhead when no hooks are registered

### New API

```go
// Typed hooks structure
type Hooks[T any] struct {
    OnStart    func()       // Stream begins processing
    OnValue    func(T)      // Successful value received
    OnError    func(error)  // Error received
    OnSentinel func(error)  // Sentinel received
    OnComplete func()       // Stream finished
}

// Attach hooks to context
ctx = core.WithHooks(ctx, core.Hooks[int]{
    OnValue: func(v int) { count++ },
    OnError: func(err error) { log.Error(err) },
})
```

### Automatic Invocation

Hooks are automatically invoked by transformers (`Mapper`, `FlatMapper`, `IterFlatMapper`) during stream processing. No explicit `Intercept()` stage is needed.

## Consequences

### Positive

- **Type safety**: `OnValue` receives `T`, not `any`
- **Zero allocation**: No variadic args, no interface boxing for registered types
- **Simpler API**: `WithHooks()` vs Registry + Register + Intercept
- **Predictable performance**: O(1) hook lookup, no pattern matching
- **Composable**: Multiple `WithHooks()` calls compose in FIFO order

### Negative

- **Type-specific registration**: Must register hooks for each stream type separately
- **No wildcard observers**: Cannot observe all streams regardless of type
- **Breaking change**: Existing interceptor-based code must be migrated

### Neutral

- The `Registry` and `Interceptor` interfaces remain for other delegate types (factories, pools, configs)
- `core.Intercept()` transmitter still exists but now uses hooks internally

## Migration Guide

### Before (Interceptor-based)

```go
// Old: Register interceptor with registry
ctx, registry := core.WithRegistry(ctx)
counter := observe.NewCounterInterceptor()
registry.Register(counter)

// Old: Explicit intercept stage required
stream := someMapper.Apply(ctx, input)
intercepted := core.Intercept[int]().Apply(ctx, stream)
result, _ := core.Slice(ctx, intercepted)

fmt.Println(counter.Values())
```

### After (Hooks-based)

```go
// New: Attach hooks directly to context
ctx, counter := observe.WithCounter[int](ctx)

// New: No intercept stage needed - hooks fire automatically
stream := someMapper.Apply(ctx, input)
result, _ := core.Slice(ctx, stream)

fmt.Println(counter.Values())
```

## References

- [hooks.go](../../flow/core/hooks.go) - Core hooks implementation
- [observe/register.go](../../flow/observe/register.go) - Convenience functions
- [Interceptor Redesign Proposal](../proposals/interceptor-redesign.md) - Original design document
