# Stream Observation Guide

This guide explains when and how to observe stream processing in min-flow. Choose the right approach based on your use case.

## Quick Reference

| Use Case | Approach | Package |
|----------|----------|---------|
| Count values/errors for a specific type | `observe.WithCounter[T]()` | observe |
| Log all stream events | `observe.WithLogging[T]()` | observe |
| Collect errors for analysis | `observe.WithErrorCollector[T]()` or `flowerrors.WithErrorCollector[T]()` | observe/flowerrors |
| Custom per-value callback | `observe.WithValueHook[T]()` | observe |
| Monitor error rates | `flowerrors.WithCircuitBreakerMonitor[T]()` | flowerrors |
| Full custom observation | `core.WithHooks[T]()` | core |

## Observation Methods

### 1. Pre-built Observers (Recommended for Common Cases)

The `observe` and `flowerrors` packages provide ready-to-use observers:

```go
import "github.com/lguimbarda/min-flow/flow/observe"

// Count values and errors
ctx, counter := observe.WithCounter[int](ctx)
// ... process stream ...
fmt.Printf("Values: %d, Errors: %d\n", counter.Values(), counter.Errors())

// Collect errors for later inspection
ctx, collector := observe.WithErrorCollector[int](ctx)
// ... process stream ...
for _, err := range collector.Errors() {
    log.Error(err)
}

// Log all events
ctx = observe.WithLogging[int](ctx, func(format string, args ...any) {
    log.Printf(format, args...)
})
```

**When to use**: Standard metrics, logging, error collection where you don't need custom logic.

### 2. Individual Hook Functions (For Specific Events)

When you only care about specific events:

```go
import "github.com/lguimbarda/min-flow/flow/observe"

// Only track values
ctx = observe.WithValueHook(ctx, func(v int) {
    metrics.RecordValue(v)
})

// Only track errors
ctx = observe.WithErrorHook[int](ctx, func(err error) {
    alerting.SendError(err)
})

// Track lifecycle
ctx = observe.WithStartHook[int](ctx, func() {
    timer = time.Now()
})
ctx = observe.WithCompleteHook[int](ctx, func() {
    metrics.RecordDuration(time.Since(timer))
})
```

**When to use**: You need callbacks for specific events without the overhead of unused hooks.

### 3. Full Hooks Structure (For Complex Observation)

When you need coordinated observation across multiple events:

```go
import "github.com/lguimbarda/min-flow/flow/core"

ctx = core.WithHooks(ctx, core.Hooks[int]{
    OnStart: func() {
        batch.Begin()
    },
    OnValue: func(v int) {
        batch.Add(v)
    },
    OnError: func(err error) {
        batch.RecordError(err)
    },
    OnComplete: func() {
        batch.Commit()
    },
})
```

**When to use**: You need state shared across hooks, or all hooks work together as a unit.

### 4. SafeHooks (For Unreliable Callbacks)

When hook callbacks might panic:

```go
import "github.com/lguimbarda/min-flow/flow/core"

safeHooks := core.SafeHooks(core.Hooks[int]{
    OnValue: func(v int) {
        riskyOperation(v) // Might panic
    },
}, func(event string, r any) {
    log.Printf("Hook panic in %s: %v", event, r)
})

ctx = core.WithHooks(ctx, safeHooks)
```

**When to use**: Hooks call external code that might panic, and you want to recover gracefully.

## Composing Multiple Observers

Multiple hooks compose in FIFO (first-in, first-out) order:

```go
// First observer
ctx = observe.WithValueHook(ctx, func(v int) {
    fmt.Println("First:", v)
})

// Second observer (fires after first)
ctx = observe.WithValueHook(ctx, func(v int) {
    fmt.Println("Second:", v)
})

// Output for value 42:
// First: 42
// Second: 42
```

## Type-Specific Registration

Hooks are type-parameterized. `Hooks[int]` only fires for `Stream[int]`:

```go
// This hook fires for int streams
ctx = observe.WithValueHook(ctx, func(v int) {
    fmt.Println("Int:", v)
})

// This hook fires for string streams (different type!)
ctx = observe.WithValueHook(ctx, func(v string) {
    fmt.Println("String:", v)
})

intStream := flow.FromSlice([]int{1, 2, 3})
stringStream := flow.FromSlice([]string{"a", "b", "c"})

// Only the int hook fires
mapper := core.Map(func(x int) (int, error) { return x * 2, nil })
mapper.Apply(ctx, intStream)
```

## Performance Considerations

1. **Zero overhead when unused**: If no hooks are registered for a type, there's no overhead
2. **No allocations**: Hook invocation doesn't allocate (unlike the old interceptor system)
3. **Keep hooks fast**: Hooks run synchronously; slow hooks block the pipeline
4. **Use goroutines for slow work**: If you need async processing, spawn a goroutine in your hook

```go
// Good: Fast hook with async processing
results := make(chan int, 100)
ctx = observe.WithValueHook(ctx, func(v int) {
    select {
    case results <- v:
    default:
        // Drop if buffer full
    }
})

// Process asynchronously
go func() {
    for v := range results {
        slowOperation(v)
    }
}()
```

## Debugging Tips

1. **Use `WithLogging` during development** to see all stream events
2. **Check hook type parameters** - `Hooks[int]` won't fire for `Stream[string]`
3. **Hooks fire at transformer boundaries** - they're invoked by `Mapper`, `FlatMapper`, etc.
4. **Multiple transformers = multiple hook invocations** - each transformer stage invokes hooks

## Error Observation Patterns

### Count and Alert on Threshold

```go
import "github.com/lguimbarda/min-flow/flow/flowerrors"

ctx, cbMonitor := flowerrors.WithCircuitBreakerMonitor[int](ctx, 5, func() {
    alerting.Send("Error threshold exceeded!")
})
```

### Collect Specific Errors

```go
ctx, collector := flowerrors.WithErrorCollector[int](ctx,
    flowerrors.WithPredicate(func(err error) bool {
        return errors.Is(err, context.DeadlineExceeded)
    }),
    flowerrors.WithMaxErrors(100),
)
```

### Log Errors with Context

```go
ctx = flowerrors.OnErrorDo[int](ctx, func(err error) {
    log.WithFields(log.Fields{
        "stream": "user-processing",
        "error":  err.Error(),
    }).Error("Stream error")
})
```
