# ADR-002: Function Types Implement Interfaces

## Status

Accepted

## Context

Go interfaces define behavior, but implementing them typically requires defining a struct with methods. For simple behaviors that are essentially "one function," this creates unnecessary boilerplate:

```go
// Without function types - verbose
type MyTransformer struct{}

func (m MyTransformer) Apply(ctx context.Context, in Stream[int]) Stream[int] {
    // implementation
}

// Usage requires instantiation
var t Transformer[int, int] = MyTransformer{}
```

Stream processing is heavily functional in nature - most operations are transformations that can be expressed as single functions. We needed a way to:

- Make it easy to create ad-hoc transformers/streams
- Reduce boilerplate for common cases
- Still allow struct-based implementations when state is needed

## Decision

We define **function types** that implement the core interfaces:

```go
// Emitter is a function type that implements Stream
type Emitter[T any] func(context.Context) <-chan Result[T]

func (e Emitter[T]) Emit(ctx context.Context) <-chan Result[T] {
    return e(ctx)
}

// Transmitter is a function type that implements Transformer
type Transmitter[IN, OUT any] func(context.Context, <-chan Result[IN]) <-chan Result[OUT]

func (t Transmitter[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT] {
    return Emit(func(ctx context.Context) <-chan Result[OUT] {
        return t(ctx, in.Emit(ctx))
    })
}

// Similarly: Mapper, FlatMapper, Sink
```

This establishes a hierarchy:

- **Interfaces** (`Stream`, `Transformer`) - define contracts
- **Function types** (`Emitter`, `Transmitter`, `Mapper`, `FlatMapper`, `Sink`) - implement interfaces

## Consequences

### Positive

1. **Inline definitions** - create streams/transformers with anonymous functions:

   ```go
   stream := core.Emit(func(ctx context.Context) <-chan Result[int] {
       // implementation
   })
   ```

2. **Consistency** - all function types follow the same pattern, making the API predictable

3. **Flexibility** - users can still define struct-based implementations when they need state or multiple methods

4. **Composability** - function types can be composed, fused, or wrapped easily

5. **Type inference** - Go can infer the generic parameters in many cases

### Negative

1. **Method receivers on functions** - unusual in Go, may confuse newcomers

2. **Can't add methods** - function types can only have the methods we define; struct implementations are more extensible

3. **Naming** - having both `Stream` (interface) and `Emitter` (function type implementing it) requires clear documentation

### Design Pattern

This follows the **functional options** and **handler function** patterns common in Go:

- `http.HandlerFunc` implements `http.Handler`
- `sort.Interface` has `sort.Slice` as a functional alternative

Our hierarchy:

```
Stream[T]           ← interface
└── Emitter[T]      ← function type implementing Stream

Transformer[IN,OUT] ← interface
├── Transmitter     ← channel-level function type
├── Mapper          ← item-level function type (1:1)
├── FlatMapper      ← item-level function type (1:N)
└── Sink            ← terminal function type
```
