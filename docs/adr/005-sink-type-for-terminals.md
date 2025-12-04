# ADR-005: Sink Type for Terminal Operations

## Status

Accepted

## Context

Terminal operations consume a stream and produce a final result. Initially, we had:

- Free functions: `Slice(ctx, stream)`, `First(ctx, stream)`, `Run(ctx, stream)`
- These existed alongside `Transformer.Apply(ctx, stream) → Stream`

This created an asymmetry:

- **Transformers** have `Apply(ctx, stream)` method → composable, discoverable
- **Terminals** were free functions → not composable, scattered API

Users observed that terminals are conceptually similar to transformers - they take a stream and produce something. The only difference is that terminals produce a final result rather than another stream.

We wanted:

1. **Parallel structure** - terminals should mirror transformers
2. **Composability** - terminals should be passable as values
3. **Discoverability** - terminal options should be easy to find (autocomplete on a type)
4. **Minimal interface growth** - avoid bloating the Stream interface

## Decision

We introduce **Sink[IN, OUT]** as a function type that:

1. Implements `Transformer[IN, OUT]` (via `Apply`)
2. Adds `From(ctx, stream) (OUT, error)` as the primary consumption method

```go
// Sink is a function type that consumes a Stream and produces a terminal result.
type Sink[IN, OUT any] func(context.Context, Stream[IN]) (OUT, error)

// From consumes a Stream - mirrors Transformer.Apply
func (s Sink[IN, OUT]) From(ctx context.Context, stream Stream[IN]) (OUT, error) {
    return s(ctx, stream)
}

// Apply implements Transformer - wraps result in single-element Stream
func (s Sink[IN, OUT]) Apply(ctx context.Context, stream Stream[IN]) Stream[OUT] {
    return Emit(func(ctx context.Context) <-chan Result[OUT] {
        out := make(chan Result[OUT], 1)
        go func() {
            defer close(out)
            result, err := s(ctx, stream)
            if err != nil {
                out <- Err[OUT](err)
            } else {
                out <- Ok(result)
            }
        }()
        return out
    })
}
```

Built-in sinks reuse existing terminal functions:

```go
func ToSlice[T any]() Sink[T, []T] { return Sink[T, []T](Slice[T]) }
func ToFirst[T any]() Sink[T, T]   { return Sink[T, T](First[T]) }
func ToRun[T any]() Sink[T, struct{}] { /* wraps Run */ }
```

## Consequences

### Positive

1. **Symmetry with Transformer**

   ```go
   // Transformer: Apply produces a Stream
   transformed := myTransformer.Apply(ctx, stream)

   // Sink: From produces a result (parallel naming)
   result, err := mySink.From(ctx, stream)
   ```

2. **Sinks are Transformers** - can be used in pipelines:

   ```go
   // Sink.Apply wraps result in a single-element stream
   resultStream := ToSlice[int]().Apply(ctx, stream)
   ```

3. **Composable** - sinks can be passed as values, stored in collections:

   ```go
   terminals := []Sink[int, any]{ToSlice[int](), ToFirst[int]()}
   ```

4. **Discoverable** - `flow.To<Tab>` shows available sinks

5. **Backward compatible** - free functions still work:

   ```go
   // Both work:
   vals, err := flow.Slice(ctx, stream)
   vals, err := flow.ToSlice[int]().From(ctx, stream)
   ```

6. **Minimal interface** - Stream interface unchanged; Sink is just a function type

### Negative

1. **Two ways to do the same thing** - may confuse users about which to prefer

2. **Verbose for simple cases** - `ToSlice[int]().From(ctx, stream)` vs `Slice(ctx, stream)`

3. **ToRun returns struct{}** - awkward type since Run only returns error

### Design Notes

**Naming: `From` vs alternatives**

- `From(ctx, stream)` was chosen to parallel `Apply(ctx, stream)`
- Considered: `Consume`, `Collect`, `Into`, `Drain`
- `From` reads well: "ToSlice from this stream"

**Pattern alignment**
This completes the function-type hierarchy:

```
Emitter      → implements Stream
Transmitter  → implements Transformer (channel-level)
Mapper       → implements Transformer (item-level, 1:1)
FlatMapper   → implements Transformer (item-level, 1:N)
Sink         → implements Transformer (terminal)
```
