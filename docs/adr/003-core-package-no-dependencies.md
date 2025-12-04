# ADR-003: Core Package Has No External Dependencies

## Status

Accepted

## Context

The `flow/core` package defines the foundational abstractions: `Result`, `Stream`, `Transformer`, `Emitter`, `Transmitter`, `Mapper`, `FlatMapper`, `Sink`, and the delegate system.

Dependencies in core packages create several problems:

- **Transitive dependencies** - anything importing core inherits all its dependencies
- **Version conflicts** - different packages may need different versions
- **Build times** - more dependencies mean slower compilation
- **Security surface** - each dependency is a potential vulnerability
- **Portability** - dependencies may not work on all platforms (WASM, embedded, etc.)

We needed to decide what dependencies, if any, the core package should allow.

## Decision

The `flow/core` package has **zero external dependencies** - it only uses the Go standard library.

This is enforced by:

1. Code review policy
2. The package documentation header explicitly states this constraint
3. Import analysis in CI (could be added)

```go
// package core defines the core abstractions for data flow processing.
// NOTE: this package should have no dependencies outside the standard
// library, including other flow packages.
package core
```

## Consequences

### Positive

1. **Minimal footprint** - core adds no transitive dependencies to user projects

2. **Maximum portability** - works anywhere Go works: WASM, TinyGo, embedded systems

3. **Fast compilation** - no dependency resolution or compilation of external code

4. **Stability** - no external breaking changes can affect core

5. **Security** - smaller attack surface, fewer CVEs to track

6. **Clear boundary** - forces good separation between core abstractions and feature packages

### Negative

1. **Feature limitations** - can't use helpful libraries (e.g., `golang.org/x/sync/errgroup`)

2. **Reimplementation** - may need to reimplement utilities that exist in external packages

3. **Testing constraints** - can't use external test utilities like `testify`

### Mitigations

1. **Subpackages can have dependencies** - `flow/flowerrors`, `flow/parallel`, etc. can import external packages

2. **Standard library is rich** - `context`, `sync`, `iter`, `errors` provide most of what we need

3. **Table-driven tests** - Go's built-in testing is sufficient with good patterns

### What This Means in Practice

```go
// ✅ Allowed in flow/core
import (
    "context"
    "errors"
    "fmt"
    "iter"
    "sync"
)

// ❌ Not allowed in flow/core
import (
    "golang.org/x/sync/errgroup"           // external
    "github.com/lguimbarda/min-flow/flow/filter"  // other flow package
)
```
