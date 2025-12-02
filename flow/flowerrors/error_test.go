package flowerrors_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
)

func TestOnError(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error2"))
		}()
		return out
	})
	stream := emitter

	var capturedErrors []string
	handler := func(err error) {
		capturedErrors = append(capturedErrors, err.Error())
	}

	result := flowerrors.OnError[int](handler).Apply(ctx, stream)

	// Consume the stream
	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	// Check values passed through
	if len(values) != 2 {
		t.Errorf("got %d values, want 2", len(values))
	}

	// Check errors passed through
	if errCount != 2 {
		t.Errorf("got %d errors, want 2", errCount)
	}

	// Check handler was called
	if len(capturedErrors) != 2 {
		t.Fatalf("got %d captured errors, want 2", len(capturedErrors))
	}
	if capturedErrors[0] != "error1" || capturedErrors[1] != "error2" {
		t.Errorf("captured errors = %v, want [error1, error2]", capturedErrors)
	}
}

func TestCatchError(t *testing.T) {
	ctx := context.Background()

	errRecoverable := errors.New("recoverable")
	errFatal := errors.New("fatal")

	tests := []struct {
		name       string
		errors     []error
		predicate  func(error) bool
		handler    func(error) (int, error)
		wantValues []int
		wantErrors int
	}{
		{
			name:   "catch matching errors",
			errors: []error{errRecoverable, errRecoverable},
			predicate: func(err error) bool {
				return errors.Is(err, errRecoverable)
			},
			handler: func(err error) (int, error) {
				return 999, nil
			},
			wantValues: []int{999, 999},
			wantErrors: 0,
		},
		{
			name:   "ignore non-matching errors",
			errors: []error{errFatal},
			predicate: func(err error) bool {
				return errors.Is(err, errRecoverable)
			},
			handler: func(err error) (int, error) {
				return 999, nil
			},
			wantValues: []int{},
			wantErrors: 1,
		},
		{
			name:   "handler returns error",
			errors: []error{errRecoverable},
			predicate: func(err error) bool {
				return true
			},
			handler: func(err error) (int, error) {
				return 0, errors.New("handler error")
			},
			wantValues: []int{},
			wantErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
				out := make(chan *core.Result[int])
				go func() {
					defer close(out)
					for _, err := range tt.errors {
						out <- core.Err[int](err)
					}
				}()
				return out
			})
			stream := emitter

			result := flowerrors.CatchError(tt.predicate, tt.handler).Apply(ctx, stream)

			var values []int
			var errCount int
			for r := range result.Emit(ctx) {
				if r.IsValue() {
					values = append(values, r.Value())
				} else if r.IsError() {
					errCount++
				}
			}

			if len(values) != len(tt.wantValues) {
				t.Fatalf("got %d values, want %d", len(values), len(tt.wantValues))
			}
			for i := range values {
				if values[i] != tt.wantValues[i] {
					t.Errorf("values[%d] = %d, want %d", i, values[i], tt.wantValues[i])
				}
			}
			if errCount != tt.wantErrors {
				t.Errorf("got %d errors, want %d", errCount, tt.wantErrors)
			}
		})
	}
}

func TestFilterErrors(t *testing.T) {
	ctx := context.Background()

	errToFilter := errors.New("filter me")
	errToKeep := errors.New("keep me")

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errToFilter)
			out <- core.Ok(2)
			out <- core.Err[int](errToKeep)
			out <- core.Ok(3)
		}()
		return out
	})
	stream := emitter

	predicate := func(err error) bool {
		return errors.Is(err, errToFilter)
	}

	result := flowerrors.FilterErrors[int](predicate).Apply(ctx, stream)

	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	// All values should pass through
	wantValues := []int{1, 2, 3}
	if len(values) != len(wantValues) {
		t.Fatalf("got %d values, want %d", len(values), len(wantValues))
	}
	for i := range values {
		if values[i] != wantValues[i] {
			t.Errorf("values[%d] = %d, want %d", i, values[i], wantValues[i])
		}
	}

	// Only errToKeep should pass through
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestIgnoreErrors(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error2"))
			out <- core.Ok(3)
		}()
		return out
	})
	stream := emitter

	result := flowerrors.IgnoreErrors[int]().Apply(ctx, stream)
	got, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMapErrors(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("original"))
			out <- core.Ok(2)
		}()
		return out
	})
	stream := emitter

	mapper := func(err error) error {
		return fmt.Errorf("wrapped: %w", err)
	}

	result := flowerrors.MapErrors[int](mapper).Apply(ctx, stream)

	var values []int
	var errMsgs []string
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errMsgs = append(errMsgs, r.Error().Error())
		}
	}

	if len(values) != 2 {
		t.Errorf("got %d values, want 2", len(values))
	}
	if len(errMsgs) != 1 {
		t.Fatalf("got %d errors, want 1", len(errMsgs))
	}
	if errMsgs[0] != "wrapped: original" {
		t.Errorf("got error %q, want %q", errMsgs[0], "wrapped: original")
	}
}

func TestWrapError(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Err[int](errors.New("inner"))
		}()
		return out
	})
	stream := emitter

	wrapper := func(err error) error {
		return fmt.Errorf("outer: %w", err)
	}

	result := flowerrors.WrapError[int](wrapper).Apply(ctx, stream)

	for r := range result.Emit(ctx) {
		if r.IsError() {
			got := r.Error().Error()
			want := "outer: inner"
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		}
	}
}

func TestErrorsOnly(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error2"))
			out <- core.Sentinel[int](errors.New("sentinel"))
		}()
		return out
	})
	stream := emitter

	result := flowerrors.ErrorsOnly[int]().Apply(ctx, stream)

	var errCount int
	var valueCount int
	var sentinelCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			valueCount++
		} else if r.IsSentinel() {
			sentinelCount++
		} else if r.IsError() {
			errCount++
		}
	}

	if valueCount != 0 {
		t.Errorf("got %d values, want 0", valueCount)
	}
	if sentinelCount != 0 {
		t.Errorf("got %d sentinels, want 0", sentinelCount)
	}
	if errCount != 2 {
		t.Errorf("got %d errors, want 2", errCount)
	}
}

func TestMaterialize(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error"))
			out <- core.Ok(2)
		}()
		return out
	})
	stream := emitter

	result := flowerrors.Materialize[int]().Apply(ctx, stream)

	var materialized []flowerrors.Materialized[int]
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			materialized = append(materialized, r.Value())
		}
	}

	if len(materialized) != 3 {
		t.Fatalf("got %d materialized, want 3", len(materialized))
	}

	// First: value 1
	if !materialized[0].IsValue || materialized[0].Value != 1 {
		t.Errorf("materialized[0] = %+v, want value 1", materialized[0])
	}

	// Second: error
	if materialized[1].IsValue || materialized[1].Err == nil {
		t.Errorf("materialized[1] = %+v, want error", materialized[1])
	}

	// Third: value 2
	if !materialized[2].IsValue || materialized[2].Value != 2 {
		t.Errorf("materialized[2] = %+v, want value 2", materialized[2])
	}
}

func TestDematerialize(t *testing.T) {
	ctx := context.Background()

	// Create a stream of Materialized values
	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[flowerrors.Materialized[int]] {
		out := make(chan *core.Result[flowerrors.Materialized[int]])
		go func() {
			defer close(out)
			out <- core.Ok(flowerrors.Materialized[int]{Value: 1, IsValue: true})
			out <- core.Ok(flowerrors.Materialized[int]{Err: errors.New("error"), IsValue: false})
			out <- core.Ok(flowerrors.Materialized[int]{Value: 2, IsValue: true})
		}()
		return out
	})
	stream := emitter

	result := flowerrors.Dematerialize[int]().Apply(ctx, stream)

	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	if len(values) != 2 {
		t.Fatalf("got %d values, want 2", len(values))
	}
	if values[0] != 1 || values[1] != 2 {
		t.Errorf("values = %v, want [1, 2]", values)
	}
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestCountErrors(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error2"))
			out <- core.Err[int](errors.New("error3"))
			out <- core.Ok(3)
		}()
		return out
	})
	stream := emitter

	var counter int64
	result := flowerrors.CountErrors[int](&counter).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	if counter != 3 {
		t.Errorf("got counter = %d, want 3", counter)
	}
}

func TestThrowOnError(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("stop here"))
			out <- core.Ok(3) // Should not reach this
			out <- core.Ok(4) // Should not reach this
		}()
		return out
	})
	stream := emitter

	result := flowerrors.ThrowOnError[int]().Apply(ctx, stream)

	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	// Should have 2 values before the error
	if len(values) != 2 {
		t.Errorf("got %d values, want 2", len(values))
	}
	if values[0] != 1 || values[1] != 2 {
		t.Errorf("values = %v, want [1, 2]", values)
	}

	// Should have exactly 1 error (the stopping error)
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestErrorContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter := core.Emit(func(ctx context.Context) <-chan *core.Result[int] {
		out := make(chan *core.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(i):
				}
			}
		}()
		return out
	})
	stream := emitter

	var handlerCalls int
	handler := func(err error) {
		handlerCalls++
	}

	result := flowerrors.OnError[int](handler).Apply(ctx, stream)

	// Consume a few items then cancel
	count := 0
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			count++
			if count >= 5 {
				cancel()
			}
		}
	}

	// Should have received around 5 items
	if count < 3 || count > 10 {
		t.Errorf("got %d items, expected around 5", count)
	}
}
