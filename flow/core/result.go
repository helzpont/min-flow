package core

// Result represents the outcome of processing an item in the stream.
// It can either hold a successful value of type OUT, or an error.
// Additionally, it can be marked as a sentinel, indicating a special
// condition in the stream processing (e.g., end of stream).
type Result[OUT any] struct {
	value      OUT
	err        error
	isSentinel bool
}

func NewResult[OUT any](value OUT, err error, isSentinel bool) *Result[OUT] {
	return &Result[OUT]{value: value, err: err, isSentinel: isSentinel}
}

func Ok[OUT any](value OUT) *Result[OUT] {
	return &Result[OUT]{value: value, err: nil, isSentinel: false}
}

func Err[OUT any](err error) *Result[OUT] {
	return &Result[OUT]{value: *new(OUT), err: err, isSentinel: false}
}

func Sentinel[OUT any](err error) *Result[OUT] {
	return &Result[OUT]{value: *new(OUT), err: err, isSentinel: true}
}

func (r *Result[OUT]) IsValue() bool {
	return r.err == nil && !r.isSentinel
}

func (r *Result[OUT]) IsSentinel() bool {
	return r.isSentinel
}

func (r *Result[OUT]) IsError() bool {
	return r.err != nil && !r.isSentinel
}

func (r *Result[OUT]) Value() OUT {
	return r.value
}

func (r *Result[OUT]) Error() error {
	if r.isSentinel {
		return nil
	}
	return r.err
}

func (r *Result[OUT]) Sentinel() error {
	if !r.isSentinel {
		return nil
	}
	return r.err
}

func (r *Result[OUT]) Unwrap() (OUT, error) {
	return r.value, r.err
}
