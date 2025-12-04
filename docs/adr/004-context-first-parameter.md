# ADR-004: Context as First Parameter

## Status

Accepted

## Context

Go's `context.Context` is the standard mechanism for:

- Cancellation propagation
- Deadline/timeout management
- Request-scoped values (though this is controversial)

In stream processing, context is essential:

- Long-running pipelines need cancellation
- Timeouts prevent resource exhaustion
- We use context to propagate the delegate registry

The question was: where should context appear in our APIs?

Options considered:

1. **Context in constructors** - `NewStream(ctx)` returns a stream bound to that context
2. **Context in every method** - `stream.Emit(ctx)`, `transformer.Apply(ctx, stream)`
3. **Context wrapper type** - `stream.WithContext(ctx).Emit()`
4. **Context as last parameter** - `stream.Emit(ctx)` but after other params

## Decision

**Context is always the first parameter** in methods that perform work:

```go
// Stream
func (e Emitter[T]) Emit(ctx context.Context) <-chan Result[T]

// Transformer
func (t Transformer[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT]

// Sink
func (s Sink[IN, OUT]) From(ctx context.Context, in Stream[IN]) (OUT, error)

// Terminal functions
func Slice[T any](ctx context.Context, stream Stream[T]) ([]T, error)
func First[T any](ctx context.Context, stream Stream[T]) (T, error)
```

This follows the Go convention established by the standard library and `golang.org/x/net/context` documentation:

> "Do not store Contexts inside a struct type; instead, pass a Context explicitly to each function that needs it. The Context should be the first parameter."

## Consequences

### Positive

1. **Follows Go conventions** - matches `http.NewRequestWithContext`, database drivers, etc.

2. **Explicit cancellation** - callers see that operations can be cancelled

3. **Flexible composition** - different parts of a pipeline can use different contexts:

   ```go
   ctx1, cancel1 := context.WithTimeout(ctx, 5*time.Second)
   stream1 := source.Apply(ctx1, input)

   ctx2, cancel2 := context.WithTimeout(ctx, 10*time.Second)
   stream2 := expensive.Apply(ctx2, stream1)
   ```

4. **Registry propagation** - our delegate registry travels via context naturally

### Negative

1. **Repetitive** - `ctx` appears in many places:

   ```go
   result := filter.Where(p).Apply(ctx,
       transform.Map(f).Apply(ctx,
           source.Apply(ctx, input)))
   ```

2. **Verbosity** - every method call needs the context parameter

### Alternatives Considered

**Option: Context binding with `.With(ctx)`**

```go
// Considered but rejected
stream.With(ctx).Slice()  // binds context, returns bound stream
```

Rejected because:

- Creates two stream types (bound and unbound)
- Context gets hidden, easy to miss
- Doesn't match Go conventions

**Option: Context in constructor**

```go
// Considered but rejected
stream := NewStream(ctx, ...)
stream.Emit()  // uses the bound context
```

Rejected because:

- Context lifetime may not match stream lifetime
- Makes it impossible to change context mid-pipeline
- Violates the "don't store context" guideline
