package core

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// ErrPanic wraps a recovered panic value as an error.
// This is used when a user-provided function panics during stream processing.
// It includes a cleaned-up stack trace that excludes internal min-flow frames.
type ErrPanic struct {
	Value any
	Stack string // Cleaned stack trace
}

func (e ErrPanic) Error() string {
	if e.Stack != "" {
		return fmt.Sprintf("panic: %v\n%s", e.Value, e.Stack)
	}
	return fmt.Sprintf("panic: %v", e.Value)
}

// NewPanicError creates an ErrPanic from a recovered value with a cleaned stack trace.
// It captures the current stack and removes internal min-flow frames to show only
// user code, making it easier to identify where the panic originated.
func NewPanicError(recovered any) ErrPanic {
	return ErrPanic{
		Value: recovered,
		Stack: cleanStack(captureStack(4)), // skip: runtime.Callers, captureStack, NewPanicError, defer func
	}
}

// captureStack returns the current stack trace as a string.
func captureStack(skip int) string {
	const maxFrames = 32
	var pcs [maxFrames]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder

	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}

	return sb.String()
}

// cleanStack removes internal min-flow frames from a stack trace.
// It keeps user code and standard library frames while filtering out
// github.com/lguimbarda/min-flow internal frames.
func cleanStack(stack string) string {
	lines := strings.Split(stack, "\n")
	var result []string
	var skipNext bool

	for _, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Check if this is a function line (not a file:line)
		if !strings.HasPrefix(line, "\t") {
			// Skip internal min-flow frames
			if strings.Contains(line, "github.com/lguimbarda/min-flow/flow/") {
				skipNext = true
				continue
			}
			skipNext = false
		} else if skipNext {
			// Skip the file:line that follows a skipped function
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// Result represents the outcome of processing an item in the stream.
// It exists in one of three states:
//   - Value: successful processing result (IsValue() returns true)
//   - Error: processing failure that is non-fatal (IsError() returns true)
//   - Sentinel: stream control signal like end-of-stream (IsSentinel() returns true)
//
// Errors are recoverable and the stream continues processing subsequent items.
// Sentinels are control signals that may carry optional context via an error value.
type Result[OUT any] struct {
	value      OUT
	err        error
	isSentinel bool
}

// NewResult creates a Result with explicit control over all fields.
// Prefer Ok(), Err(), Sentinel(), or EndOfStream() for common cases.
func NewResult[OUT any](value OUT, err error, isSentinel bool) Result[OUT] {
	return Result[OUT]{value: value, err: err, isSentinel: isSentinel}
}

// Ok creates a successful Result containing the given value.
func Ok[OUT any](value OUT) Result[OUT] {
	return Result[OUT]{value: value, err: nil, isSentinel: false}
}

// Err creates an error Result. The stream will continue processing;
// use this for recoverable errors that should be propagated downstream.
func Err[OUT any](err error) Result[OUT] {
	var zero OUT
	return Result[OUT]{value: zero, err: err, isSentinel: false}
}

// Sentinel creates a sentinel Result with an optional descriptive error.
// Sentinels signal stream control conditions (e.g., pagination boundaries,
// batch markers). Use EndOfStream() for the common end-of-stream case.
func Sentinel[OUT any](err error) Result[OUT] {
	var zero OUT
	return Result[OUT]{value: zero, err: err, isSentinel: true}
}

// ErrEndOfStream is the sentinel error indicating normal stream termination.
var ErrEndOfStream = errors.New("end of stream")

// EndOfStream creates a sentinel Result indicating the stream has ended normally.
// This is the canonical way to signal stream completion.
func EndOfStream[OUT any]() Result[OUT] {
	var zero OUT
	return Result[OUT]{value: zero, err: ErrEndOfStream, isSentinel: true}
}

func (r Result[OUT]) IsValue() bool {
	return r.err == nil && !r.isSentinel
}

func (r Result[OUT]) IsSentinel() bool {
	return r.isSentinel
}

func (r Result[OUT]) IsError() bool {
	return r.err != nil && !r.isSentinel
}

func (r Result[OUT]) Value() OUT {
	return r.value
}

func (r Result[OUT]) Error() error {
	if r.isSentinel {
		return nil
	}
	return r.err
}

func (r Result[OUT]) Sentinel() error {
	if !r.isSentinel {
		return nil
	}
	return r.err
}

func (r Result[OUT]) Unwrap() (OUT, error) {
	return r.value, r.err
}
