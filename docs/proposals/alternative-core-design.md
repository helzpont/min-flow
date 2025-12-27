# Alternative Core Package Design

## Executive Summary

This document proposes an alternative design for the min-flow core package, built from first principles while maintaining the same goals: a composable, type-safe stream processing framework with no external dependencies. The redesign aims to address some complexity and duplication observed in the current implementation while potentially improving developer experience and performance.

---

## Analysis of Current Implementation

### Strengths

1. **Layered abstraction model** - Clear separation between interfaces, function types, and pure functions
2. **Result type** - Elegantly handles values, errors, and sentinels in a single channel
3. **Function types implementing interfaces** - Reduces boilerplate for common cases
4. **Typed hooks** - Non-invasive observation without modifying stream logic
5. **Panic recovery** - User functions can't crash pipelines

### Identified Pain Points

1. **Duplication across Mapper, FlatMapper, IterFlatMapper**

   - Each has nearly identical `runWithEveryItemCheck` and `runWithCapacityCheck` methods (~60 lines duplicated 3x)
   - Hook invocation logic repeated in each

2. **Multiple layers of indirection**

   - `Mapper` → `Apply` → `Emit` → `Transmitter` (conceptually)
   - Understanding the flow requires tracking through several abstractions

3. **Configuration complexity**

   - `TransformConfig`, `TransformOption`, `applyOptions` adds cognitive overhead
   - Context-level config vs functional options creates two configuration paths

4. **Hooks embedded in transform logic**

   - Every transformer must manually invoke hooks
   - Easy to forget or implement incorrectly in custom transformers

5. **Two Apply signatures**

   - `Apply(Stream[IN])` vs `ApplyWith(ctx, Stream[IN], ...opts)`
   - Breaks the simple Transformer interface

6. **Result type carries overhead everywhere**
   - Even for "happy path" only pipelines, every item is wrapped

---

## Proposed Alternative Design

### Core Philosophy

**"Composition over complexity"**

Rather than multiple abstraction layers (Stream → Emitter, Transformer → Transmitter → Mapper), use a single unified model with composition primitives.

### Design Principles

1. **Single source of truth** - One way to define transformations
2. **Explicit over implicit** - No magic context lookups for configuration
3. **Hooks as middleware** - Observation is a transformer, not embedded behavior
4. **Result is optional** - Allow raw streams for performance-critical paths
5. **Zero-cost abstractions** - Pay only for what you use

---

## Type Definitions

### 1. The Pipe: Universal Transformation

Instead of Mapper, FlatMapper, IterFlatMapper, Transmitter, define one universal type:

```go
// Pipe is the universal transformation primitive.
// It transforms an input channel to an output channel.
// All other abstractions (Map, Filter, FlatMap) are built on Pipe.
type Pipe[IN, OUT any] func(ctx context.Context, in <-chan IN) <-chan OUT

// Apply composes a Pipe with a source to create a new source.
func (p Pipe[IN, OUT]) Apply(source Source[IN]) Source[OUT] {
    return func(ctx context.Context) <-chan OUT {
        return p(ctx, source(ctx))
    }
}

// Then chains two Pipes together.
func (p Pipe[IN, MID]) Then(next Pipe[MID, OUT]) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan IN) <-chan OUT {
        return next(ctx, p(ctx, in))
    }
}
```

### 2. Source: Where Data Originates

```go
// Source produces a channel of values.
// This replaces Stream interface + Emitter function type.
type Source[T any] func(ctx context.Context) <-chan T

// Collect consumes a source into a slice.
func (s Source[T]) Collect(ctx context.Context) []T {
    var result []T
    for v := range s(ctx) {
        result = append(result, v)
    }
    return result
}

// First returns the first value from a source.
func (s Source[T]) First(ctx context.Context) (T, bool) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    v, ok := <-s(ctx)
    return v, ok
}
```

### 3. Result: Optional Error Handling

Rather than forcing Result everywhere, make it opt-in:

```go
// Result wraps a value with possible error state.
// Use when you need error propagation through streams.
type Result[T any] struct {
    val T
    err error
}

func Ok[T any](v T) Result[T]     { return Result[T]{val: v} }
func Fail[T any](e error) Result[T] { return Result[T]{err: e} }

func (r Result[T]) Value() T        { return r.val }
func (r Result[T]) Err() error      { return r.err }
func (r Result[T]) IsOk() bool      { return r.err == nil }
func (r Result[T]) Unwrap() (T, error) { return r.val, r.err }

// Fallible creates a Pipe that handles errors via Result.
// This bridges raw values to Result-wrapped streams.
func Fallible[IN, OUT any](f func(IN) (OUT, error)) Pipe[IN, Result[OUT]] {
    return func(ctx context.Context, in <-chan IN) <-chan Result[OUT] {
        out := make(chan Result[OUT])
        go func() {
            defer close(out)
            for v := range in {
                result, err := safeCall(f, v)
                select {
                case <-ctx.Done():
                    return
                case out <- Result[OUT]{val: result, err: err}:
                }
            }
        }()
        return out
    }
}
```

### 4. Sentinels as Values (Not Special State)

Instead of a tri-state Result, use Go's type system:

```go
// Sentinel is a marker type for control signals.
type Sentinel struct {
    Kind string
    Data any
}

// StreamItem is either a value, error, or sentinel.
// Use Go's interface{} pattern with type switches.
type StreamItem[T any] interface {
    isStreamItem()
}

type Value[T any] struct{ V T }
type Error struct{ E error }
type Signal struct{ S Sentinel }

func (Value[T]) isStreamItem() {}
func (Error) isStreamItem()    {}
func (Signal) isStreamItem()   {}

// Or simpler: just use Result[T] and a convention that
// specific error types (like io.EOF) indicate end-of-stream.
```

Actually, the current Result design is good. Let's keep it but simplify.

### 5. Simplified Result (Keep Current Design)

```go
// Result represents the outcome of processing a stream item.
type Result[T any] struct {
    value      T
    err        error
    isSentinel bool
}

// Constructors
func Ok[T any](v T) Result[T]           { return Result[T]{value: v} }
func Err[T any](e error) Result[T]      { return Result[T]{err: e} }
func End[T any]() Result[T]             { return Result[T]{isSentinel: true} }

// Accessors
func (r Result[T]) Value() T             { return r.value }
func (r Result[T]) Error() error         { return r.err }
func (r Result[T]) IsOk() bool           { return r.err == nil && !r.isSentinel }
func (r Result[T]) IsFail() bool         { return r.err != nil && !r.isSentinel }
func (r Result[T]) IsEnd() bool          { return r.isSentinel }
```

---

## Unified Transformation Model

### The Core Insight

All transformations can be expressed as:

```go
func transform[IN, OUT](in Result[IN]) iter.Seq[Result[OUT]]
```

This single signature handles:

- **Map (1:1)**: yield exactly one result
- **Filter (1:0 or 1:1)**: yield zero or one result
- **FlatMap (1:N)**: yield any number of results

### Implementation

```go
// Transform is the universal item-level transformation.
// It converts one input Result to zero or more output Results.
type Transform[IN, OUT any] func(Result[IN]) iter.Seq[Result[OUT]]

// Map creates a 1:1 Transform.
func Map[IN, OUT any](f func(IN) (OUT, error)) Transform[IN, OUT] {
    return func(in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            if in.IsFail() {
                yield(Err[OUT](in.Error()))
                return
            }
            if in.IsEnd() {
                yield(End[OUT]())
                return
            }

            out, err := safeCall(f, in.Value())
            if err != nil {
                yield(Err[OUT](err))
            } else {
                yield(Ok(out))
            }
        }
    }
}

// Filter creates a 1:0-or-1 Transform.
func Filter[T any](pred func(T) bool) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            if !in.IsOk() {
                yield(in)
                return
            }
            if pred(in.Value()) {
                yield(in)
            }
            // No yield = item filtered out
        }
    }
}

// FlatMap creates a 1:N Transform.
func FlatMap[IN, OUT any](f func(IN) iter.Seq[OUT]) Transform[IN, OUT] {
    return func(in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            if in.IsFail() {
                yield(Err[OUT](in.Error()))
                return
            }
            if in.IsEnd() {
                yield(End[OUT]())
                return
            }

            // Execute with panic recovery
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        yield(Err[OUT](PanicError{r}))
                    }
                }()
                for v := range f(in.Value()) {
                    if !yield(Ok(v)) {
                        return
                    }
                }
            }()
        }
    }
}
```

### Single Runner

Now we need only ONE function to run any Transform:

```go
// Run executes a Transform over a channel with configurable behavior.
func Run[IN, OUT any](
    ctx context.Context,
    in <-chan Result[IN],
    transform Transform[IN, OUT],
    opts ...Option,
) <-chan Result[OUT] {
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    out := make(chan Result[OUT], cfg.BufferSize)

    go func() {
        defer close(out)

        // Optional hooks
        if cfg.OnStart != nil {
            cfg.OnStart()
        }
        defer func() {
            if cfg.OnComplete != nil {
                cfg.OnComplete()
            }
        }()

        itemCount := 0
        for resIn := range in {
            for resOut := range transform(resIn) {
                // Invoke item hooks
                if cfg.OnItem != nil {
                    cfg.OnItem(resOut)
                }

                // Context check strategy
                if cfg.CheckEveryItem || itemCount >= cfg.BufferSize {
                    select {
                    case <-ctx.Done():
                        return
                    default:
                    }
                    itemCount = 0
                }

                // Send result
                select {
                case <-ctx.Done():
                    return
                case out <- resOut:
                    itemCount++
                }
            }
        }
    }()

    return out
}
```

### Composition

```go
// Pipe wraps a Transform with configuration, making it a reusable pipeline stage.
type Pipe[IN, OUT any] struct {
    transform Transform[IN, OUT]
    opts      []Option
}

func NewPipe[IN, OUT any](t Transform[IN, OUT], opts ...Option) Pipe[IN, OUT] {
    return Pipe[IN, OUT]{transform: t, opts: opts}
}

// Apply connects a Pipe to a Source.
func (p Pipe[IN, OUT]) Apply(source Source[IN]) Source[OUT] {
    return func(ctx context.Context) <-chan Result[OUT] {
        return Run(ctx, source(ctx), p.transform, p.opts...)
    }
}

// Then chains two Pipes, fusing their transforms when possible.
func (p Pipe[IN, MID]) Then(next Pipe[MID, OUT]) Pipe[IN, OUT] {
    // Fuse transforms to avoid intermediate channels
    fused := func(in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            for mid := range p.transform(in) {
                for out := range next.transform(mid) {
                    if !yield(out) {
                        return
                    }
                }
            }
        }
    }
    return Pipe[IN, OUT]{transform: fused, opts: mergeOpts(p.opts, next.opts)}
}
```

---

## Hooks as Middleware

Instead of embedding hooks in every transformer, make observation a composable layer:

```go
// Observe creates a Transform that wraps another with observation hooks.
func Observe[T any](hooks Hooks[T]) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            // Invoke appropriate hook
            switch {
            case in.IsOk():
                if hooks.OnValue != nil {
                    hooks.OnValue(in.Value())
                }
            case in.IsFail():
                if hooks.OnError != nil {
                    hooks.OnError(in.Error())
                }
            case in.IsEnd():
                if hooks.OnSentinel != nil {
                    hooks.OnSentinel()
                }
            }
            yield(in) // Pass through unchanged
        }
    }
}

// Usage:
pipeline := NewPipe(Map(double)).
    Then(NewPipe(Observe(myHooks))).
    Then(NewPipe(Filter(isEven)))
```

This approach:

- Removes hook logic from core Transform implementations
- Makes observation explicit and composable
- Allows multiple observation points in a pipeline
- Hooks can be placed anywhere, not just at transform boundaries

---

## Configuration Model

### Simple, Explicit Options

```go
type Config struct {
    BufferSize     int
    CheckEveryItem bool  // vs CheckOnCapacity

    // Observation callbacks (alternative to middleware approach)
    OnStart    func()
    OnItem     func(any)  // Called with Result[OUT]
    OnComplete func()
}

type Option func(*Config)

func WithBuffer(size int) Option {
    return func(c *Config) { c.BufferSize = size }
}

func WithFastCancel() Option {
    return func(c *Config) { c.CheckEveryItem = true }
}

func defaultConfig() Config {
    return Config{
        BufferSize:     64,
        CheckEveryItem: false,
    }
}
```

No context-level configuration - all configuration is explicit at the call site.

---

## Source Constructors

```go
// FromSlice creates a Source from a slice.
func FromSlice[T any](items []T) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])
        go func() {
            defer close(out)
            for _, item := range items {
                select {
                case <-ctx.Done():
                    return
                case out <- Ok(item):
                }
            }
        }()
        return out
    }
}

// FromIter creates a Source from an iterator.
func FromIter[T any](seq iter.Seq[T]) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])
        go func() {
            defer close(out)
            for item := range seq {
                select {
                case <-ctx.Done():
                    return
                case out <- Ok(item):
                }
            }
        }()
        return out
    }
}

// FromFunc creates a Source from a generator function.
func FromFunc[T any](gen func(yield func(T) bool)) Source[T] {
    return FromIter(gen)
}

// FromChan wraps an existing channel as a Source.
func FromChan[T any](ch <-chan T) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])
        go func() {
            defer close(out)
            for item := range ch {
                select {
                case <-ctx.Done():
                    return
                case out <- Ok(item):
                }
            }
        }()
        return out
    }
}
```

---

## Terminal Operations (Sinks)

```go
// Sink consumes a Source and produces a final result.
type Sink[IN, OUT any] func(context.Context, Source[IN]) (OUT, error)

// Collect gathers all values into a slice.
func Collect[T any]() Sink[T, []T] {
    return func(ctx context.Context, source Source[T]) ([]T, error) {
        var result []T
        for res := range source(ctx) {
            if res.IsFail() {
                return nil, res.Error()
            }
            if res.IsOk() {
                result = append(result, res.Value())
            }
        }
        return result, nil
    }
}

// First returns the first value.
func First[T any]() Sink[T, T] {
    return func(ctx context.Context, source Source[T]) (T, error) {
        ctx, cancel := context.WithCancel(ctx)
        defer cancel()

        var zero T
        for res := range source(ctx) {
            if res.IsFail() {
                return zero, res.Error()
            }
            if res.IsOk() {
                return res.Value(), nil
            }
        }
        return zero, ErrEmpty
    }
}

// ForEach runs a side-effecting function on each value.
func ForEach[T any](f func(T)) Sink[T, struct{}] {
    return func(ctx context.Context, source Source[T]) (struct{}, error) {
        for res := range source(ctx) {
            if res.IsFail() {
                return struct{}{}, res.Error()
            }
            if res.IsOk() {
                f(res.Value())
            }
        }
        return struct{}{}, nil
    }
}

// Reduce folds values with an accumulator.
func Reduce[T, ACC any](init ACC, f func(ACC, T) ACC) Sink[T, ACC] {
    return func(ctx context.Context, source Source[T]) (ACC, error) {
        acc := init
        for res := range source(ctx) {
            if res.IsFail() {
                return acc, res.Error()
            }
            if res.IsOk() {
                acc = f(acc, res.Value())
            }
        }
        return acc, nil
    }
}
```

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/lguimbarda/min-flow/flow/core"
)

func main() {
    ctx := context.Background()

    // Define pipeline
    source := core.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

    pipeline := core.NewPipe(core.Filter(func(n int) bool {
        return n%2 == 0  // Keep evens
    })).Then(core.NewPipe(core.Map(func(n int) (int, error) {
        return n * n, nil  // Square
    }))).Then(core.NewPipe(core.Observe(core.Hooks[int]{
        OnValue: func(v int) { fmt.Printf("Processing: %d\n", v) },
    })))

    // Execute
    results, err := core.Collect[int]()(ctx, pipeline.Apply(source))
    if err != nil {
        panic(err)
    }

    fmt.Printf("Results: %v\n", results)
    // Output:
    // Processing: 4
    // Processing: 16
    // Processing: 36
    // Processing: 64
    // Processing: 100
    // Results: [4 16 36 64 100]
}
```

---

## File Structure

```
flow/core/
    result.go      # Result[T] type and constructors
    source.go      # Source[T] type and constructors (FromSlice, FromIter, etc.)
    transform.go   # Transform[IN,OUT], Map, Filter, FlatMap
    pipe.go        # Pipe[IN,OUT], composition, Run()
    sink.go        # Sink[IN,OUT], Collect, First, Reduce, ForEach
    observe.go     # Hooks, Observe transform
    config.go      # Config, Option, defaults
    errors.go      # ErrEmpty, PanicError, etc.
```

---

## Comparison Summary

| Aspect                         | Current Design                                      | Proposed Design                       |
| ------------------------------ | --------------------------------------------------- | ------------------------------------- |
| **Abstraction Layers**         | 3 (Interface, Function Type, Pure)                  | 2 (Transform, Pipe)                   |
| **Transformer Types**          | 4 (Mapper, FlatMapper, IterFlatMapper, Transmitter) | 1 (Transform)                         |
| **Hook Integration**           | Embedded in each transformer                        | Composable middleware                 |
| **Configuration**              | Context + Functional Options                        | Functional Options only               |
| **Runner Implementation**      | Duplicated 3x                                       | Single `Run()` function               |
| **Interface/Function Duality** | Stream/Emitter, Transformer/Transmitter             | Source (function only), Pipe (struct) |
| **Apply Signatures**           | `Apply(stream)` + `ApplyWith(ctx, stream, opts)`    | `Apply(source)` only                  |

---

## Trade-offs

### Pros of Proposed Design

1. **Less code duplication** - Single `Run()` handles all transforms
2. **Simpler mental model** - One Transform type, one composition method
3. **Explicit observation** - Hooks are transforms, not magic context values
4. **Cleaner separation** - Transform logic is pure, channel management is in `Run()`
5. **Better fusion** - `Then()` automatically fuses transforms

### Cons of Proposed Design

1. **No interface-based polymorphism** - Can't have struct-based Stream implementations
2. **Hooks require explicit placement** - Must add `Observe()` to pipeline
3. **Breaking change** - Incompatible with current API
4. **Less familiar** - Different from RxJS/ReactiveX patterns

### Mitigation

The interface polymorphism can be added back if needed:

```go
type Emitter[T any] interface {
    Emit(context.Context) <-chan Result[T]
}

func (s Source[T]) Emit(ctx context.Context) <-chan Result[T] {
    return s(ctx)
}
```

---

## Extensibility: Custom Hooks and Configuration

A key concern for any framework is extensibility. Users will inevitably need:

- **Custom hooks** beyond `OnValue`/`OnError`/`OnSentinel` (e.g., `OnSuccess`, `OnFailure`, `OnWarning`, `OnSkipped`)
- **Custom configuration** beyond `BufferSize`/`CheckEveryItem`

### Approach 1: User-Defined Observe Transforms (Recommended)

Since `Transform` is just a function, users can create domain-specific observation transforms:

```go
// User-defined hooks for a validation pipeline
type ValidationHooks[T any] struct {
    OnValid    func(T)
    OnInvalid  func(T, error)
    OnWarning  func(T, string)
    OnSkipped  func(T, string)
}

// User creates their own Observe-like transform
func ObserveValidation[T any](hooks ValidationHooks[T]) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            if in.IsOk() {
                // User determines what constitutes warning vs valid vs skipped
                // This logic is domain-specific
                hooks.OnValid(in.Value())
            } else if in.IsFail() {
                hooks.OnInvalid(in.Value(), in.Error())
            }
            yield(in)
        }
    }
}

// Usage
pipeline := NewPipe(Validate[Record]()).
    Then(NewPipe(ObserveValidation(ValidationHooks[Record]{
        OnValid:   func(r Record) { metrics.ValidCount.Inc() },
        OnInvalid: func(r Record, e error) { log.Warn("invalid", r, e) },
        OnWarning: func(r Record, w string) { log.Info("warning", r, w) },
        OnSkipped: func(r Record, reason string) { metrics.SkipCount.Inc() },
    })))
```

**Benefits:**

- No framework changes needed
- Type-safe: hooks are typed to the domain
- Composable: can have multiple observation points
- Explicit: clear where observation happens

### Approach 2: Tagged Results for Rich Signaling

For complex pipelines that need to signal more than just value/error/end, extend Result:

```go
// Tag represents additional metadata about a result
type Tag string

const (
    TagNone    Tag = ""
    TagWarning Tag = "warning"
    TagSkipped Tag = "skipped"
    TagRetried Tag = "retried"
)

type Result[T any] struct {
    value      T
    err        error
    isSentinel bool
    tag        Tag        // New: optional categorization
    metadata   any        // New: arbitrary payload
}

// Additional constructors
func Warn[T any](v T, msg string) Result[T] {
    return Result[T]{value: v, tag: TagWarning, metadata: msg}
}

func Skip[T any](v T, reason string) Result[T] {
    return Result[T]{value: v, tag: TagSkipped, metadata: reason}
}

// Observe can now dispatch on tags
func Observe[T any](hooks Hooks[T]) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            switch {
            case in.IsOk() && in.Tag() == TagWarning:
                if hooks.OnWarning != nil {
                    hooks.OnWarning(in.Value(), in.Metadata().(string))
                }
            case in.IsOk() && in.Tag() == TagSkipped:
                if hooks.OnSkipped != nil {
                    hooks.OnSkipped(in.Value(), in.Metadata().(string))
                }
            case in.IsOk():
                if hooks.OnValue != nil {
                    hooks.OnValue(in.Value())
                }
            // ... etc
            }
            yield(in)
        }
    }
}
```

**Trade-off:** Adds complexity to core Result type, but enables richer signaling.

### Approach 3: Event Bus Pattern

For maximum flexibility, use an event-driven model:

```go
// Event represents any observable occurrence in the pipeline
type Event struct {
    Kind string
    Data any
    Time time.Time
}

// EventSink receives events (could be channel, callback, etc.)
type EventSink interface {
    Emit(Event)
}

// ObserveEvents creates a Transform that emits events to a sink
func ObserveEvents[T any](sink EventSink, eventMapper func(Result[T]) []Event) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            for _, event := range eventMapper(in) {
                sink.Emit(event)
            }
            yield(in)
        }
    }
}

// Usage - user defines what events to emit
sink := NewChannelSink(100)
pipeline := NewPipe(Process[Record]()).
    Then(NewPipe(ObserveEvents(sink, func(r Result[Record]) []Event {
        if r.IsOk() {
            return []Event{{Kind: "record.processed", Data: r.Value()}}
        }
        return []Event{{Kind: "record.failed", Data: r.Error()}}
    })))
```

**Benefits:** Maximum flexibility, decoupled event consumers
**Costs:** More indirection, potential performance overhead

---

## Custom Configuration

### Approach 1: Type-Keyed Context (Current min-flow Pattern)

```go
// Core provides generic config storage
type configKey[C any] struct{}

func WithConfig[C any](ctx context.Context, cfg C) context.Context {
    return context.WithValue(ctx, configKey[C]{}, cfg)
}

func GetConfig[C any](ctx context.Context) (C, bool) {
    if cfg, ok := ctx.Value(configKey[C]{}).(C); ok {
        return cfg, true
    }
    var zero C
    return zero, false
}

// Users define their own config types
type RetryConfig struct {
    MaxAttempts int
    Backoff     time.Duration
}

type ValidationConfig struct {
    StrictMode  bool
    AllowEmpty  bool
}

// Usage
ctx := context.Background()
ctx = WithConfig(ctx, &RetryConfig{MaxAttempts: 3})
ctx = WithConfig(ctx, &ValidationConfig{StrictMode: true})

// In a custom transform
func RetryTransform[T any](f func(T) (T, error)) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            // Get config from wherever context is available
            // Note: This requires passing ctx through...
        }
    }
}
```

**Problem:** Transforms don't have access to context in this design!

### Approach 2: Context-Aware Transforms (Better)

Modify Transform to receive context:

```go
// Transform now receives context for config access
type Transform[IN, OUT any] func(context.Context, Result[IN]) iter.Seq[Result[OUT]]

// Map becomes:
func Map[IN, OUT any](f func(IN) (OUT, error)) Transform[IN, OUT] {
    return func(ctx context.Context, in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            // Can now access GetConfig[MyConfig](ctx)
            // ...
        }
    }
}

// Run passes context to transform
func Run[IN, OUT any](
    ctx context.Context,
    in <-chan Result[IN],
    transform Transform[IN, OUT],
    opts ...Option,
) <-chan Result[OUT] {
    // ...
    for resIn := range in {
        for resOut := range transform(ctx, resIn) {  // Pass ctx
            // ...
        }
    }
}
```

Now custom transforms can access configuration:

```go
func ValidateWithConfig[T Validatable]() Transform[T, T] {
    return func(ctx context.Context, in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            cfg, _ := GetConfig[*ValidationConfig](ctx)

            if in.IsOk() {
                err := in.Value().Validate(cfg.StrictMode)
                if err != nil && !cfg.AllowEmpty {
                    yield(Err[T](err))
                    return
                }
            }
            yield(in)
        }
    }
}
```

### Approach 3: Config as Option Parameter

For transforms that need specific configuration, pass it directly:

```go
// Configuration is explicit, not magical
func ValidateWith[T Validatable](cfg ValidationConfig) Transform[T, T] {
    return func(in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            if in.IsOk() {
                err := in.Value().Validate(cfg.StrictMode)
                // ...
            }
            yield(in)
        }
    }
}

// Usage - explicit config at construction time
pipeline := NewPipe(ValidateWith[Record](ValidationConfig{
    StrictMode: true,
    AllowEmpty: false,
}))
```

**Benefits:**

- Explicit dependencies
- No context magic
- Easy to test

**Costs:**

- Can't change config dynamically
- Must thread config through pipeline construction

### Approach 4: Hybrid - Core + Extensions

Separate core config (which affects Run behavior) from transform-specific config:

```go
// Core config - affects channel/goroutine behavior
type CoreConfig struct {
    BufferSize     int
    CheckEveryItem bool
}

type Option func(*CoreConfig)

// Transform-specific config - passed via closures or context
// Each transform decides its own config strategy

// Example: Transform that uses both
func RetryMap[IN, OUT any](
    f func(IN) (OUT, error),
    retryOpts RetryOptions,  // Transform-specific, explicit
) Transform[IN, OUT] {
    return func(ctx context.Context, in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            // Use retryOpts directly (explicit)
            // Can also check GetConfig[*GlobalRetryPolicy](ctx) for overrides
        }
    }
}
```

---

## Recommended Extensibility Pattern

Based on the design principles ("explicit over implicit"), I recommend:

### For Custom Hooks

**Use Approach 1: User-defined Observe transforms**

- Users create domain-specific hook containers
- Users create domain-specific Observe transforms
- Core provides the `Transform` type and `Run` function
- No framework changes needed for new hook types

### For Custom Config

**Use Approach 3 + Context fallback**

- Primary: Pass config explicitly to transform constructors
- Fallback: Context-based config for cross-cutting concerns
- Modify Transform signature to include context

```go
// Final recommended Transform signature
type Transform[IN, OUT any] func(ctx context.Context, in Result[IN]) iter.Seq[Result[OUT]]
```

This keeps explicit config as the default (good for testing, readability) while allowing context-based config for concerns that span the entire pipeline (like observability settings, feature flags, etc.).

---

## Parallel Processing Extensions

Parallelism is critical for high-throughput pipelines. The proposed design should accommodate parallel execution without complicating the core abstractions.

### Parallelism Patterns

#### 1. Parallel Map (Data Parallelism)

Process multiple items concurrently with a worker pool:

```go
// ParallelMap processes items concurrently with n workers.
// Order is NOT preserved - use OrderedParallelMap if needed.
func ParallelMap[IN, OUT any](n int, f func(IN) (OUT, error)) Transform[IN, OUT] {
    return func(ctx context.Context, in Result[IN]) iter.Seq[Result[OUT]] {
        // This doesn't work well with the per-item Transform signature!
        // Parallelism needs to operate at the channel level, not item level.
    }
}
```

**Problem:** The `Transform` signature processes one item at a time, which doesn't naturally support parallelism across items.

**Solution:** Parallel operations are `Pipe`s, not `Transform`s:

```go
// ParallelPipe creates a Pipe that processes items with n concurrent workers.
func ParallelPipe[IN, OUT any](n int, transform Transform[IN, OUT], opts ...Option) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
        cfg := applyOpts(opts...)
        out := make(chan Result[OUT], cfg.BufferSize)

        // Create worker pool
        var wg sync.WaitGroup
        wg.Add(n)

        for i := 0; i < n; i++ {
            go func() {
                defer wg.Done()
                for resIn := range in {
                    for resOut := range transform(ctx, resIn) {
                        select {
                        case <-ctx.Done():
                            return
                        case out <- resOut:
                        }
                    }
                }
            }()
        }

        // Close output when all workers done
        go func() {
            wg.Wait()
            close(out)
        }()

        return out
    }
}

// Usage
pipeline := ParallelPipe(8, Map(expensiveComputation))
```

#### 2. Ordered Parallel Map

Preserve input order while processing in parallel:

```go
// OrderedParallelPipe processes items concurrently but emits in input order.
func OrderedParallelPipe[IN, OUT any](n int, transform Transform[IN, OUT], opts ...Option) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
        cfg := applyOpts(opts...)
        out := make(chan Result[OUT], cfg.BufferSize)

        // Use a semaphore for concurrency limit
        sem := make(chan struct{}, n)

        // Ordered results channel per input
        type indexed struct {
            seq     int
            results []Result[OUT]
        }
        resultsCh := make(chan indexed, n)

        go func() {
            defer close(out)

            var wg sync.WaitGroup
            seq := 0
            pending := make(map[int][]Result[OUT])
            nextSeq := 0

            // Producer: dispatch work
            go func() {
                for resIn := range in {
                    currentSeq := seq
                    seq++

                    sem <- struct{}{} // Acquire
                    wg.Add(1)

                    go func(s int, r Result[IN]) {
                        defer func() {
                            <-sem // Release
                            wg.Done()
                        }()

                        var results []Result[OUT]
                        for resOut := range transform(ctx, r) {
                            results = append(results, resOut)
                        }

                        select {
                        case <-ctx.Done():
                        case resultsCh <- indexed{seq: s, results: results}:
                        }
                    }(currentSeq, resIn)
                }

                wg.Wait()
                close(resultsCh)
            }()

            // Consumer: emit in order
            for ir := range resultsCh {
                pending[ir.seq] = ir.results

                // Emit all consecutive available results
                for {
                    results, ok := pending[nextSeq]
                    if !ok {
                        break
                    }
                    delete(pending, nextSeq)
                    nextSeq++

                    for _, r := range results {
                        select {
                        case <-ctx.Done():
                            return
                        case out <- r:
                        }
                    }
                }
            }
        }()

        return out
    }
}
```

#### 3. Fan-Out / Fan-In

Split a stream to multiple consumers, then merge results:

```go
// FanOut distributes items round-robin to n output channels.
func FanOut[T any](n int) Pipe[T, T] {
    return func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
        // Returns a multiplexed channel
        // Actually, fan-out produces multiple channels...
    }
}
```

**Problem:** `Pipe` returns a single channel. Fan-out needs multiple outputs.

**Solution:** Define fan-out/fan-in as higher-order operations on Sources:

```go
// Fanout splits a source into n sources.
// Items are distributed round-robin.
func Fanout[T any](source Source[T], n int) []Source[T] {
    return func(ctx context.Context) []<-chan Result[T] {
        in := source(ctx)
        outs := make([]chan Result[T], n)
        for i := range outs {
            outs[i] = make(chan Result[T])
        }

        go func() {
            defer func() {
                for _, ch := range outs {
                    close(ch)
                }
            }()

            i := 0
            for res := range in {
                select {
                case <-ctx.Done():
                    return
                case outs[i] <- res:
                    i = (i + 1) % n
                }
            }
        }()

        // Convert to Source slice
        sources := make([]Source[T], n)
        for i, ch := range outs {
            ch := ch // capture
            sources[i] = func(ctx context.Context) <-chan Result[T] {
                return ch
            }
        }
        return sources
    }(ctx)
}

// Merge combines multiple sources into one (unordered).
func Merge[T any](sources ...Source[T]) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])
        var wg sync.WaitGroup

        for _, src := range sources {
            wg.Add(1)
            go func(s Source[T]) {
                defer wg.Done()
                for res := range s(ctx) {
                    select {
                    case <-ctx.Done():
                        return
                    case out <- res:
                    }
                }
            }(src)
        }

        go func() {
            wg.Wait()
            close(out)
        }()

        return out
    }
}

// Usage: Process with 4 parallel pipelines, then merge
sources := Fanout(source, 4)
processed := make([]Source[int], 4)
for i, src := range sources {
    processed[i] = pipeline.Apply(src)
}
result := Merge(processed...)
```

#### 4. Parallel via Partitioning (Keyed Parallelism)

Process items in parallel while ensuring items with the same key go to the same worker (useful for stateful processing):

```go
// Partition splits items by key, ensuring same keys go to same worker.
func Partition[T any, K comparable](
    source Source[T],
    n int,
    keyFn func(T) K,
) []Source[T] {
    return func(ctx context.Context) []Source[T] {
        in := source(ctx)
        outs := make([]chan Result[T], n)
        for i := range outs {
            outs[i] = make(chan Result[T], 64) // Buffer to prevent blocking
        }

        go func() {
            defer func() {
                for _, ch := range outs {
                    close(ch)
                }
            }()

            for res := range in {
                if !res.IsOk() {
                    // Broadcast errors/sentinels to all
                    for _, ch := range outs {
                        select {
                        case <-ctx.Done():
                            return
                        case ch <- res:
                        }
                    }
                    continue
                }

                // Hash key to partition
                key := keyFn(res.Value())
                partition := hashKey(key) % n

                select {
                case <-ctx.Done():
                    return
                case outs[partition] <- res:
                }
            }
        }()

        // Convert to Source slice
        sources := make([]Source[T], n)
        for i, ch := range outs {
            ch := ch
            sources[i] = func(ctx context.Context) <-chan Result[T] {
                return ch
            }
        }
        return sources
    }(ctx)
}
```

### Design Implications

#### Transform vs Pipe for Parallelism

| Abstraction | Parallelism Support | Use Case                      |
| ----------- | ------------------- | ----------------------------- |
| `Transform` | ❌ Single-item      | Pure transformations, fusion  |
| `Pipe`      | ✅ Multi-item       | Parallel execution, buffering |

**Recommendation:** Keep `Transform` for pure item-level logic and `Pipe` for channel-level operations including parallelism.

#### Fusion and Parallelism

Fused transforms execute in a single goroutine. Parallelism requires breaking fusion:

```go
// These are fused - runs in 1 goroutine
pipeline := NewPipe(Map(f1)).Then(NewPipe(Map(f2))).Then(NewPipe(Map(f3)))

// This is parallel - runs in n goroutines
parallel := ParallelPipe(8, Fuse(Map(f1), Map(f2), Map(f3)))
```

The user explicitly chooses between fusion (less overhead, sequential) and parallelism (more overhead, concurrent).

#### Configuration for Parallelism

```go
type ParallelConfig struct {
    Workers       int           // Number of concurrent workers
    PreserveOrder bool          // Whether to maintain input order
    BufferSize    int           // Per-worker buffer size
    OnWorkerPanic func(any)     // Panic handler
}

type ParallelOption func(*ParallelConfig)

func WithWorkers(n int) ParallelOption {
    return func(c *ParallelConfig) { c.Workers = n }
}

func WithPreserveOrder(preserve bool) ParallelOption {
    return func(c *ParallelConfig) { c.PreserveOrder = preserve }
}

// Unified parallel pipe constructor
func Parallel[IN, OUT any](transform Transform[IN, OUT], opts ...ParallelOption) Pipe[IN, OUT] {
    cfg := defaultParallelConfig()
    for _, opt := range opts {
        opt(&cfg)
    }

    if cfg.PreserveOrder {
        return orderedParallelPipe(cfg.Workers, transform, cfg)
    }
    return unorderedParallelPipe(cfg.Workers, transform, cfg)
}
```

### Parallel Extension File Structure

```
flow/
  core/
    ... (core types)
  parallel/
    parallel.go     # Parallel, ParallelConfig, ParallelOption
    fanout.go       # Fanout, Merge
    partition.go    # Partition, keyed parallelism
    ordered.go      # OrderedParallel implementation
    pool.go         # Worker pool utilities
```

### Key Decisions for Parallelism

1. **Parallelism is a `Pipe`, not a `Transform`** - Channel-level operation
2. **Explicit opt-in** - No automatic parallelization
3. **Order preservation is optional** - Unordered is faster, ordered when needed
4. **Separate package** - `flow/parallel` to keep core simple
5. **Panic isolation** - Worker panics don't crash the pipeline

---

## Distributed Stream Processing

Moving from single-process parallelism to multi-node distributed processing introduces fundamental challenges. This section explores how the proposed design can accommodate distributed execution.

### Challenges of Distribution

| Challenge              | Description                                    |
| ---------------------- | ---------------------------------------------- |
| **Serialization**      | Data must cross process/network boundaries     |
| **Delivery Semantics** | At-most-once, at-least-once, exactly-once      |
| **Ordering**           | Maintaining order across partitions            |
| **State**              | Coordinating stateful operations across nodes  |
| **Failure Recovery**   | Handling node failures, retries, checkpointing |
| **Backpressure**       | Flow control across network boundaries         |

### Architectural Approach: Boundary Abstraction

The key insight is that **channels are a local abstraction**. For distribution, we need a boundary layer that bridges local pipelines to remote communication.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Node A                                            │
│  ┌──────────┐    ┌───────────┐    ┌──────────────┐                         │
│  │  Source  │───►│  Pipeline │───►│   Boundary   │─────────┐               │
│  └──────────┘    └───────────┘    │   (Encode)   │         │               │
│                                   └──────────────┘         │               │
└────────────────────────────────────────────────────────────┼───────────────┘
                                                             │
                                          ┌──────────────────┘
                                          │  Network (Kafka, NATS, gRPC, etc.)
                                          └──────────────────┐
                                                             │
┌────────────────────────────────────────────────────────────┼───────────────┐
│                           Node B                           │               │
│                                   ┌──────────────┐         │               │
│                                   │   Boundary   │◄────────┘               │
│  ┌──────────┐    ┌───────────┐◄───│   (Decode)   │                         │
│  │   Sink   │◄───│  Pipeline │    └──────────────┘                         │
│  └──────────┘    └───────────┘                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Core Abstractions for Distribution

#### 1. Codec: Serialization Contract

```go
// Codec handles serialization for distributed transport.
type Codec[T any] interface {
    Encode(T) ([]byte, error)
    Decode([]byte) (T, error)
}

// JSONCodec is a simple JSON-based codec.
type JSONCodec[T any] struct{}

func (c JSONCodec[T]) Encode(v T) ([]byte, error) {
    return json.Marshal(v)
}

func (c JSONCodec[T]) Decode(data []byte) (T, error) {
    var v T
    err := json.Unmarshal(data, &v)
    return v, err
}

// ResultCodec wraps a value codec to handle Result[T].
type ResultCodec[T any] struct {
    inner Codec[T]
}

func (c ResultCodec[T]) Encode(r Result[T]) ([]byte, error) {
    // Encode result state + value/error
    envelope := struct {
        IsOk       bool   `json:"ok"`
        IsSentinel bool   `json:"sentinel,omitempty"`
        Value      []byte `json:"value,omitempty"`
        Error      string `json:"error,omitempty"`
    }{
        IsOk:       r.IsOk(),
        IsSentinel: r.IsEnd(),
    }

    if r.IsOk() {
        data, err := c.inner.Encode(r.Value())
        if err != nil {
            return nil, err
        }
        envelope.Value = data
    } else if r.IsFail() {
        envelope.Error = r.Error().Error()
    }

    return json.Marshal(envelope)
}
```

#### 2. Transport: Network Abstraction

```go
// Transport abstracts the network layer.
// Implementations: Kafka, NATS, Redis Streams, gRPC, TCP, etc.
type Transport interface {
    // Sender sends messages to a topic/queue
    NewSender(topic string) (Sender, error)
    // Receiver receives messages from a topic/queue
    NewReceiver(topic string, opts ...ReceiverOption) (Receiver, error)
    Close() error
}

type Sender interface {
    Send(ctx context.Context, msg Message) error
    Close() error
}

type Receiver interface {
    Receive(ctx context.Context) <-chan Message
    Ack(ctx context.Context, msg Message) error
    Nack(ctx context.Context, msg Message) error
    Close() error
}

type Message struct {
    Key       []byte            // Partition key
    Value     []byte            // Encoded payload
    Headers   map[string]string // Metadata
    Timestamp time.Time
    Offset    int64             // For acknowledging
}
```

#### 3. Boundary Pipes: Bridging Local and Remote

```go
// ToRemote creates a Pipe that sends results to a remote transport.
func ToRemote[T any](
    transport Transport,
    topic string,
    codec Codec[Result[T]],
    opts ...RemoteOption,
) Pipe[T, T] {
    return func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
        cfg := applyRemoteOpts(opts...)
        out := make(chan Result[T], cfg.BufferSize)

        go func() {
            defer close(out)

            sender, err := transport.NewSender(topic)
            if err != nil {
                out <- Err[T](fmt.Errorf("failed to create sender: %w", err))
                return
            }
            defer sender.Close()

            for res := range in {
                // Encode and send
                data, err := codec.Encode(res)
                if err != nil {
                    out <- Err[T](fmt.Errorf("encode error: %w", err))
                    continue
                }

                msg := Message{
                    Value:     data,
                    Timestamp: time.Now(),
                }

                // Add partition key if configured
                if cfg.KeyFunc != nil && res.IsOk() {
                    msg.Key = []byte(cfg.KeyFunc(res.Value()))
                }

                if err := sender.Send(ctx, msg); err != nil {
                    out <- Err[T](fmt.Errorf("send error: %w", err))
                    continue
                }

                // Pass through locally (optional - for monitoring)
                if cfg.PassThrough {
                    out <- res
                }
            }
        }()

        return out
    }
}

// FromRemote creates a Source that receives from a remote transport.
func FromRemote[T any](
    transport Transport,
    topic string,
    codec Codec[Result[T]],
    opts ...RemoteOption,
) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        cfg := applyRemoteOpts(opts...)
        out := make(chan Result[T], cfg.BufferSize)

        go func() {
            defer close(out)

            receiver, err := transport.NewReceiver(topic)
            if err != nil {
                out <- Err[T](fmt.Errorf("failed to create receiver: %w", err))
                return
            }
            defer receiver.Close()

            for msg := range receiver.Receive(ctx) {
                res, err := codec.Decode(msg.Value)
                if err != nil {
                    out <- Err[T](fmt.Errorf("decode error: %w", err))
                    receiver.Nack(ctx, msg)
                    continue
                }

                select {
                case <-ctx.Done():
                    return
                case out <- res:
                    // Ack after successful delivery to channel
                    if cfg.AutoAck {
                        receiver.Ack(ctx, msg)
                    }
                }
            }
        }()

        return out
    }
}
```

### Delivery Semantics

#### At-Least-Once (Default)

Messages are retried on failure. Consumers must handle duplicates.

```go
// At-least-once: ack after processing
source := FromRemote(transport, "topic", codec, WithAutoAck(false))
pipeline := NewPipe(Map(process)).Then(NewPipe(Ack(transport)))

// Ack creates a passthrough Pipe that acknowledges after processing
func Ack[T any](transport Transport) Pipe[T, T] {
    return func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
        // Implementation tracks message offsets and acks after emit
    }
}
```

#### Exactly-Once (With Idempotency)

Requires idempotent operations or deduplication.

```go
// Deduplicate filters out already-seen message IDs
func Deduplicate[T any](store DedupeStore, idFunc func(T) string) Transform[T, T] {
    return func(ctx context.Context, in Result[T]) iter.Seq[Result[T]] {
        return func(yield func(Result[T]) bool) {
            if !in.IsOk() {
                yield(in)
                return
            }

            id := idFunc(in.Value())
            if store.Seen(ctx, id) {
                // Skip duplicate
                return
            }

            store.Mark(ctx, id)
            yield(in)
        }
    }
}

type DedupeStore interface {
    Seen(ctx context.Context, id string) bool
    Mark(ctx context.Context, id string) error
}
```

#### At-Most-Once

Fire and forget - fastest but may lose messages.

```go
source := FromRemote(transport, "topic", codec,
    WithAutoAck(true),      // Ack immediately on receive
    WithPrefetch(100),      // High prefetch for throughput
)
```

### Distributed Partitioning

#### Partition by Key

```go
// PartitionedSink sends to partitions based on key.
func PartitionedSink[T any](
    transport Transport,
    topicPrefix string,
    numPartitions int,
    keyFunc func(T) string,
    codec Codec[Result[T]],
) Pipe[T, T] {
    return func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
        out := make(chan Result[T])

        // Create sender per partition
        senders := make([]Sender, numPartitions)
        for i := 0; i < numPartitions; i++ {
            topic := fmt.Sprintf("%s-%d", topicPrefix, i)
            sender, _ := transport.NewSender(topic)
            senders[i] = sender
        }

        go func() {
            defer close(out)
            defer func() {
                for _, s := range senders {
                    s.Close()
                }
            }()

            for res := range in {
                if !res.IsOk() {
                    // Broadcast errors to all partitions or handle specially
                    out <- res
                    continue
                }

                // Route to partition
                key := keyFunc(res.Value())
                partition := hashString(key) % numPartitions

                data, _ := codec.Encode(res)
                senders[partition].Send(ctx, Message{
                    Key:   []byte(key),
                    Value: data,
                })

                out <- res
            }
        }()

        return out
    }
}
```

### Stateful Distributed Processing

For stateful operations across nodes, use external state stores:

```go
// StateStore abstracts distributed state (Redis, etcd, etc.)
type StateStore interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Put(ctx context.Context, key string, value []byte) error
    Delete(ctx context.Context, key string) error
    // For atomic operations
    CompareAndSwap(ctx context.Context, key string, expected, new []byte) (bool, error)
}

// StatefulMap applies a stateful transformation with external state.
func StatefulMap[IN, OUT, STATE any](
    store StateStore,
    stateKey func(IN) string,
    stateCodec Codec[STATE],
    f func(IN, STATE) (OUT, STATE, error),
) Transform[IN, OUT] {
    return func(ctx context.Context, in Result[IN]) iter.Seq[Result[OUT]] {
        return func(yield func(Result[OUT]) bool) {
            if !in.IsOk() {
                yield(Err[OUT](in.Error()))
                return
            }

            key := stateKey(in.Value())

            // Load state
            var state STATE
            if data, err := store.Get(ctx, key); err == nil {
                state, _ = stateCodec.Decode(data)
            }

            // Apply function
            out, newState, err := f(in.Value(), state)
            if err != nil {
                yield(Err[OUT](err))
                return
            }

            // Save state
            data, _ := stateCodec.Encode(newState)
            store.Put(ctx, key, data)

            yield(Ok(out))
        }
    }
}
```

### Checkpointing and Recovery

```go
// Checkpoint periodically saves progress for recovery.
type Checkpoint struct {
    StreamID  string
    Offset    int64
    Timestamp time.Time
    Metadata  map[string]string
}

type CheckpointStore interface {
    Save(ctx context.Context, cp Checkpoint) error
    Load(ctx context.Context, streamID string) (Checkpoint, error)
}

// WithCheckpointing wraps a Source to checkpoint progress.
func WithCheckpointing[T any](
    source Source[T],
    store CheckpointStore,
    streamID string,
    interval time.Duration,
) Source[T] {
    return func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])

        go func() {
            defer close(out)

            // Load last checkpoint
            cp, _ := store.Load(ctx, streamID)
            offset := cp.Offset

            ticker := time.NewTicker(interval)
            defer ticker.Stop()

            for res := range source(ctx) {
                offset++

                select {
                case <-ticker.C:
                    store.Save(ctx, Checkpoint{
                        StreamID:  streamID,
                        Offset:    offset,
                        Timestamp: time.Now(),
                    })
                default:
                }

                out <- res
            }

            // Final checkpoint
            store.Save(ctx, Checkpoint{
                StreamID:  streamID,
                Offset:    offset,
                Timestamp: time.Now(),
            })
        }()

        return out
    }
}
```

### Example: Distributed Pipeline

```go
func main() {
    ctx := context.Background()

    // Connect to Kafka
    transport, _ := kafka.NewTransport(kafka.Config{
        Brokers: []string{"localhost:9092"},
    })
    defer transport.Close()

    codec := JSONCodec[Record]{}
    resultCodec := ResultCodec[Record]{inner: codec}

    // Node A: Ingest and partition
    if nodeType == "ingester" {
        source := FromHTTP[Record](":8080", "/ingest")

        pipeline := NewPipe(Validate[Record]()).
            Then(NewPipe(Enrich[Record]())).
            Then(PartitionedSink(transport, "records", 8,
                func(r Record) string { return r.CustomerID },
                resultCodec))

        _, _ = ForEach(func(r Record) {})(ctx, pipeline.Apply(source))
    }

    // Node B: Process partition
    if nodeType == "processor" {
        partitionID := os.Getenv("PARTITION_ID")
        topic := fmt.Sprintf("records-%s", partitionID)

        source := FromRemote(transport, topic, resultCodec,
            WithAutoAck(false),
            WithConsumerGroup("processors"))

        // Redis for state
        stateStore := redis.NewStateStore("localhost:6379")

        pipeline := NewPipe(StatefulMap(stateStore,
            func(r Record) string { return r.CustomerID },
            JSONCodec[CustomerState]{},
            processWithState)).
            Then(ToRemote(transport, "processed", resultCodec))

        _, _ = ForEach(func(r Record) {})(ctx, pipeline.Apply(source))
    }
}
```

### Design Implications

#### What Stays in Core

- `Source`, `Pipe`, `Transform`, `Result` - unchanged
- Local parallelism - unchanged
- Composition via `Then()` - unchanged

#### What Lives in Extensions

```
flow/
  core/           # Unchanged
  parallel/       # Local parallelism
  distributed/
    codec.go      # Codec interface, JSON/Protobuf codecs
    transport.go  # Transport interface
    boundary.go   # ToRemote, FromRemote
    checkpoint.go # Checkpointing
    state.go      # Distributed state
    dedup.go      # Deduplication
  transport/
    kafka/        # Kafka Transport implementation
    nats/         # NATS Transport implementation
    redis/        # Redis Streams implementation
    grpc/         # gRPC streaming implementation
```

### Key Decisions for Distribution

1. **Boundary is explicit** - `ToRemote`/`FromRemote` are visible in pipeline
2. **Transport is pluggable** - Core doesn't know about Kafka, NATS, etc.
3. **Codecs are user-provided** - Framework doesn't impose serialization format
4. **Delivery semantics are configurable** - User chooses tradeoffs
5. **State is external** - No built-in distributed state, use Redis/etcd/etc.
6. **Checkpointing is opt-in** - Wrap source when recovery matters

---

## Remote Worker Pool (Scatter-Gather)

The previous section described pipeline-to-pipeline communication via message queues. This section addresses a different pattern: **local orchestration with remote computation** — where the source and sink remain on a single coordinator node, but processing fans out to remote worker nodes.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Coordinator Node                                     │
│                                                                             │
│  ┌────────┐    ┌─────────────┐         ┌─────────────┐    ┌────────┐       │
│  │ Source │───►│  Scatter    │─────────│   Gather    │───►│  Sink  │       │
│  └────────┘    └─────────────┘         └─────────────┘    └────────┘       │
│                      │ │ │                   ▲ ▲ ▲                          │
└──────────────────────┼─┼─┼───────────────────┼─┼─┼──────────────────────────┘
                       │ │ │     Network       │ │ │
         ┌─────────────┘ │ └─────────────┐     │ │ │
         │               │               │     │ │ │
         ▼               ▼               ▼     │ │ │
    ┌─────────┐    ┌─────────┐    ┌─────────┐  │ │ │
    │Worker 1 │────│Worker 2 │────│Worker N │──┘ │ │
    │(Node A) │    │(Node B) │    │(Node C) │────┘ │
    └─────────┘    └─────────┘    └─────────┘──────┘
```

### Use Cases

- **CPU-intensive transforms** - Distribute computation across machines
- **GPU workers** - Fan out to nodes with specialized hardware
- **Memory-intensive operations** - Leverage aggregate cluster memory
- **Geo-distributed processing** - Process data near its origin

### Core Abstractions

#### 1. Worker: Remote Execution Unit

```go
// Worker represents a remote execution endpoint.
type Worker interface {
    // ID returns a unique identifier for this worker
    ID() string

    // Process sends an item for processing and returns the result.
    // For streaming, we use channels instead.
    Process(ctx context.Context, req Request) (Response, error)

    // Stream opens a bidirectional stream for continuous processing.
    Stream(ctx context.Context) (WorkerStream, error)

    // Health checks if the worker is available.
    Health(ctx context.Context) error

    Close() error
}

type Request struct {
    ID      string // Correlation ID for matching responses
    Payload []byte
}

type Response struct {
    ID      string
    Payload []byte
    Error   string // Non-empty if worker encountered an error
}

type WorkerStream interface {
    Send(Request) error
    Recv() (Response, error)
    Close() error
}
```

#### 2. WorkerPool: Managing Remote Workers

```go
// WorkerPool manages a set of remote workers.
type WorkerPool interface {
    // Workers returns currently available workers.
    Workers() []Worker

    // Acquire gets an available worker (blocks if none available).
    Acquire(ctx context.Context) (Worker, error)

    // Release returns a worker to the pool.
    Release(Worker)

    // AddWorker dynamically adds a worker.
    AddWorker(Worker) error

    // RemoveWorker removes a worker from the pool.
    RemoveWorker(workerID string) error

    Close() error
}

// StaticPool is a simple pool with fixed workers.
type StaticPool struct {
    workers []Worker
    avail   chan Worker
    mu      sync.RWMutex
}

func NewStaticPool(workers ...Worker) *StaticPool {
    p := &StaticPool{
        workers: workers,
        avail:   make(chan Worker, len(workers)),
    }
    for _, w := range workers {
        p.avail <- w
    }
    return p
}

func (p *StaticPool) Acquire(ctx context.Context) (Worker, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case w := <-p.avail:
        return w, nil
    }
}

func (p *StaticPool) Release(w Worker) {
    p.avail <- w
}
```

#### 3. Load Balancing Strategies

```go
// LoadBalancer determines which worker receives each item.
type LoadBalancer interface {
    // Select picks a worker for the given item.
    Select(ctx context.Context, pool WorkerPool, item any) (Worker, error)
}

// RoundRobin distributes items evenly across workers.
type RoundRobin struct {
    counter atomic.Uint64
}

func (r *RoundRobin) Select(ctx context.Context, pool WorkerPool, _ any) (Worker, error) {
    workers := pool.Workers()
    if len(workers) == 0 {
        return nil, errors.New("no workers available")
    }
    idx := r.counter.Add(1) % uint64(len(workers))
    return workers[idx], nil
}

// KeyAffinity routes items with the same key to the same worker.
// Useful for stateful processing.
type KeyAffinity[T any] struct {
    keyFunc   func(T) string
    mu        sync.RWMutex
    affinities map[string]string // key -> workerID
}

func (k *KeyAffinity[T]) Select(ctx context.Context, pool WorkerPool, item any) (Worker, error) {
    key := k.keyFunc(item.(T))

    k.mu.RLock()
    workerID, exists := k.affinities[key]
    k.mu.RUnlock()

    if exists {
        // Find the worker with this ID
        for _, w := range pool.Workers() {
            if w.ID() == workerID {
                return w, nil
            }
        }
    }

    // Assign to a worker (consistent hashing or round-robin)
    workers := pool.Workers()
    idx := hashString(key) % len(workers)
    w := workers[idx]

    k.mu.Lock()
    k.affinities[key] = w.ID()
    k.mu.Unlock()

    return w, nil
}

// LeastLoaded picks the worker with the fewest in-flight requests.
type LeastLoaded struct {
    inFlight sync.Map // workerID -> *atomic.Int64
}

func (l *LeastLoaded) Select(ctx context.Context, pool WorkerPool, _ any) (Worker, error) {
    workers := pool.Workers()
    if len(workers) == 0 {
        return nil, errors.New("no workers available")
    }

    var best Worker
    bestCount := int64(math.MaxInt64)

    for _, w := range workers {
        count := l.getCount(w.ID())
        if count < bestCount {
            bestCount = count
            best = w
        }
    }

    l.increment(best.ID())
    return best, nil
}
```

### Scatter-Gather Pipe

```go
// ScatterGather creates a Pipe that distributes work to remote workers.
type ScatterGatherConfig struct {
    Pool           WorkerPool
    Codec          Codec[any]       // Serialization for network
    LoadBalancer   LoadBalancer
    PreserveOrder  bool             // Whether to emit in input order
    Timeout        time.Duration    // Per-item timeout
    MaxRetries     int              // Retries on worker failure
    CircuitBreaker *CircuitBreaker  // Optional circuit breaker
}

func ScatterGather[IN, OUT any](
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
        out := make(chan Result[OUT], 64)

        go func() {
            defer close(out)

            if cfg.PreserveOrder {
                scatterGatherOrdered(ctx, in, out, codec, outCodec, cfg)
            } else {
                scatterGatherUnordered(ctx, in, out, codec, outCodec, cfg)
            }
        }()

        return out
    }
}

func scatterGatherUnordered[IN, OUT any](
    ctx context.Context,
    in <-chan Result[IN],
    out chan<- Result[OUT],
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
) {
    var wg sync.WaitGroup
    sem := make(chan struct{}, len(cfg.Pool.Workers())*2) // Limit concurrency

    for resIn := range in {
        if !resIn.IsOk() {
            out <- Err[OUT](resIn.Error())
            continue
        }

        sem <- struct{}{}
        wg.Add(1)

        go func(item IN) {
            defer func() {
                <-sem
                wg.Done()
            }()

            result := executeOnWorker(ctx, item, codec, outCodec, cfg)

            select {
            case <-ctx.Done():
            case out <- result:
            }
        }(resIn.Value())
    }

    wg.Wait()
}

func scatterGatherOrdered[IN, OUT any](
    ctx context.Context,
    in <-chan Result[IN],
    out chan<- Result[OUT],
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
) {
    type indexed struct {
        seq    int
        result Result[OUT]
    }

    results := make(chan indexed, 64)
    var wg sync.WaitGroup

    // Dispatcher
    go func() {
        seq := 0
        for resIn := range in {
            if !resIn.IsOk() {
                results <- indexed{seq: seq, result: Err[OUT](resIn.Error())}
                seq++
                continue
            }

            currentSeq := seq
            seq++
            wg.Add(1)

            go func(s int, item IN) {
                defer wg.Done()
                result := executeOnWorker(ctx, item, codec, outCodec, cfg)

                select {
                case <-ctx.Done():
                case results <- indexed{seq: s, result: result}:
                }
            }(currentSeq, resIn.Value())
        }

        wg.Wait()
        close(results)
    }()

    // Reorder and emit
    pending := make(map[int]Result[OUT])
    nextSeq := 0

    for ir := range results {
        pending[ir.seq] = ir.result

        for {
            r, ok := pending[nextSeq]
            if !ok {
                break
            }
            delete(pending, nextSeq)
            nextSeq++

            select {
            case <-ctx.Done():
                return
            case out <- r:
            }
        }
    }
}

func executeOnWorker[IN, OUT any](
    ctx context.Context,
    item IN,
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
) Result[OUT] {
    // Serialize
    payload, err := codec.Encode(item)
    if err != nil {
        return Err[OUT](fmt.Errorf("encode error: %w", err))
    }

    req := Request{
        ID:      uuid.New().String(),
        Payload: payload,
    }

    // Retry loop
    var lastErr error
    for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
        // Select worker
        worker, err := cfg.LoadBalancer.Select(ctx, cfg.Pool, item)
        if err != nil {
            lastErr = err
            continue
        }

        // Execute with timeout
        execCtx := ctx
        if cfg.Timeout > 0 {
            var cancel context.CancelFunc
            execCtx, cancel = context.WithTimeout(ctx, cfg.Timeout)
            defer cancel()
        }

        resp, err := worker.Process(execCtx, req)
        if err != nil {
            lastErr = err
            continue
        }

        if resp.Error != "" {
            lastErr = errors.New(resp.Error)
            continue
        }

        // Deserialize response
        var out OUT
        if err := outCodec.Decode(resp.Payload, &out); err != nil {
            return Err[OUT](fmt.Errorf("decode error: %w", err))
        }

        return Ok(out)
    }

    return Err[OUT](fmt.Errorf("all retries failed: %w", lastErr))
}
```

### Worker Implementations

#### gRPC Worker

```go
// GRPCWorker connects to a remote worker via gRPC.
type GRPCWorker struct {
    id     string
    conn   *grpc.ClientConn
    client WorkerServiceClient // Generated from proto
}

func NewGRPCWorker(id, address string) (*GRPCWorker, error) {
    conn, err := grpc.Dial(address, grpc.WithInsecure()) // Configure TLS in production
    if err != nil {
        return nil, err
    }

    return &GRPCWorker{
        id:     id,
        conn:   conn,
        client: NewWorkerServiceClient(conn),
    }, nil
}

func (w *GRPCWorker) ID() string { return w.id }

func (w *GRPCWorker) Process(ctx context.Context, req Request) (Response, error) {
    resp, err := w.client.Process(ctx, &ProcessRequest{
        Id:      req.ID,
        Payload: req.Payload,
    })
    if err != nil {
        return Response{}, err
    }

    return Response{
        ID:      resp.Id,
        Payload: resp.Payload,
        Error:   resp.Error,
    }, nil
}

func (w *GRPCWorker) Health(ctx context.Context) error {
    _, err := w.client.Health(ctx, &HealthRequest{})
    return err
}

func (w *GRPCWorker) Close() error {
    return w.conn.Close()
}
```

#### HTTP Worker

```go
// HTTPWorker connects to a remote worker via HTTP.
type HTTPWorker struct {
    id         string
    baseURL    string
    httpClient *http.Client
}

func NewHTTPWorker(id, baseURL string) *HTTPWorker {
    return &HTTPWorker{
        id:      id,
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (w *HTTPWorker) Process(ctx context.Context, req Request) (Response, error) {
    body, _ := json.Marshal(req)

    httpReq, err := http.NewRequestWithContext(ctx, "POST",
        w.baseURL+"/process", bytes.NewReader(body))
    if err != nil {
        return Response{}, err
    }
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := w.httpClient.Do(httpReq)
    if err != nil {
        return Response{}, err
    }
    defer resp.Body.Close()

    var response Response
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return Response{}, err
    }

    return response, nil
}
```

### Resilience Patterns

#### Circuit Breaker

```go
// CircuitBreaker prevents cascading failures by stopping requests to failing workers.
type CircuitBreaker struct {
    mu          sync.RWMutex
    state       map[string]circuitState // workerID -> state
    threshold   int                      // Failures before opening
    timeout     time.Duration            // Time before half-open
}

type circuitState struct {
    failures   int
    state      string // "closed", "open", "half-open"
    lastFailed time.Time
}

func (cb *CircuitBreaker) Allow(workerID string) bool {
    cb.mu.RLock()
    s := cb.state[workerID]
    cb.mu.RUnlock()

    switch s.state {
    case "closed", "":
        return true
    case "open":
        if time.Since(s.lastFailed) > cb.timeout {
            cb.mu.Lock()
            cb.state[workerID] = circuitState{state: "half-open"}
            cb.mu.Unlock()
            return true
        }
        return false
    case "half-open":
        return true
    }
    return false
}

func (cb *CircuitBreaker) RecordSuccess(workerID string) {
    cb.mu.Lock()
    cb.state[workerID] = circuitState{state: "closed"}
    cb.mu.Unlock()
}

func (cb *CircuitBreaker) RecordFailure(workerID string) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    s := cb.state[workerID]
    s.failures++
    s.lastFailed = time.Now()

    if s.failures >= cb.threshold {
        s.state = "open"
    }

    cb.state[workerID] = s
}
```

#### Hedged Requests

Send request to multiple workers, use first response:

```go
// HedgedScatterGather sends to multiple workers and uses the first response.
func HedgedScatterGather[IN, OUT any](
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
    hedgeFactor int, // Number of parallel requests
) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
        out := make(chan Result[OUT], 64)

        go func() {
            defer close(out)

            for resIn := range in {
                if !resIn.IsOk() {
                    out <- Err[OUT](resIn.Error())
                    continue
                }

                // Race multiple workers
                result := hedgedExecute(ctx, resIn.Value(), codec, outCodec, cfg, hedgeFactor)

                select {
                case <-ctx.Done():
                    return
                case out <- result:
                }
            }
        }()

        return out
    }
}

func hedgedExecute[IN, OUT any](
    ctx context.Context,
    item IN,
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
    hedgeFactor int,
) Result[OUT] {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    resultCh := make(chan Result[OUT], hedgeFactor)

    workers := cfg.Pool.Workers()
    if len(workers) < hedgeFactor {
        hedgeFactor = len(workers)
    }

    // Launch hedged requests
    for i := 0; i < hedgeFactor; i++ {
        go func(w Worker) {
            result := executeOnWorkerDirect(ctx, item, w, codec, outCodec, cfg)
            select {
            case resultCh <- result:
            case <-ctx.Done():
            }
        }(workers[i])
    }

    // Return first success (or last error)
    var lastErr Result[OUT]
    for i := 0; i < hedgeFactor; i++ {
        select {
        case <-ctx.Done():
            return Err[OUT](ctx.Err())
        case r := <-resultCh:
            if r.IsOk() {
                return r
            }
            lastErr = r
        }
    }

    return lastErr
}
```

### Dynamic Worker Discovery

```go
// ServiceDiscovery finds workers dynamically.
type ServiceDiscovery interface {
    // Discover returns currently available workers.
    Discover(ctx context.Context) ([]WorkerEndpoint, error)

    // Watch returns a channel of worker updates.
    Watch(ctx context.Context) <-chan WorkerUpdate
}

type WorkerEndpoint struct {
    ID      string
    Address string
    Tags    map[string]string
}

type WorkerUpdate struct {
    Added   []WorkerEndpoint
    Removed []string // Worker IDs
}

// DynamicPool manages workers based on service discovery.
type DynamicPool struct {
    discovery ServiceDiscovery
    factory   func(WorkerEndpoint) (Worker, error)
    pool      *StaticPool
    mu        sync.RWMutex
}

func NewDynamicPool(discovery ServiceDiscovery, factory func(WorkerEndpoint) (Worker, error)) *DynamicPool {
    return &DynamicPool{
        discovery: discovery,
        factory:   factory,
        pool:      NewStaticPool(),
    }
}

func (p *DynamicPool) Start(ctx context.Context) error {
    // Initial discovery
    endpoints, err := p.discovery.Discover(ctx)
    if err != nil {
        return err
    }

    for _, ep := range endpoints {
        w, err := p.factory(ep)
        if err != nil {
            continue
        }
        p.pool.AddWorker(w)
    }

    // Watch for updates
    go func() {
        for update := range p.discovery.Watch(ctx) {
            for _, ep := range update.Added {
                if w, err := p.factory(ep); err == nil {
                    p.pool.AddWorker(w)
                }
            }
            for _, id := range update.Removed {
                p.pool.RemoveWorker(id)
            }
        }
    }()

    return nil
}
```

### Example: Image Processing Pipeline

```go
func main() {
    ctx := context.Background()

    // Create worker pool
    pool := NewStaticPool(
        NewGRPCWorker("gpu-1", "gpu-node-1:50051"),
        NewGRPCWorker("gpu-2", "gpu-node-2:50051"),
        NewGRPCWorker("gpu-3", "gpu-node-3:50051"),
    )
    defer pool.Close()

    // Configure scatter-gather
    cfg := ScatterGatherConfig{
        Pool:          pool,
        LoadBalancer:  &LeastLoaded{},
        PreserveOrder: true,
        Timeout:       10 * time.Second,
        MaxRetries:    2,
        CircuitBreaker: NewCircuitBreaker(5, 30*time.Second),
    }

    // Pipeline: local source → remote processing → local sink
    source := FromSlice(imageURLs)

    pipeline := NewPipe(Map(downloadImage)).
        Then(ScatterGather(ImageCodec{}, ImageCodec{}, cfg)). // Remote GPU processing
        Then(NewPipe(Map(saveToStorage)))

    results, err := Collect[Image]()(ctx, pipeline.Apply(source))
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Processed %d images across %d workers", len(results), len(pool.Workers()))
}
```

### Worker-Side Implementation

Workers are separate processes/services:

```go
// Worker service (runs on remote nodes)
func main() {
    lis, _ := net.Listen("tcp", ":50051")

    server := grpc.NewServer()
    RegisterWorkerServiceServer(server, &imageProcessor{})

    log.Println("Worker listening on :50051")
    server.Serve(lis)
}

type imageProcessor struct {
    UnimplementedWorkerServiceServer
}

func (p *imageProcessor) Process(ctx context.Context, req *ProcessRequest) (*ProcessResponse, error) {
    // Decode image
    var img Image
    if err := json.Unmarshal(req.Payload, &img); err != nil {
        return &ProcessResponse{Error: err.Error()}, nil
    }

    // GPU-accelerated processing
    processed, err := processImageOnGPU(ctx, img)
    if err != nil {
        return &ProcessResponse{Error: err.Error()}, nil
    }

    // Encode result
    payload, _ := json.Marshal(processed)
    return &ProcessResponse{
        Id:      req.Id,
        Payload: payload,
    }, nil
}
```

### Streaming Workers (Persistent Connections)

For high-throughput scenarios, use streaming instead of request-response:

```go
// StreamingScatterGather maintains persistent connections to workers.
func StreamingScatterGather[IN, OUT any](
    codec Codec[IN],
    outCodec Codec[OUT],
    cfg ScatterGatherConfig,
) Pipe[IN, OUT] {
    return func(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
        out := make(chan Result[OUT], 64)

        go func() {
            defer close(out)

            // Open streams to all workers
            streams := make(map[string]WorkerStream)
            responses := make(chan Response, 64)
            pending := make(map[string]int) // requestID -> sequence

            for _, w := range cfg.Pool.Workers() {
                stream, err := w.Stream(ctx)
                if err != nil {
                    continue
                }
                streams[w.ID()] = stream

                // Receiver goroutine per worker
                go func(s WorkerStream) {
                    for {
                        resp, err := s.Recv()
                        if err != nil {
                            return
                        }
                        responses <- resp
                    }
                }(stream)
            }

            // Dispatch and collect
            var wg sync.WaitGroup
            seq := 0

            // Dispatcher
            wg.Add(1)
            go func() {
                defer wg.Done()
                workerIdx := 0
                workers := cfg.Pool.Workers()

                for resIn := range in {
                    if !resIn.IsOk() {
                        out <- Err[OUT](resIn.Error())
                        continue
                    }

                    payload, _ := codec.Encode(resIn.Value())
                    reqID := fmt.Sprintf("%d", seq)
                    pending[reqID] = seq
                    seq++

                    // Round-robin to streams
                    stream := streams[workers[workerIdx].ID()]
                    stream.Send(Request{ID: reqID, Payload: payload})
                    workerIdx = (workerIdx + 1) % len(workers)
                }
            }()

            // Collector (handles ordering if needed)
            // ... similar to ordered scatter-gather
        }()

        return out
    }
}
```

### Design Summary: Remote Workers

| Aspect             | Approach                                  |
| ------------------ | ----------------------------------------- |
| **Orchestration**  | Local coordinator, remote workers         |
| **Transport**      | gRPC, HTTP, or custom protocol            |
| **Load Balancing** | Round-robin, least-loaded, key-affinity   |
| **Resilience**     | Retries, circuit breaker, hedged requests |
| **Discovery**      | Static, Consul, Kubernetes, DNS           |
| **Ordering**       | Optional preserve-order mode              |

### Key Decisions for Remote Workers

1. **ScatterGather is a Pipe** - Fits naturally in the pipeline
2. **Worker is an interface** - Supports gRPC, HTTP, custom protocols
3. **Load balancing is pluggable** - Choose strategy per use case
4. **Circuit breaker is opt-in** - Not all workloads need it
5. **Order preservation has cost** - Unordered is faster, ordered when needed
6. **Workers are stateless by default** - Key-affinity for stateful cases

### Package Structure (Updated)

```
flow/
  core/           # Unchanged
  parallel/       # Local multi-core parallelism
  distributed/
    codec.go      # Codec interface
    transport.go  # Transport for message queues
    boundary.go   # ToRemote, FromRemote (queue-based)
    worker/
      worker.go       # Worker interface
      pool.go         # WorkerPool, StaticPool, DynamicPool
      loadbalancer.go # RoundRobin, LeastLoaded, KeyAffinity
      circuit.go      # CircuitBreaker
      scatter.go      # ScatterGather Pipe
      discovery.go    # ServiceDiscovery interface
    grpc/         # gRPC Worker implementation
    http/         # HTTP Worker implementation
  transport/
    kafka/        # Kafka Transport
    nats/         # NATS Transport
```

---

### Comparison with Other Frameworks

| Feature          | min-flow (proposed)   | Kafka Streams      | Apache Flink       |
| ---------------- | --------------------- | ------------------ | ------------------ |
| **Deployment**   | Any (library)         | Kafka-only         | Cluster required   |
| **State**        | External (pluggable)  | Built-in (RocksDB) | Built-in (managed) |
| **Exactly-once** | Via dedup/idempotency | Built-in           | Built-in           |
| **Windowing**    | Extension package     | Built-in           | Built-in           |
| **Complexity**   | Low                   | Medium             | High               |
| **Transport**    | Pluggable             | Kafka              | Various            |

**min-flow's niche:** Simple, composable pipelines that can scale from local to distributed without framework lock-in.

---

## Conclusion

The proposed design achieves the same functionality with:

- **~40% less code** in the core package
- **Single source of truth** for transformation execution
- **Composable observation** instead of embedded hooks
- **Clearer composition model** with `Then()` chaining

The key insight is that `iter.Seq[Result[OUT]]` as the return type for transforms unifies Map, Filter, and FlatMap into a single abstraction, eliminating the need for separate types and their duplicated runner logic.

This design prioritizes **simplicity and composability** while maintaining the same robustness guarantees (panic recovery, context cancellation, error propagation) as the current implementation.
