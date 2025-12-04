# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the min-flow project.

## What are ADRs?

ADRs document significant architectural decisions made during the development of this project. Each record captures the context, decision, and consequences of a particular choice.

## Index

| ADR                                               | Title                                     | Status   |
| ------------------------------------------------- | ----------------------------------------- | -------- |
| [001](001-result-type-for-stream-items.md)        | Result Type for Stream Items              | Accepted |
| [002](002-function-types-implement-interfaces.md) | Function Types Implement Interfaces       | Accepted |
| [003](003-core-package-no-dependencies.md)        | Core Package Has No External Dependencies | Accepted |
| [004](004-context-first-parameter.md)             | Context as First Parameter                | Accepted |
| [005](005-sink-type-for-terminals.md)             | Sink Type for Terminal Operations         | Accepted |
| [006](006-panic-recovery-in-user-functions.md)    | Panic Recovery in User Functions          | Accepted |
| [007](007-buffered-channels-by-default.md)        | Buffered Channels by Default              | Accepted |

## Template

When adding a new ADR, use this template:

```markdown
# ADR-NNN: Title

## Status

Proposed | Accepted | Deprecated | Superseded by ADR-XXX

## Context

What is the issue that we're seeing that is motivating this decision?

## Decision

What is the change that we're proposing and/or doing?

## Consequences

What becomes easier or more difficult to do because of this change?
```
