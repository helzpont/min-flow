# Interceptor Redesign: Development Plan

**Status**: Proposal  
**Author**: GitHub Copilot  
**Date**: December 2024  
**Target**: min-flow v2.0

## Executive Summary

This document proposes a redesign of min-flow's interceptor/observer system to improve type safety, reduce overhead, and provide a cleaner API for both simple and advanced use cases. The design uses a layered approach with three tiers:

1. **Layer 1**: Context-embedded typed hooks for framework events (simple, automatic)
2. **Layer 2**: Callback injection for transformer-specific events (explicit, typed)
3. **Layer 3**: Channel-based tap for async/custom observation (advanced, decoupled)

## Motivation

### Current Implementation Issues

1. **Type Erasure**: All events use `any` for payloads, requiring runtime type assertions
2. **Registry Overhead**: Context lookup → Registry → Interceptor list → Event matching
3. **Complexity**: The `observe` package has multiple overlapping patterns
4. **Custom Events**: No clean way to define and handle user-defined events
5. **Coupling**: Observers must implement the full `Interceptor` interface

### Goals

- **G1**: Type-safe hooks without `any` casting for common cases
- **G2**: Minimal overhead when no observation is needed
- **G3**: Clean separation between framework and custom events
- **G4**: Support async observation without blocking the pipeline
- **G5**: Maintain backward compatibility during transition

---

## Proposed Architecture

### Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Code                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Layer 3: Channel Tap          ┌──────────────────────────────┐ │
│  (Async, Custom Events)        │  tapped := Tap(stream)       │ │
│                                │  go observe(tapped.Events)   │ │
│                                └──────────────────────────────┘ │
│                                                                  │
│  Layer 2: Callback Injection   ┌──────────────────────────────┐ │
│  (Per-Transformer)             │  Batch(opts{OnBatch: fn})    │ │
│                                └──────────────────────────────┘ │
│                                                                  │
│  Layer 1: Context Hooks        ┌──────────────────────────────┐ │
│  (Framework Events)            │  WithObserver(ctx, hooks)    │ │
│                                └──────────────────────────────┘ │
│                                                                  │
├─────────────────────────────────────────────────────────────────┤
│                      Core Stream Processing                      │
└─────────────────────────────────────────────────────────────────┘
```

---

## Layer 1: Context-Embedded Typed Hooks

### Design

Replace the registry-based interceptor system with direct, typed hook functions embedded in context.

```go
// core/hooks.go

// Hooks holds typed observation callbacks for a stream.
// All fields are optional - nil means no observation for that event.
type Hooks[T any] struct {
    OnStart    func()           // Stream begins processing
    OnValue    func(T)          // Successful value received
    OnError    func(error)      // Error received
    OnSentinel func(error)      // Sentinel received
    OnComplete func()           // Stream finished
}

// hooksKey is unexported to prevent collisions
type hooksKey[T any] struct{}

// WithHooks attaches typed hooks to the context.
// Multiple hook sets can be attached; all are invoked.
func WithHooks[T any](ctx context.Context, hooks Hooks[T]) context.Context {
    existing := getHooks[T](ctx)
    combined := combineHooks(existing, hooks)
    return context.WithValue(ctx, hooksKey[T]{}, combined)
}

// getHooks retrieves hooks from context (internal use).
func getHooks[T any](ctx context.Context) *Hooks[T] {
    if h, ok := ctx.Value(hooksKey[T]{}).(*Hooks[T]); ok {
        return h
    }
    return nil
}

// invokeValue calls OnValue if hooks exist (inlined in hot path).
func invokeValue[T any](ctx context.Context, value T) {
    if h := getHooks[T](ctx); h != nil && h.OnValue != nil {
        h.OnValue(value)
    }
}
```

### Integration with Transformers

```go
// core/map.go - inside Mapper.Apply

func (m *Mapper[IN, OUT]) transmit(ctx context.Context, in <-chan Result[IN]) <-chan Result[OUT] {
    out := make(chan Result[OUT], DefaultBufferSize)

    go func() {
        defer close(out)

        // Get hooks once at start (not per-item)
        hooks := getHooks[OUT](ctx)
        hasHooks := hooks != nil

        if hasHooks && hooks.OnStart != nil {
            hooks.OnStart()
        }
        defer func() {
            if hasHooks && hooks.OnComplete != nil {
                hooks.OnComplete()
            }
        }()

        fn := m.fn // Cache for hot loop
        for res := range in {
            // ... transform logic ...

            if hasHooks {
                if result.IsValue() && hooks.OnValue != nil {
                    hooks.OnValue(result.Value())
                } else if result.IsError() && hooks.OnError != nil {
                    hooks.OnError(result.Error())
                }
            }

            out <- result
        }
    }()

    return out
}
```

### Usage Examples

```go
// Simple counting
var count atomic.Int64
ctx := core.WithHooks(ctx, core.Hooks[int]{
    OnValue: func(v int) { count.Add(1) },
})
result := stream.Collect(ctx)
fmt.Printf("Processed %d items\n", count.Load())

// Logging with timing
start := time.Now()
ctx := core.WithHooks(ctx, core.Hooks[Order]{
    OnStart:    func() { log.Println("Processing orders...") },
    OnValue:    func(o Order) { log.Printf("Order %s: $%.2f", o.ID, o.Total) },
    OnError:    func(err error) { log.Printf("Error: %v", err) },
    OnComplete: func() { log.Printf("Done in %v", time.Since(start)) },
})

// Multiple hooks compose
ctx = core.WithHooks(ctx, metricsHooks)
ctx = core.WithHooks(ctx, loggingHooks)
ctx = core.WithHooks(ctx, alertingHooks)
```

### Performance Characteristics

- **No hooks**: Single `nil` check per transformer instantiation
- **With hooks**: One context value lookup at stream start, direct function calls per item
- **No allocation**: Hooks struct is pointer-shared, no per-item allocation

---

## Layer 2: Callback Injection

### Design

Transformers that have meaningful internal events expose typed callbacks in their options.

```go
// transform/batch.go

// BatchCallbacks defines hooks for batch-specific events.
type BatchCallbacks[T any] struct {
    OnBatchReady func(batch []T)              // Full batch ready to emit
    OnFlush      func(batch []T)              // Partial batch flushed at end
    OnOverflow   func(dropped T, batchSize int) // Item dropped (if bounded)
}

// BatchOptions configures the Batch transformer.
type BatchOptions[T any] struct {
    Size      int
    Timeout   time.Duration
    Callbacks BatchCallbacks[T] // Optional observation hooks
}

func Batch[T any](opts BatchOptions[T]) *Transformer[T, []T] {
    return Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[[]T] {
        out := make(chan Result[[]T])

        go func() {
            defer close(out)
            batch := make([]T, 0, opts.Size)

            for res := range in {
                if res.IsValue() {
                    batch = append(batch, res.Value())

                    if len(batch) >= opts.Size {
                        // Invoke callback before emitting
                        if opts.Callbacks.OnBatchReady != nil {
                            opts.Callbacks.OnBatchReady(batch)
                        }
                        out <- core.Ok(batch)
                        batch = make([]T, 0, opts.Size)
                    }
                }
            }

            // Flush remaining
            if len(batch) > 0 {
                if opts.Callbacks.OnFlush != nil {
                    opts.Callbacks.OnFlush(batch)
                }
                out <- core.Ok(batch)
            }
        }()

        return out
    })
}
```

### Standard Callback Patterns

Define common callback patterns that transformers can adopt:

```go
// core/callbacks.go

// ProgressCallbacks is a standard pattern for progress reporting.
type ProgressCallbacks struct {
    OnProgress func(processed, total int64) // Called periodically
    Interval   int64                         // Report every N items (default: 1000)
}

// RetryCallbacks is a standard pattern for retry transformers.
type RetryCallbacks[T any] struct {
    OnRetry   func(item T, attempt int, err error)
    OnGiveUp  func(item T, attempts int, lastErr error)
    OnSuccess func(item T, attempts int)
}

// ThrottleCallbacks for rate-limiting transformers.
type ThrottleCallbacks struct {
    OnThrottle func(queueDepth int, waitTime time.Duration)
    OnResume   func()
}
```

### Usage Examples

```go
// Batch with metrics
pipeline := transform.Batch(transform.BatchOptions[Order]{
    Size: 100,
    Callbacks: transform.BatchCallbacks[Order]{
        OnBatchReady: func(batch []Order) {
            metrics.RecordBatchSize(len(batch))
            total := sumOrderTotals(batch)
            metrics.RecordBatchValue(total)
        },
    },
}).Apply(ctx, orders)

// Retry with alerting
pipeline := resilience.Retry(resilience.RetryOptions[Request]{
    MaxAttempts: 3,
    Callbacks: resilience.RetryCallbacks[Request]{
        OnGiveUp: func(req Request, attempts int, err error) {
            alert.Send("Request failed after %d attempts: %v", attempts, err)
        },
    },
}).Apply(ctx, requests)
```

---

## Layer 3: Channel Tap

### Design

Provide a way to "tap" into a stream and receive a copy of all items on a separate channel, enabling async observation that won't block the main pipeline.

```go
// core/tap.go

// TappedStream wraps a stream with an observation channel.
type TappedStream[T any] struct {
    Stream Stream[T]         // The main stream (unchanged behavior)
    Events <-chan StreamEvent[T] // Observation channel (buffered)
}

// StreamEvent represents an observable event from the stream.
type StreamEvent[T any] struct {
    Type      EventType
    Value     T         // Valid when Type == EventValue
    Error     error     // Valid when Type == EventError or EventSentinel
    Timestamp time.Time
}

type EventType uint8

const (
    EventStart EventType = iota
    EventValue
    EventError
    EventSentinel
    EventComplete
)

// Tap creates a tapped stream with the specified event buffer size.
// The events channel receives copies of all stream events.
// If the events channel fills up, events are dropped (never blocks main flow).
func Tap[T any](stream Stream[T], bufferSize int) TappedStream[T] {
    events := make(chan StreamEvent[T], bufferSize)

    tappedEmitter := Emit(func(ctx context.Context) <-chan Result[T] {
        out := make(chan Result[T])

        go func() {
            defer close(out)
            defer close(events)

            // Non-blocking send helper
            send := func(e StreamEvent[T]) {
                select {
                case events <- e:
                default:
                    // Drop event if buffer full - never block pipeline
                }
            }

            send(StreamEvent[T]{Type: EventStart, Timestamp: time.Now()})

            for res := range stream.Emit(ctx) {
                // Send event (non-blocking)
                now := time.Now()
                if res.IsValue() {
                    send(StreamEvent[T]{Type: EventValue, Value: res.Value(), Timestamp: now})
                } else if res.IsSentinel() {
                    send(StreamEvent[T]{Type: EventSentinel, Error: res.Error(), Timestamp: now})
                } else {
                    send(StreamEvent[T]{Type: EventError, Error: res.Error(), Timestamp: now})
                }

                // Always forward to main output
                select {
                case <-ctx.Done():
                    return
                case out <- res:
                }
            }

            send(StreamEvent[T]{Type: EventComplete, Timestamp: time.Now()})
        }()

        return out
    })

    return TappedStream[T]{
        Stream: tappedEmitter,
        Events: events,
    }
}

// TapWithBackpressure creates a tap that blocks if the events channel fills.
// Use when you cannot afford to drop events.
func TapWithBackpressure[T any](stream Stream[T], bufferSize int) TappedStream[T] {
    // Similar but uses blocking sends
}
```

### Event Bus Extension

For custom events that span multiple pipeline stages:

```go
// observe/eventbus.go

// EventBus provides typed pub/sub for custom events.
type EventBus[E any] struct {
    subscribers []func(E)
    mu          sync.RWMutex
    buffer      chan E
    closed      atomic.Bool
}

// NewEventBus creates a new event bus with the specified buffer.
func NewEventBus[E any](bufferSize int) *EventBus[E] {
    bus := &EventBus[E]{
        buffer: make(chan E, bufferSize),
    }
    go bus.dispatch()
    return bus
}

// Subscribe registers a handler for events.
func (b *EventBus[E]) Subscribe(handler func(E)) {
    b.mu.Lock()
    b.subscribers = append(b.subscribers, handler)
    b.mu.Unlock()
}

// Emit sends an event to all subscribers.
func (b *EventBus[E]) Emit(event E) {
    if b.closed.Load() {
        return
    }
    select {
    case b.buffer <- event:
    default:
        // Buffer full, drop event
    }
}

// Close stops the event bus.
func (b *EventBus[E]) Close() {
    if b.closed.CompareAndSwap(false, true) {
        close(b.buffer)
    }
}

func (b *EventBus[E]) dispatch() {
    for event := range b.buffer {
        b.mu.RLock()
        subs := b.subscribers
        b.mu.RUnlock()

        for _, handler := range subs {
            handler(event)
        }
    }
}
```

### Usage Examples

```go
// Async metrics collection
tapped := core.Tap(expensiveStream, 1000)

// Observer runs in separate goroutine
go func() {
    var stats StreamStats
    for event := range tapped.Events {
        switch event.Type {
        case core.EventStart:
            stats.StartTime = event.Timestamp
        case core.EventValue:
            stats.Count++
        case core.EventError:
            stats.Errors++
        case core.EventComplete:
            stats.Duration = event.Timestamp.Sub(stats.StartTime)
            reportStats(stats)
        }
    }
}()

// Main processing - unaffected by slow observer
results := tapped.Stream.Collect(ctx)


// Custom events across pipeline stages
type CheckpointEvent struct {
    Stage     string
    ItemCount int64
    Timestamp time.Time
}

checkpoints := observe.NewEventBus[CheckpointEvent](100)

// Subscribe to checkpoints
checkpoints.Subscribe(func(e CheckpointEvent) {
    log.Printf("[%s] Processed %d items at %v", e.Stage, e.ItemCount, e.Timestamp)
})

// Emit from custom transformers
func MyStage(name string, bus *observe.EventBus[CheckpointEvent]) Transformer[T, T] {
    return core.Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
        out := make(chan Result[T])
        go func() {
            defer close(out)
            var count int64
            for res := range in {
                count++
                if count%10000 == 0 {
                    bus.Emit(CheckpointEvent{Stage: name, ItemCount: count, Timestamp: time.Now()})
                }
                out <- res
            }
        }()
        return out
    })
}
```

---

## Migration Strategy

### Phase 1: Add New System (Non-Breaking)

1. Implement `core.Hooks[T]` and `WithHooks`
2. Implement `core.Tap` and `TappedStream`
3. Implement `observe.EventBus[E]`
4. Add callback options to key transformers (Batch, Retry, etc.)
5. Update examples and documentation

**No changes to existing code paths. Both systems work in parallel.**

### Phase 2: Deprecate Old System

1. Mark `observe.OnValue`, `OnError`, etc. as deprecated
2. Mark `core.Registry` interceptor functions as deprecated
3. Provide migration guide with before/after examples
4. Add deprecation warnings in documentation

### Phase 3: Remove Old System (Major Version)

1. Remove deprecated functions
2. Simplify `core.Registry` to only handle non-interceptor delegates
3. Remove `interceptorDispatch` and related code
4. Update all internal usage

---

## Implementation Tasks

### Milestone 1: Core Hooks (Layer 1)

| Task                                              | Effort | Priority |
| ------------------------------------------------- | ------ | -------- |
| Implement `Hooks[T]` struct and context functions | S      | P0       |
| Add hook invocation to `Mapper`                   | S      | P0       |
| Add hook invocation to `FlatMapper`               | S      | P0       |
| Add hook invocation to `Transmitter`              | S      | P0       |
| Add hook combining (multiple `WithHooks` calls)   | M      | P1       |
| Write unit tests                                  | M      | P0       |
| Write benchmarks comparing to old system          | S      | P1       |
| Update documentation                              | M      | P1       |

### Milestone 2: Channel Tap (Layer 3)

| Task                                    | Effort | Priority |
| --------------------------------------- | ------ | -------- |
| Implement `Tap` function                | M      | P0       |
| Implement `TapWithBackpressure` variant | S      | P1       |
| Implement `EventBus[E]`                 | M      | P1       |
| Add dropped event metrics               | S      | P2       |
| Write unit tests                        | M      | P0       |
| Write integration tests                 | M      | P1       |

### Milestone 3: Callback Injection (Layer 2)

| Task                                    | Effort | Priority |
| --------------------------------------- | ------ | -------- |
| Define standard callback patterns       | S      | P0       |
| Add callbacks to `Batch` transformer    | S      | P1       |
| Add callbacks to `Retry` transformer    | S      | P1       |
| Add callbacks to `Throttle` transformer | S      | P1       |
| Add callbacks to parallel transformers  | M      | P2       |
| Document callback patterns              | M      | P1       |

### Milestone 4: Migration & Cleanup

| Task                                      | Effort | Priority |
| ----------------------------------------- | ------ | -------- |
| Write migration guide                     | M      | P0       |
| Add deprecation notices                   | S      | P0       |
| Update all examples                       | L      | P1       |
| Create compatibility shim (old API → new) | M      | P2       |
| Remove deprecated code (v2.0)             | M      | P0       |

**Effort**: S = Small (< 1 day), M = Medium (1-3 days), L = Large (3+ days)

---

## API Summary

### Layer 1: Context Hooks

```go
// Attach typed hooks to context
ctx := core.WithHooks(ctx, core.Hooks[T]{
    OnStart:    func() { ... },
    OnValue:    func(v T) { ... },
    OnError:    func(err error) { ... },
    OnComplete: func() { ... },
})

// Hooks are automatically invoked by transformers
result := mapper.Apply(ctx, stream).Collect(ctx)
```

### Layer 2: Callback Injection

```go
// Transformer-specific callbacks in options
transform.Batch(transform.BatchOptions[T]{
    Size: 100,
    Callbacks: transform.BatchCallbacks[T]{
        OnBatchReady: func(batch []T) { ... },
    },
})
```

### Layer 3: Channel Tap

```go
// Tap for async observation
tapped := core.Tap(stream, 1000)
go processEvents(tapped.Events)
result := tapped.Stream.Collect(ctx)

// Event bus for custom events
bus := observe.NewEventBus[MyEvent](100)
bus.Subscribe(handler)
bus.Emit(MyEvent{...})
```

---

## Open Questions

1. **Hook composition order**: When multiple `WithHooks` calls are made, should hooks fire in FIFO or LIFO order?

definitely FIFO.

2. **Error handling in hooks**: Should hook panics be recovered? Should hook errors stop processing?

it really depends on whether the side-effects are essential or not. provide the user the choice with functions like `Do` and `MustDo`

3. **Tap event ordering**: Is strict ordering between Events channel and main output required?

it's not. interceptors shouldn't affect processing, ordering isn't required. if use cases arise, then we can explore strict ordering.

4. **Generic type erasure**: Should we provide `Hooks[any]` for heterogeneous pipelines?

yes? or we can make it easier to define hooks for the different types. alternatively, we could define Hooks2[T,U], Hooks3[T,U,V], etc. let's avoid using `any` if we can.

5. **Metrics integration**: Should we provide built-in Prometheus/OpenTelemetry hook implementations?

i think an OpenTelemetry integration would be appropriate.

---

## Appendix: Comparison with Current System

| Aspect                    | Current                                   | Proposed                        |
| ------------------------- | ----------------------------------------- | ------------------------------- |
| Type safety               | `any` everywhere                          | Fully typed per layer           |
| Overhead (no observers)   | Registry lookup + nil check               | Single nil check                |
| Overhead (with observers) | Event matching loop                       | Direct function call            |
| Custom events             | String-based, untyped                     | Typed EventBus                  |
| Async observation         | Not supported                             | Tap with buffer                 |
| Per-transformer events    | Not supported                             | Callback injection              |
| Learning curve            | Complex (Registry, Interceptor interface) | Progressive (simple → advanced) |
