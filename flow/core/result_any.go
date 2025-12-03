package core

// AnyResult is an experimental Result implementation that stores values as `any`.
// This is a proof-of-concept to measure whether avoiding type-specific struct
// instantiation reduces allocation overhead.
//
// The hypothesis is that storing the value as `any` allows zero-copy type
// assertions when retrieving the value, potentially reducing memory pressure
// compared to creating new Result[T] structs for each transformed value.
//
// Trade-offs:
//   - Pro: May reduce allocations by reusing the same container type
//   - Pro: Type assertion is very fast in Go
//   - Con: Loses compile-time type safety (runtime assertion needed)
//   - Con: Interface boxing may have its own overhead for small types
//   - Con: Less idiomatic Go - users expect generics
type AnyResult struct {
	value      any
	err        error
	isSentinel bool
}

// AnyOk creates a successful AnyResult containing the given value.
func AnyOk(value any) AnyResult {
	return AnyResult{value: value, err: nil, isSentinel: false}
}

// AnyErr creates an error AnyResult.
func AnyErr(err error) AnyResult {
	return AnyResult{value: nil, err: err, isSentinel: false}
}

// AnySentinel creates a sentinel AnyResult.
func AnySentinel(err error) AnyResult {
	return AnyResult{value: nil, err: err, isSentinel: true}
}

// IsValue returns true if this AnyResult contains a successful value.
func (r AnyResult) IsValue() bool {
	return r.err == nil && !r.isSentinel
}

// IsSentinel returns true if this AnyResult is a sentinel.
func (r AnyResult) IsSentinel() bool {
	return r.isSentinel
}

// IsError returns true if this AnyResult contains an error.
func (r AnyResult) IsError() bool {
	return r.err != nil && !r.isSentinel
}

// Value returns the contained value. Caller must type-assert to the expected type.
func (r AnyResult) Value() any {
	return r.value
}

// ValueAs performs a type assertion and returns the value as type T.
// Returns the zero value if the assertion fails or if this is not a value result.
func ValueAs[T any](r AnyResult) T {
	if v, ok := r.value.(T); ok {
		return v
	}
	var zero T
	return zero
}

// ValueAsOk performs a type assertion and returns the value with success indicator.
func ValueAsOk[T any](r AnyResult) (T, bool) {
	v, ok := r.value.(T)
	return v, ok
}

// Error returns the error if this is an error AnyResult.
func (r AnyResult) Error() error {
	if r.isSentinel {
		return nil
	}
	return r.err
}

// Sentinel returns the sentinel's context error.
func (r AnyResult) Sentinel() error {
	if !r.isSentinel {
		return nil
	}
	return r.err
}

// ToResult converts an AnyResult to a typed Result[T].
// This performs a type assertion on the value.
func ToResult[T any](r AnyResult) Result[T] {
	if r.isSentinel {
		return Sentinel[T](r.err)
	}
	if r.err != nil {
		return Err[T](r.err)
	}
	if v, ok := r.value.(T); ok {
		return Ok(v)
	}
	var zero T
	return Ok(zero)
}

// FromResult converts a typed Result[T] to an AnyResult.
func FromResult[T any](r Result[T]) AnyResult {
	if r.IsSentinel() {
		return AnySentinel(r.Sentinel())
	}
	if r.IsError() {
		return AnyErr(r.Error())
	}
	return AnyOk(r.Value())
}
