# Min-Flow – Copilot Instructions

## Project Status: ALPHA - NO USERS

**IMPORTANT: This project is in alpha stage with ZERO users. There are no backward compatibility concerns to consider. Breaking changes are not only acceptable but encouraged when they improve the framework's design, performance, or maintainability.**

## Project Overview

This project is a unified stream processing framework combining the best of exoflow, flow-next, and flowai. It provides a complete solution for building scalable, observable, and resilient data pipelines in Go, from local development to distributed production deployments.

Basically, it encapsulates the data stream processing logic and provides a user-friendly interface so that developers can focus on the business logic of their applications without worrying about the underlying complexities of data flow management.

## Architecture & Core Concepts

The framework is built around a layered abstraction model for stream processing:

### Abstraction Layers (lowest to highest)

1. **Mapper/FlatMapper** (`map.go`) - Lowest level: pure functions that transform individual items

   - `Mapper[IN, OUT]`: 1:1 transformation (one input → one output)
   - `FlatMapper[IN, OUT]`: 1:N transformation (one input → zero or more outputs)
   - Both recover from panics and wrap errors in Results

2. **Emitter/Transmitter** (`channel.go`) - Channel-level abstraction

   - `Emitter[OUT]`: produces a channel of Results (answers: "How is data produced?")
   - `Transmitter[IN, OUT]`: transforms one channel to another (answers: "How is data transformed?")

3. **Stream/Transformer** (`stream.go`) - Highest level: composable pipeline components
   - `Stream[OUT]`: flow of data with `Emit()`, `Collect()`, `All()` methods
   - `Transformer[IN, OUT]`: transforms one Stream to another via `Apply()`
   - `Streamer[OUT]`: function type implementing Stream interface

### Result Type (`result.go`)

All data flows through `Result[OUT]` which encapsulates:

- **Value**: successful processing result
- **Error**: processing failure (non-fatal, continues stream)
- **Sentinel**: special condition marker (e.g., end of stream)

Constructors: `Ok(value)`, `Err(err)`, `Sentinel(err)`, `NewResult(value, err, isSentinel)`

### Delegate System (`delegate.go`)

Extensibility through registered components:

- `Delegate`: base interface with `Init()` and `Close()`
- `Interceptor`: event handling (StreamStart, StreamEnd, etc.)
- `Factory[T]`: instance creation
- `Pool[T]`: resource pooling
- `Config`: validated configuration
- `Registry`: thread-safe delegate storage with context propagation

### Terminal Operations (`terminal.go`)

Stream consumers that produce final results:

- `Slice()`: collect all values into a slice
- `First()`: get the first value
- `Run()`: execute for side effects only

## Package Structure

```
flow/
  core/           # Core abstractions (NO external dependencies, stdlib only)
    channel.go    # Emitter, Transmitter
    delegate.go   # Delegate, Interceptor, Factory, Pool, Config, Registry
    map.go        # Mapper, FlatMapper
    result.go     # Result type
    stream.go     # Stream, Transformer, Streamer
    terminal.go   # Terminal operations (Slice, First, Run)
```

## Design Priorities

If there is a question of what we should do or how we should do it, here are the priorities in this project:

0. **Viability**: ensuring the framework is robust and reliable for real-world data processing needs. Users must trust that the framework does what it promises without data loss or corruption.
1. **Developer Experience**: providing clear documentation, intuitive APIs, and helpful error messages to make it easy for developers to use and integrate the framework into their applications. This includes progressive examples and tutorials to onboard new users effectively.
2. **Go Ecosystem Alignment**: adhering to Go best practices and conventions to ensure that the framework feels familiar to Go developers. This includes using idiomatic Go patterns, leveraging standard libraries, and maintaining compatibility with popular Go tools and frameworks. We also want to utilize any relevant Go concurrency features to optimize performance.
3. **Extension**: designing the framework to be modular and extensible, allowing users to easily add new features, integrations, and transformations as needed. This involves creating clear interfaces and abstractions to facilitate customization and extension. From the core flow package, we'll build out additional modules and plugins that can be integrated seamlessly. We will eat our own dog food by using the framework to build the framework itself and real-world data processing applications, which will help identify areas for improvement and ensure that the framework meets practical needs.
4. **Efficiency**: optimizing the framework for high throughput and low latency to handle large volumes of data streams effectively. This involves implementing efficient algorithms and data structures, as well as minimizing resource consumption.
5. **Performance**: ensuring that the framework can scale horizontally and vertically to accommodate growing data processing demands. This includes implementing load balancing, resource management, and performance tuning techniques.

## Code Patterns & Conventions

### Generic Type Parameters

- Use descriptive names: `IN`, `OUT` for stream data types; `T` for general types
- Keep type constraints minimal but explicit

### Error Handling

- Wrap processing errors in `Result` using `Err[T](err)` - stream continues
- Use `defer recover()` in user-provided functions (Mapper, FlatMapper)
- Return errors from framework functions, don't panic
- Panics ONLY for user misuse (e.g., nil function, invalid type assertion)

### Context Usage

- All stream operations accept `context.Context` as first parameter
- Use `context.WithCancel` for early termination
- Propagate delegates via context using `WithRegistry()`

### Concurrency

- Channels are the primary concurrency primitive
- Always close output channels when done
- Respect context cancellation in long-running operations
- Use `sync.RWMutex` for thread-safe access to shared state

### Naming

- Constructors: `New<Type>()` or short forms like `Ok()`, `Err()`
- Interface methods: verb phrases (`Emit`, `Apply`, `Collect`)
- Function types: noun + "er" suffix (`Streamer`, `Mapper`, `Emitter`)

## Test Guidelines

- All tests must be written in a table-driven style where possible.
- There should be one test file per code file, mirroring the structure (e.g., tests for `stream.go` should be in `stream_test.go`).
- Tests should cover both typical and edge cases.
- Use timeouts when testing asynchronous code to prevent hanging tests.
- Ensure all tests pass before committing code changes.
- Test context cancellation behavior explicitly.
- Test panic recovery in user-provided functions.

## Development Instructions

### Breaking Changes Policy

Since this is alpha with no users, prioritize the best possible design over backward compatibility. Make breaking changes freely when they improve robustness, performance, or maintainability.

### Before Committing

- Ensure all tests pass (`go test ./...`)
- Code coverage is adequate
- Linting checks are clear (`go vet`, `staticcheck`)
- Documentation is updated
- Use descriptive commit messages

### Code Standards

- Adhere to Go coding standards and best practices
- Avoid old or deprecated Go idioms
- Min-Flow is very opinionated: default to the most robust, maintainable, and idiomatic approach even if it requires more code or complexity
- Ensure code is well-documented and maintainable

### Error Philosophy

- Panics are allowed for user-induced errors (e.g., invalid type assertions) but NOT for internal framework errors or data processing issues
- Fail fast when the user misuses the framework
- Handle data processing errors gracefully within the framework using Result
- Always provide meaningful error messages

### Core Package Constraints

The `flow/core` package must have NO dependencies outside the standard library, including other flow packages. This ensures the core abstractions remain portable and dependency-free.
