# ADR-006: Panic Recovery in User Functions

## Status

Accepted

## Context

Users provide functions to the framework:

- Map functions: `func(T) (U, error)`
- FlatMap functions: `func(T) ([]U, error)`
- Filter predicates: `func(T) bool`

These functions can panic due to:

- Nil pointer dereference
- Index out of bounds
- Type assertion failure
- Explicit `panic()` calls

If we don't recover from panics:

- The entire pipeline crashes
- Resources may leak (unclosed channels, goroutines)
- Users can't distinguish "framework bug" from "my function panicked"

We needed to decide how the framework handles panics in user-provided code.

## Decision

**All user-provided functions are wrapped with panic recovery.** Panics are converted to errors and flow through the Result system.

```go
func (m Mapper[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT] {
    return Emit(func(ctx context.Context) <-chan Result[OUT] {
        out := make(chan Result[OUT], DefaultBufferSize)
        go func() {
            defer close(out)
            for res := range in.Emit(ctx) {
                // ... handle Result states ...

                // Call user function with recovery
                func() {
                    defer func() {
                        if r := recover(); r != nil {
                            out <- Err[OUT](fmt.Errorf("panic in mapper: %v", r))
                        }
                    }()
                    result, err := m(res.Value())
                    // ... send result ...
                }()
            }
        }()
        return out
    })
}
```

However, **framework code should not panic** for recoverable errors - it should return errors. Panics in framework code indicate bugs.

## Consequences

### Positive

1. **Resilience** - one bad item doesn't crash the entire pipeline:

   ```go
   // If this panics on one item, other items still process
   flow.Map(func(s string) (int, error) {
       return strconv.Atoi(s)  // panics on nil
   })
   ```

2. **Errors are uniform** - panics become `Result[T]` errors, processable downstream:

   ```go
   // Downstream can filter, log, or recover from panic-errors
   flowerrors.OnError(logPanic).Apply(ctx, stream)
   ```

3. **Resources are protected** - deferred `close(out)` ensures channels are closed

4. **Debugging** - panic message is preserved in the error

### Negative

1. **Performance overhead** - `defer recover()` has a small cost per item

2. **Stack trace loss** - the panic's stack trace is not preserved in the error (could be enhanced)

3. **Silent failures** - users might not notice panics if they don't check errors

### When Framework Code Should Panic

The framework itself panics only for **programmer errors** that indicate misuse:

```go
// ✅ Panic: nil function is programmer error
func Map[IN, OUT any](f func(IN) (OUT, error)) Mapper[IN, OUT] {
    if f == nil {
        panic("Map: function cannot be nil")
    }
    return Mapper[IN, OUT](f)
}

// ❌ Don't panic: runtime condition, return error instead
func (s Sink[IN, OUT]) From(ctx context.Context, stream Stream[IN]) (OUT, error) {
    if stream == nil {
        var zero OUT
        return zero, errors.New("stream cannot be nil")
    }
    // ...
}
```

### Error Message Format

Panic errors include context about where they occurred:

```go
// Format: "panic in {component}: {panic value}"
"panic in mapper: runtime error: index out of range [5] with length 3"
"panic in flatmapper: interface conversion: interface {} is nil"
"panic in predicate: assignment to entry in nil map"
```
