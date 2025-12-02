# Contributing to min-flow

Thank you for your interest in contributing to min-flow! This document provides guidelines and information for contributors.

## Project Status

This project is in **ALPHA** stage with no users yet. This means:

- Breaking changes are welcomed when they improve the design
- No backward compatibility concerns
- Focus on getting the architecture right

## Development Setup

1. **Clone the repository**

   ```bash
   git clone https://github.com/lguimbarda/min-flow.git
   cd min-flow
   ```

2. **Ensure you have Go 1.23+**

   ```bash
   go version
   ```

3. **Run tests**

   ```bash
   go test ./...
   ```

4. **Run linting**
   ```bash
   go vet ./...
   ```

## Code Standards

### Architecture

The framework is built around layered abstractions:

1. **Mapper/FlatMapper** - Pure functions for 1:1 and 1:N transformations
2. **Emitter/Transmitter** - Channel-level abstractions
3. **Stream/Transformer** - High-level composable pipeline components

### Naming Conventions

- Constructors: `New<Type>()` or short forms like `Ok()`, `Err()`
- Interface methods: verb phrases (`Emit`, `Apply`, `Collect`)
- Function types: noun + "er" suffix (`Streamer`, `Mapper`, `Emitter`)

### Error Handling

- Use `Result[T]` for stream data errors (non-fatal, stream continues)
- Use `defer recover()` in user-provided functions
- Return errors from framework functions, don't panic
- Panics ONLY for user misuse (e.g., nil function, invalid type assertion)

### Context Usage

- All stream operations accept `context.Context` as first parameter
- Use `context.WithCancel` for early termination
- Always respect context cancellation

### Concurrency

- Channels are the primary concurrency primitive
- Always close output channels when done
- Use `sync.RWMutex` for thread-safe shared state

## Testing Guidelines

- All tests must use table-driven style where possible
- One test file per source file (e.g., `stream_test.go` for `stream.go`)
- Cover both typical and edge cases
- Use timeouts in async tests to prevent hanging
- Test context cancellation explicitly
- Test panic recovery in user-provided functions

Example test structure:

```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name     string
        input    []int
        expected []int
    }{
        {
            name:     "basic case",
            input:    []int{1, 2, 3},
            expected: []int{2, 4, 6},
        },
        {
            name:     "empty input",
            input:    []int{},
            expected: []int{},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx, cancel := context.WithTimeout(context.Background(), time.Second)
            defer cancel()

            // Test implementation
        })
    }
}
```

## Core Package Constraints

The `flow/core` package must have **NO dependencies** outside the standard library. This ensures the core abstractions remain portable and dependency-free.

## Pull Request Process

1. **Create a feature branch**

   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make your changes** following the code standards above

3. **Ensure all tests pass**

   ```bash
   go test ./... -v
   ```

4. **Run linting**

   ```bash
   go vet ./...
   ```

5. **Submit a pull request** with a clear description of the changes

## Design Priorities

When making design decisions, consider these priorities in order:

0. **Viability** - Framework must be robust and reliable
1. **Developer Experience** - Clear APIs, helpful errors, good docs
2. **Go Ecosystem Alignment** - Idiomatic Go patterns
3. **Extension** - Modular and extensible design
4. **Efficiency** - High throughput, low latency
5. **Performance** - Scalable horizontally and vertically

## Questions?

Feel free to open an issue for:

- Bug reports
- Feature requests
- Questions about the codebase
- Design discussions

Thank you for contributing!
