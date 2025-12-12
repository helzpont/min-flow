# min-flow Context Usage Analysis

This document analyzes context.Context usage across min-flow to identify:

1. Places where context is correctly used
2. Places where context might be missing
3. Places where context might be unnecessary
4. Inconsistencies in context handling

---

## Executive Summary

**Overall Status**: ✅ Context usage is **consistent and correct across MVP**

- **All Transformer.Apply() methods**: ✅ Take context as first parameter
- **All Stream.Emit() methods**: ✅ Take context as first parameter
- **All terminal operations**: ✅ Take context as first parameter
- **All generator functions**: ✅ Respect context in goroutines
- **Source constructors**: ✅ Don't take context (context passed via Emit)
- **Transformer constructors**: ✅ Don't take context (applied at .Apply time)

---

## Correct Usage Patterns

### 1. Stream Sources (Don't need context parameter)

✅ **Correct - No context needed in constructor**

```go
// flow/source.go
func Empty[T any]() Stream[T]                                    // No context
func Once[T any](value T) Stream[T]                              // No context
func Range(start, end int) Stream[int]                           // No context
func Generate[T any](fn func() (T, bool, error)) Stream[T]      // No context
func Repeat[T any](value T, n int) Stream[T]                    // No context
func Interval(period time.Duration) Stream[int]                 // No context

// All these receive context in Emit() when the stream is consumed
func (e Emitter[OUT]) Emit(ctx context.Context) <-chan Result[OUT]
```

**Why this is correct**: Sources are lazy - they don't execute until `.Emit(ctx)` is called. The context is only needed when actually emitting values, not when creating the stream.

### 2. Stream.Emit() (Takes context)

✅ **Correct - All implementations take context**

```go
// flow/core/channel.go
func (e Emitter[OUT]) Emit(ctx context.Context) <-chan Result[OUT]

// flow/fast/stream.go
func (e Emitter[T]) Emit(ctx context.Context) <-chan T

// flow/observe/lifecycle.go
func (n Notification[T]) Emit(ctx context.Context) <-chan Result[T]
```

### 3. Transformer.Apply() (Takes context as first param)

✅ **Correct - All implementations take context as first parameter**

```go
// flow/core/map.go
func (m Mapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT]
func (fm FlatMapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT]

// flow/core/channel.go
func (t Transmitter[IN, OUT]) Apply(ctx context.Context, in Stream[IN]) Stream[OUT]

// flow/filter/filter.go
func (w filterTransformer[T]) Apply(ctx context.Context, s Stream[T]) Stream[T]

// flow/aggregate/reduce.go
func (r reduceTransformer[T]) Apply(ctx context.Context, s Stream[T]) Stream[T]
```

### 4. Terminal Operations (Take context as first param)

✅ **Correct - All terminals take context**

```go
// flow/types.go & flow/core/terminal.go
func Slice[T any](ctx context.Context, in Stream[T]) ([]T, error)
func First[T any](ctx context.Context, in Stream[T]) (T, error)
func Run[T any](ctx context.Context, in Stream[T]) error
func Collect[T any](ctx context.Context, stream Stream[T]) []Result[T]
func All[T any](ctx context.Context, stream Stream[T]) iter.Seq[Result[T]]

// Sink.From() also takes context
func (s Sink[IN, OUT]) From(ctx context.Context, stream Stream[IN]) (OUT, error)
```

### 5. Transformer Constructors (Don't take context)

✅ **Correct - Transformers built at construction time, applied with context**

```go
// flow/filter/filter.go
func Where[T any](predicate func(T) bool) core.Transformer[T, T]

// flow/aggregate/reduce.go
func Reduce[T any](fn func(T, T) T) core.Transformer[T, T]

// flow/flowerrors/error.go
func CatchError[T any](predicate func(error) bool, handler func(error) (T, error)) core.Transformer[T, T]

// flow/transform/utility.go
func Distinct[T comparable]() core.Transformer[T, T]
func Pairwise[T any]() core.Transformer[T, [2]T]
func StartWith[T any](values ...T) core.Transformer[T, T]
```

**Why this is correct**: Transformers are declarative - they describe _how_ to transform. The actual transformation execution (with context) happens at `.Apply()` time.

### 6. I/O Source Constructors (Don't take context parameter)

✅ **Correct - Context passed via Emit, I/O happens in goroutine**

```go
// flow/io/file.go
func ReadLines(path string) core.Stream[string]
func ReadLinesFrom(r io.Reader) core.Stream[string]

// flow/csv/csv.go
func ReadRecords(path string) core.Stream[[]string]
func ReadRecordsFrom(r io.Reader) core.Stream[[]string]

// flow/glob/glob.go
func Walk(dir string) core.Stream[string]
func WalkFiles(dir string) core.Stream[string]

// All these properly check <-ctx.Done() in their goroutines
```

---

## Consistent Context Check Pattern

All generators follow the same pattern for context checking:

```go
// Standard pattern used throughout (✅ Correct)
return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
    out := make(chan core.Result[T], bufferSize)
    go func() {
        defer close(out)
        for {
            // Do work...

            // Check context before sending
            select {
            case <-ctx.Done():
                return
            case out <- result:
            }
        }
    }()
    return out
})
```

This pattern ensures:

- ✅ Respects context cancellation
- ✅ Checks context at every send point
- ✅ Cleanly closes channel on cancel
- ✅ No goroutine leaks

---

## Analysis by Package

### ✅ flow/core

- **Emit**: ✅ Takes context in Emit()
- **Transmitter**: ✅ Takes context in Apply()
- **Mapper**: ✅ Takes context in Apply()
- **FlatMapper**: ✅ Takes context in Apply()
- **Sink**: ✅ From() takes context
- **Terminal ops**: ✅ All take context

**Status**: Perfect consistency

### ✅ flow/observe

- **Notification sources**: ✅ Context via Emit()
- **Register functions**: ✅ On*, With* are constructors (no context needed)
- **Lifecycle.go**: ✅ Materialize/Dematerialize transformers take context in Apply()

**Status**: Perfect consistency

### ✅ flow/flowerrors

- **Error transformers**: ✅ All take context in Apply()
- **Register functions**: ✅ Constructors (no context needed)
- **Interceptor.Do**: ✅ Takes context parameter

**Status**: Perfect consistency

### ✅ flow/filter

- **Where, MapWhere, Errors**: ✅ All take context in Apply()
- **Transmitter implementation**: ✅ Checks context at send points

**Status**: Perfect consistency

### ✅ flow/aggregate

- **Reduce, Fold, Scan, Batch, Window**: ✅ All take context in Apply()
- **All generators**: ✅ Check context in their goroutines

**Status**: Perfect consistency

### ✅ flow/combine

- **Merge, Concat, Zip, CombineLatest, Race**: ✅ All take context in Apply()
- **Complex multi-stream operations**: ✅ Properly propagate context to inner streams

**Status**: Perfect consistency

### ✅ flow/timing

- **Delay, Debounce, Throttle, Timeout, Sample**: ✅ All take context in Apply()
- **Timer operations**: ✅ Respect context.Done()

**Status**: Perfect consistency

### ✅ flow/io

- **ReadLines, ReadBytes, WriteLines**: ✅ Generators properly check context in goroutines
- **Path/file parameters**: ✅ Not context, correct

**Status**: Perfect consistency

### ✅ flow/csv

- **ReadRecords, ReadRecordsFrom**: ✅ Properly check context in reader loops
- **CSV-specific options**: ✅ Passed as separate config, not context

**Status**: Perfect consistency

### ✅ flow/glob

- **Walk, WalkFiles, WalkDirs**: ✅ Generators properly check context
- **Path parameters**: ✅ Not context, correct

**Status**: Perfect consistency

### ✅ flow/parallel

- **Map, MapOrdered, FlatMap, ForEach**: ✅ All take context in Apply()
- **Worker pools**: ✅ Respect context for cancellation

**Status**: Perfect consistency

### ✅ flow/transform

- **All utility transformers**: ✅ Take context in Apply()
- **Distinct, Pairwise, ConcatMap, SwitchMap, etc**: ✅ All properly respect context

**Status**: Perfect consistency

---

## Context Usage by Function Type

### Constructor Functions (Don't take context) ✅

These create objects that are lazy - context applied later:

| Category         | Functions                                               | Context |
| ---------------- | ------------------------------------------------------- | ------- |
| **Sources**      | Empty, Once, Range, Generate, Repeat, Interval, Timer   | ❌ No   |
| **Transformers** | Where, Reduce, Fold, Map, FlatMap, CatchError, Distinct | ❌ No   |
| **Factories**    | Emit, Transmit, ToSlice, ToFirst, ToRun                 | ❌ No   |
| **Composition**  | Pipe, Chain, Through                                    | ❌ No   |
| **Observers**    | OnValue, OnError, WithMetrics, WithCounter              | ❌ No   |

### Execution Functions (Take context) ✅

These execute operations - context is needed:

| Category              | Functions                       | Context             |
| --------------------- | ------------------------------- | ------------------- |
| **Stream.Emit**       | All implementations             | ✅ Yes, first param |
| **Transformer.Apply** | All implementations             | ✅ Yes, first param |
| **Terminals**         | Slice, First, Run, Collect, All | ✅ Yes, first param |
| **Sink.From**         | All Sink types                  | ✅ Yes, first param |

### Registry/Delegate Functions (Take context) ✅

These interact with the registry stored in context:

| Category            | Functions                                       | Context |
| ------------------- | ----------------------------------------------- | ------- |
| **Registry access** | GetRegistry, WithRegistry                       | ✅ Yes  |
| **Delegate lookup** | GetInterceptor, GetConfig, GetProvider, GetPool | ✅ Yes  |
| **Interceptor.Do**  | All implementations                             | ✅ Yes  |

---

## No Missing Context Issues Found ✅

**Verified**: Every place that needs context has it:

1. ✅ **Stream emission** - Always via Emit(ctx)
2. ✅ **Transformations** - Always via Apply(ctx, ...)
3. ✅ **Terminals** - Always take ctx parameter
4. ✅ **Goroutines** - Always check ctx.Done() at critical points
5. ✅ **Cancellation** - Always respected with select statements
6. ✅ **Registry propagation** - Carried via context automatically
7. ✅ **Timeouts** - Respected by context.WithTimeout()
8. ✅ **I/O operations** - Cancellable via context

---

## No Unnecessary Context Issues Found ✅

**Verified**: No functions take context when they shouldn't:

1. ✅ **Stream sources** - Correctly don't take context (lazy)
2. ✅ **Transformer constructors** - Correctly don't take context (declarative)
3. ✅ **Factory functions** - Correctly don't take context (object creation)
4. ✅ **Pure transformations** - Don't take context at definition time
5. ✅ **Data formatting** - Don't take context for encoding/decoding logic
6. ✅ **Utility functions** - Like isEven(), add() don't take context

---

## Context Testing Coverage ✅

**Found throughout codebase**:

```go
// flow/core/terminal_test.go
func TestSlice_ContextCancellation(t *testing.T)

// flow/core/context_check_test.go
func TestCheckOnCapacityCancellation(t *testing.T)
func BenchmarkContextCheckStrategies(b *testing.B)

// flow/aggregate/batch_test.go
func TestBatchContextCancellation(t *testing.T)

// flow/combine/merge_test.go
func TestMerge_ContextCancellation(t *testing.T)

// flow/timing/buffer_test.go - All timing tests include cancellation

// flow/glob/glob_test.go
func TestWalk_ContextCancellation(t *testing.T)

// flow/io/file_test.go - All file I/O tests verify cancellation
```

✅ **Strong test coverage** for context cancellation scenarios.

---

## Recommendations: All Patterns Correct

### For MVP (No changes needed)

The context usage is already:

- **Consistent** - Same patterns everywhere
- **Idiomatic** - Follows Go conventions (ADR-004)
- **Tested** - Cancellation tested thoroughly
- **Safe** - No resource leaks, proper cleanup

### For Future Phases

When adding new packages, follow these patterns:

1. **Source constructors**: No context parameter

   ```go
   func MySource(config string) core.Stream[T]  // ✅
   // NOT: func MySource(ctx context.Context, ...) // ❌
   ```

2. **Transformer constructors**: No context parameter

   ```go
   func MyTransform(param T) core.Transformer[IN, OUT]  // ✅
   ```

3. **Execution methods**: Context as first parameter

   ```go
   func (s MyStream) Apply(ctx context.Context, input Stream[IN]) Stream[OUT]  // ✅
   ```

4. **Check context in goroutines**:

   ```go
   select {
   case <-ctx.Done():
       return
   case out <- result:
   }
   ```

5. **Defer cleanup** for resources:
   ```go
   defer file.Close()
   defer reader.Stop()
   ```

---

## Summary

| Aspect                             | Status     | Evidence                        |
| ---------------------------------- | ---------- | ------------------------------- |
| **Emit() implementations**         | ✅ Correct | All take context                |
| **Apply() implementations**        | ✅ Correct | All take context as first param |
| **Terminal operations**            | ✅ Correct | All take context                |
| **Source constructors**            | ✅ Correct | Properly don't take context     |
| **Transformer constructors**       | ✅ Correct | Properly don't take context     |
| **Context checking in goroutines** | ✅ Correct | Consistent select pattern       |
| **Cancellation handling**          | ✅ Correct | Properly returns on ctx.Done()  |
| **Resource cleanup**               | ✅ Correct | defer statements throughout     |
| **Registry propagation**           | ✅ Correct | Via context, no issues          |
| **Test coverage**                  | ✅ Correct | Cancellation tests present      |

**Conclusion**: ✅ **No changes needed. Context usage across min-flow MVP is excellent.**
