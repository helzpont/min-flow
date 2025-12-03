package core

import (
	"errors"
	"fmt"
)

// ErrPanic wraps a recovered panic value as an error.
// This is used when a user-provided function panics during stream processing.
type ErrPanic struct {
	Value any
}

func (e ErrPanic) Error() string {
	return fmt.Sprintf("panic: %v", e.Value)
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
