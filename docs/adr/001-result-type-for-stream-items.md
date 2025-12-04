# ADR-001: Result Type for Stream Items

## Status

Accepted

## Context

Stream processing frameworks need to handle three distinct cases for each item flowing through the pipeline:

1. **Successful values** - the normal case of processed data
2. **Errors** - recoverable failures that shouldn't terminate the entire stream
3. **Control signals** - metadata like "end of stream" that need to flow through the pipeline

Many frameworks handle these differently:

- Go channels can only carry one type, so errors must be a separate channel or encoded in the value
- Some frameworks use `(value, error)` tuples, but this conflates "no value" with "error"
- Others use exceptions/panics, which break the flow and are expensive

We needed a design that:

- Works naturally with Go's channel-based concurrency
- Distinguishes between "error occurred but stream continues" vs "stream is done"
- Allows errors to be processed, filtered, or recovered downstream
- Is type-safe and ergonomic

## Decision

We use a **Result[T]** type that wraps every item in the stream. Each Result exists in exactly one of three states:

```go
type Result[T any] struct {
    value      T
    err        error
    isSentinel bool
}

// Three states:
// 1. Value:    err == nil && !isSentinel
// 2. Error:    err != nil && !isSentinel
// 3. Sentinel: isSentinel == true (err may contain signal type)
```

Constructors enforce valid states:

- `Ok(value)` - create a value result
- `Err[T](err)` - create an error result
- `Sentinel[T](err)` - create a sentinel (control signal)
- `EndOfStream[T]()` - convenience for the common "done" sentinel

Query methods for each state:

- `IsValue()` - true only for successful values
- `IsError()` - true only for errors
- `IsSentinel()` - true only for sentinels

## Consequences

### Positive

1. **Errors flow through the pipeline** - downstream transformers can filter, recover, or aggregate errors without special handling
2. **Single channel per stream** - no need for separate error channels or complex coordination
3. **Type-safe** - the Result type is generic, preserving type information
4. **Control flow is explicit** - sentinels like EndOfStream are first-class citizens, not magic values
5. **Composable error handling** - can build retry, fallback, circuit breaker as regular transformers

### Negative

1. **Every transformer must handle Results** - adds boilerplate, though helpers like Mapper abstract this
2. **Memory overhead** - each item carries the Result wrapper (though Go's escape analysis helps)
3. **Learning curve** - users must understand the three states, not just value/error

### Neutral

1. **Sentinel semantics** - we chose to make sentinels pass through by default (not terminate), which may surprise users expecting RxJS-style completion signals
