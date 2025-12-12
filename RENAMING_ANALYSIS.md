# min-flow MVP Renaming Analysis

This document outlines potential renaming for MVP symbols. Items are organized by category and likelihood of change. **No changes proposed** denotes symbols that are well-named and shouldn't be renamed.

---

## Core Interfaces & Types

### Stream[T]

- **Current**: Stream
- **Status**: ✅ **No changes proposed** - Clear, concise, widely understood term
- **Rationale**: "Stream" is intuitive for a flow of data. All alternatives (Sequence, Iterable, Flow) are either less precise or more generic.

### Result[T]

- **Current**: Result
- **Status**: ✅ **No changes proposed** - Clear and well-established pattern
- **Rationale**: Captures the three states (Value, Error, Sentinel). Familiar from Go's error handling patterns. Alternatives (Outcome, EventWrapper) are verbose or unclear.

### Transformer[IN, OUT]

- **Current**: Transformer
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Operator** - More Rx-like, shorter, but less descriptive
  2. **Stage** - Emphasizes pipeline composition, but less clear about transformation
  3. **Step** - Similar to Stage, more concise
  4. **Transform** (interface-level only) - More verb-focused, but conflicts with function name
- **Recommendation**: Keep **Transformer** for MVP. It's clear, descriptive, and unambiguous.

---

## Function Types (Implement Interfaces)

### Mapper[IN, OUT]

- **Current**: Mapper
- **Status**: ✅ **No changes proposed** - Domain-standard terminology
- **Rationale**: "Mapper" is well-established in FP/streaming libraries. Consistent with FlatMapper naming.

### FlatMapper[IN, OUT]

- **Current**: FlatMapper
- **Status**: ✅ **No changes proposed** - Standard FP terminology
- **Rationale**: Industry-standard name, immediately understood by FP practitioners.

### Emitter[T]

- **Current**: Emitter
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Source** - More descriptive of role, but less technical
  2. **Producer** - More Kafka-like terminology
  3. **Generator** - More Python-like, good fit for Go 1.23 iter.Seq
  4. **StreamFn** / **StreamFunc** - Describes it as stream-producing function, but verbose
- **Recommendation**: Keep **Emitter** for MVP. It emphasizes the active nature of producing values.

### Transmitter[IN, OUT]

- **Current**: Transmitter
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Pipe** - Emphasizes data flow, but conflicts with Pipe() function
  2. **Channel** - More Go-idiomatic, but conflicts with Go's channel concept
  3. **Process** - Generic but less precise
  4. **Handler** - More imperative, but clear
  5. **Relay** - Technical term, emphasizes passing data through
- **Recommendation**: Keep **Transmitter** for MVP. Good parallelism with Emitter (both are "-er" function types).

### Sink[IN, OUT]

- **Current**: Sink
- **Status**: ✅ **No changes proposed** - Clear terminal operation metaphor
- **Rationale**: "Sink" is a well-understood terminal operation metaphor (opposite of Source/Emitter). Matches Rx terminology.

---

## Stream Lifecycle Events

### StreamStart / StreamEnd

- **Current**: StreamStart, StreamEnd
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives for both**:
  1. **OnStart** / **OnComplete** - More callback-like, less event-like
  2. **Begin** / **End** - Shorter but less precise
  3. **Opened** / **Closed** - More resource-like terminology
  4. **Acquire** / **Release** - More explicit about resource management
- **Recommendation**: Keep **StreamStart** / **StreamEnd** for MVP. Clear, symmetric, event-like naming.

### Item-level Events (ItemReceived, ValueReceived, ErrorOccurred, SentinelReceived, ItemEmitted)

- **Current**: ItemReceived, ValueReceived, ErrorOccurred, SentinelReceived, ItemEmitted
- **Status**: ⚠️ **Naming convention could be improved**
- **Current Pattern**: Verb + past tense/past participle (Received, Occurred, Emitted)
- **Alternative Patterns**:
  1. **Verb + "On" prefix**: OnReceiveItem, OnReceiveValue, OnError, OnEmitItem - More callback-like
  2. **Verb + "At" prefix**: AtItemReceived, AtValueReceived - Emphasizes timing
  3. **Verb + Gerund**: ItemReceiving, ValueReceiving, ItemEmitting - Less common in Go
  4. **Past tense action verbs**: ItemArrived, ValueArrived, ErrorHappened, ErrorThrown, ItemSent
- **Recommendation**: Keep current naming for MVP. Consistent -ed/-ed pattern is clear and systematic.

---

## Delegate System

### Delegate (interface)

- **Current**: Delegate
- **Status**: ✅ **No changes proposed** - Clear, established pattern
- **Rationale**: Standard Design Pattern terminology. Implements Init/Close lifecycle.

### Interceptor (interface)

- **Current**: Interceptor
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Observer** - Emphasizes observation role, more FP-like
  2. **Listener** - Event-listener terminology, common in many frameworks
  3. **Handler** - Generic but clear, common in Go
  4. **Watcher** - Similar to Observer, less formal
  5. **Hook** - Common in web frameworks, emphasizes integration points
  6. **EventHandler** - Explicit two-word form
- **Recommendation**: Keep **Interceptor** for MVP. Emphasizes the "interception" of events as they flow through processing.

### Registry

- **Current**: Registry
- **Status**: ✅ **No changes proposed** - Clear and standard
- **Rationale**: Standard pattern name. Clear that it stores registered components.

### Factory[T]

- **Current**: Factory
- **Status**: ✅ **No changes proposed** - Well-established Design Pattern
- **Rationale**: Industry-standard terminology for instance creation.

### Pool[T]

- **Current**: Pool
- **Status**: ✅ **No changes proposed** - Clear and standard
- **Rationale**: Standard terminology for resource pooling.

### Config

- **Current**: Config
- **Status**: ✅ **No changes proposed** - Clear and idiomatic
- **Rationale**: Standard Go terminology for configuration.

---

## Stream Sources

### FromSlice

- **Current**: FromSlice
- **Status**: ✅ **No changes proposed** - Clear and consistent
- **Rationale**: Clear verb-noun pattern. Consistent with FromChannel, FromIter naming.

### FromChannel

- **Current**: FromChannel
- **Status**: ✅ **No changes proposed** - Clear and consistent
- **Rationale**: Part of coherent "From\*" source family.

### FromIter

- **Current**: FromIter
- **Status**: ⚠️ **Could be renamed, but keep for MVP**
- **Alternatives**:
  1. **FromSequence** - More explicit about Go 1.23 iter.Seq, but verbose
  2. **FromSeq** - Shorter form of FromSequence
  3. **FromRange** - More Pythonic, but conflicts with Range() function
- **Recommendation**: Keep **FromIter** for MVP. Go community knows what "iter" means in 1.23+ context.

### Range

- **Current**: Range
- **Status**: ✅ **No changes proposed** - Clear and Python-familiar
- **Rationale**: Generates a range of values. Intuitive naming.

### Generate

- **Current**: Generate
- **Status**: ✅ **No changes proposed** - Clear verb describing action
- **Rationale**: Generates values from seed and generator function.

### Repeat

- **Current**: Repeat
- **Status**: ✅ **No changes proposed** - Clear and direct
- **Rationale**: Repeats a value, unambiguous.

### Interval

- **Current**: Interval
- **Status**: ✅ **No changes proposed** - Clear for timing
- **Rationale**: Standard name for periodic tick streams.

### Empty

- **Current**: Empty
- **Status**: ✅ **No changes proposed** - Clear and concise
- **Rationale**: Communicates that stream emits nothing.

### Once

- **Current**: Once
- **Status**: ✅ **No changes proposed** - Clear and concise
- **Rationale**: Emits single value once. Familiar from Go's sync.Once.

---

## Terminal Operations

### Slice

- **Current**: Slice
- **Status**: ✅ **No changes proposed** - Clear and Go-idiomatic
- **Rationale**: Collects into a Go slice. Industry standard term.

### First

- **Current**: First
- **Status**: ✅ **No changes proposed** - Clear and direct
- **Rationale**: Gets first item. Unambiguous.

### Run

- **Current**: Run
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Drain** - Emphasizes consuming without output, more precise
  2. **Consume** - More descriptive of terminal operation
  3. **Exec** - More imperative, but shorter
  4. **ForEach** - More functional, but suggests action on items (implicit in Run)
  5. **Subscribe** - Rx-like terminology
- **Recommendation**: Keep **Run** for MVP. Short, clear, and Go-idiomatic (like testing's t.Run).

### Collect

- **Current**: Collect
- **Status**: ✅ **No changes proposed** - Clear and standard
- **Rationale**: Collects Results (with errors). Distinct from Slice which extracts values.

### All

- **Current**: All
- **Status**: ✅ **No changes proposed** - Clear for iterator pattern
- **Rationale**: Returns Go 1.23 iter.Seq. Familiar naming from Rust/other languages.

---

## Composition Functions

### Pipe

- **Current**: Pipe
- **Status**: ✅ **No changes proposed** - Industry-standard pipe terminology
- **Rationale**: Classic Unix pipe metaphor. Immediately understood.

### Chain

- **Current**: Chain
- **Status**: ✅ **No changes proposed** - Clear and standard
- **Rationale**: Creates a chain of same-type transformers. Clear terminology.

### Through

- **Current**: Through
- **Status**: ⚠️ **Multiple options, keep current for MVP**
- **Alternatives**:
  1. **Compose** - More FP-like, emphasizes composition
  2. **Sequence** - More explicit about sequential application
  3. **Then** - More fluent, but less clear in static context
  4. **Combine** - Less precise about order
- **Recommendation**: Keep **Through** for MVP. Clear and distinct from Pipe/Chain. "Through T1 then through T2" is intuitive.

### Apply

- **Current**: Apply
- **Status**: ✅ **No changes proposed** - Standard method name
- **Rationale**: Standard method on Transformer interface. Clear verb describing application of transformation.

---

## Error Handling (flow/flowerrors)

### CatchError

- **Current**: CatchError
- **Status**: ✅ **No changes proposed** - Clear and standard
- **Rationale**: "Catch" is familiar from try-catch patterns. Clear intent.

### FilterErrors

- **Current**: FilterErrors
- **Status**: ✅ **No changes proposed** - Clear and consistent
- **Rationale**: Part of coherent error filtering family. Verb-noun structure.

### IgnoreErrors

- **Current**: IgnoreErrors
- **Status**: ✅ **No changes proposed** - Clear and direct
- **Rationale**: Filters out errors. Clear intent.

### MapErrors

- **Current**: MapErrors
- **Status**: ✅ **No changes proposed** - Consistent with Map pattern
- **Rationale**: Transforms errors similar to how Map transforms values.

### WrapError

- **Current**: WrapError
- **Status**: ✅ **No changes proposed** - Clear and Go-idiomatic
- **Rationale**: Uses Go's standard error wrapping semantics.

### ErrorsOnly

- **Current**: ErrorsOnly
- **Status**: ⚠️ **Could be renamed, but keep for MVP**
- **Alternatives**:
  1. **FilterToErrors** - More explicit
  2. **ExtractErrors** - Emphasizes extraction
  3. **OnlyErrors** - Similar structure to current
- **Recommendation**: Keep **ErrorsOnly** for MVP. Clear and concise.

### ThrowOnError

- **Current**: ThrowOnError
- **Status**: ⚠️ **Could be renamed, but keep for MVP**
- **Alternatives**:
  1. **FailOnError** - More Go-idiomatic (using "error" not "exception")
  2. **StopOnError** - More descriptive of behavior
  3. **AbortOnError** - Similar intent
  4. **TerminateOnError** - More formal
- **Recommendation**: Keep **ThrowOnError** for MVP. "Throw" emphasizes the propagation effect even if not technically throwing exceptions.

---

## Observability (flow/observe)

### OnValue

- **Current**: OnValue
- **Status**: ✅ **No changes proposed** - Standard callback naming
- **Rationale**: Clear on*/Do* pattern. Familiar from event listeners.

### OnError

- **Current**: OnError
- **Status**: ✅ **No changes proposed** - Standard callback naming
- **Rationale**: Clear parallel to OnValue. Standard pattern.

### OnStart / OnComplete

- **Current**: OnStart, OnComplete
- **Status**: ✅ **No changes proposed** - Consistent with On\* pattern
- **Rationale**: Part of coherent lifecycle callback family.

### WithMetrics / WithLiveMetrics

- **Current**: WithMetrics, WithLiveMetrics
- **Status**: ✅ **No changes proposed** - Clear "With\*" pattern
- **Rationale**: Builder pattern naming. Clear intent. "With" adds capability.

### WithCounter / WithValueCounter

- **Current**: WithCounter, WithValueCounter
- **Status**: ✅ **No changes proposed** - Clear "With\*" pattern
- **Rationale**: Consistent builder pattern. Clear what's being counted.

### WithErrorCollector

- **Current**: WithErrorCollector
- **Status**: ✅ **No changes proposed** - Clear "With\*" pattern
- **Rationale**: Consistent with other With\* functions.

### WithLogging

- **Current**: WithLogging
- **Status**: ✅ **No changes proposed** - Clear "With\*" pattern
- **Rationale**: Consistent naming. Clear intent.

---

## Summary Table (MVP Symbols)

| Symbol                                                         | Category      | Status      | Recommendation     |
| -------------------------------------------------------------- | ------------- | ----------- | ------------------ |
| **Stream[T]**                                                  | Interface     | ✅ Keep     | No change needed   |
| **Result[T]**                                                  | Type          | ✅ Keep     | No change needed   |
| **Transformer[IN, OUT]**                                       | Interface     | ⚠️ Optional | Keep for MVP       |
| **Mapper[IN, OUT]**                                            | Function Type | ✅ Keep     | No change needed   |
| **FlatMapper[IN, OUT]**                                        | Function Type | ✅ Keep     | No change needed   |
| **Emitter[T]**                                                 | Function Type | ⚠️ Optional | Keep for MVP       |
| **Transmitter[IN, OUT]**                                       | Function Type | ⚠️ Optional | Keep for MVP       |
| **Sink[IN, OUT]**                                              | Function Type | ✅ Keep     | No change needed   |
| **StreamStart / StreamEnd**                                    | Events        | ⚠️ Optional | Keep for MVP       |
| **ItemReceived / ValueReceived / ErrorOccurred / ItemEmitted** | Events        | ✅ Keep     | Consistent pattern |
| **Delegate**                                                   | Interface     | ✅ Keep     | No change needed   |
| **Interceptor**                                                | Interface     | ⚠️ Optional | Keep for MVP       |
| **Registry**                                                   | Type          | ✅ Keep     | No change needed   |
| **Factory[T]**                                                 | Interface     | ✅ Keep     | No change needed   |
| **Pool[T]**                                                    | Interface     | ✅ Keep     | No change needed   |
| **Config**                                                     | Interface     | ✅ Keep     | No change needed   |
| **FromSlice**                                                  | Function      | ✅ Keep     | No change needed   |
| **FromChannel**                                                | Function      | ✅ Keep     | No change needed   |
| **FromIter**                                                   | Function      | ⚠️ Optional | Keep for MVP       |
| **Range**                                                      | Function      | ✅ Keep     | No change needed   |
| **Generate**                                                   | Function      | ✅ Keep     | No change needed   |
| **Repeat**                                                     | Function      | ✅ Keep     | No change needed   |
| **Interval**                                                   | Function      | ✅ Keep     | No change needed   |
| **Empty**                                                      | Function      | ✅ Keep     | No change needed   |
| **Once**                                                       | Function      | ✅ Keep     | No change needed   |
| **Slice**                                                      | Function      | ✅ Keep     | No change needed   |
| **First**                                                      | Function      | ✅ Keep     | No change needed   |
| **Run**                                                        | Function      | ⚠️ Optional | Keep for MVP       |
| **Collect**                                                    | Function      | ✅ Keep     | No change needed   |
| **All**                                                        | Function      | ✅ Keep     | No change needed   |
| **Pipe**                                                       | Function      | ✅ Keep     | No change needed   |
| **Chain**                                                      | Function      | ✅ Keep     | No change needed   |
| **Through**                                                    | Function      | ⚠️ Optional | Keep for MVP       |
| **Apply**                                                      | Method        | ✅ Keep     | No change needed   |
| **CatchError**                                                 | Function      | ✅ Keep     | No change needed   |
| **FilterErrors**                                               | Function      | ✅ Keep     | No change needed   |
| **IgnoreErrors**                                               | Function      | ✅ Keep     | No change needed   |
| **MapErrors**                                                  | Function      | ✅ Keep     | No change needed   |
| **WrapError**                                                  | Function      | ✅ Keep     | No change needed   |
| **ErrorsOnly**                                                 | Function      | ⚠️ Optional | Keep for MVP       |
| **ThrowOnError**                                               | Function      | ⚠️ Optional | Keep for MVP       |
| **OnValue**                                                    | Function      | ✅ Keep     | No change needed   |
| **OnError**                                                    | Function      | ✅ Keep     | No change needed   |
| **OnStart / OnComplete**                                       | Function      | ✅ Keep     | No change needed   |
| **WithMetrics**                                                | Function      | ✅ Keep     | No change needed   |
| **WithLiveMetrics**                                            | Function      | ✅ Keep     | No change needed   |
| **WithCounter**                                                | Function      | ✅ Keep     | No change needed   |
| **WithValueCounter**                                           | Function      | ✅ Keep     | No change needed   |
| **WithErrorCollector**                                         | Function      | ✅ Keep     | No change needed   |
| **WithLogging**                                                | Function      | ✅ Keep     | No change needed   |

---

## Naming Patterns (MVP)

### Established Patterns ✅

1. **From\* Sources**: FromSlice, FromChannel, FromIter
2. **With\* Builders**: WithMetrics, WithCounter, WithLogging
3. **On\* Callbacks**: OnValue, OnError, OnStart, OnComplete
4. **Function types as -er nouns**: Mapper, FlatMapper, Emitter, Transmitter
5. **Terminal operations as verbs**: Slice, First, Run, Collect
6. **Composition as verbs**: Pipe, Chain, Through, Apply
7. **Error handling as verb+noun**: CatchError, FilterErrors, MapErrors, WrapError
8. **Helper constructors match type name**: Ok, Err, Sentinel, Map, FlatMap, Emit, Transmit

### Consistency Checks ✅

- ✅ Stream sources use "From" prefix consistently
- ✅ Registration helpers use "With" prefix consistently
- ✅ Callbacks use "On" prefix consistently
- ✅ Function types use "-er" noun suffix consistently
- ✅ Event names use past tense/past participle consistently
- ✅ No conflicting names in MVP

---

## Recommendations for MVP

**Keep all current MVP naming as-is.** The existing names are:

- Clear and unambiguous
- Consistent within established patterns
- Well-documented in code
- Not causing user confusion

Optional future revisits (for later phases):

- **Transformer** → **Operator** (more Rx-like, when expanding Phase 2+)
- **Emitter** → **Source** (more descriptive, clearer naming)
- **Interceptor** → **Observer** (more FP-like terminology)
- **Run** → **Drain** (more precise about terminal operation)
- **Through** → **Compose** (clearer about composition)

However, these changes should only be made after gather user feedback, as renaming would require migration guides.
