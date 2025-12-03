package core

import (
	"context"
	"errors"
	"testing"
)

func TestCollect(t *testing.T) {
	tests := []struct {
		name       string
		stream     Stream[int]
		wantLen    int
		wantValues []int
		wantErrors int
	}{
		{
			name: "collects all results",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Ok(2)
				out <- Ok(3)
				close(out)
				return out
			}),
			wantLen:    3,
			wantValues: []int{1, 2, 3},
			wantErrors: 0,
		},
		{
			name: "collects mixed results",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Err[int](errors.New("err"))
				out <- Ok(3)
				close(out)
				return out
			}),
			wantLen:    3,
			wantValues: []int{1, 3},
			wantErrors: 1,
		},
		{
			name: "empty stream",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int])
				close(out)
				return out
			}),
			wantLen:    0,
			wantValues: nil,
			wantErrors: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := Collect(ctx, tt.stream)

			if len(results) != tt.wantLen {
				t.Errorf("Collect() got %d results, want %d", len(results), tt.wantLen)
			}

			var values []int
			var errorCount int
			for _, r := range results {
				if r.IsValue() {
					values = append(values, r.Value())
				} else if r.IsError() {
					errorCount++
				}
			}

			if errorCount != tt.wantErrors {
				t.Errorf("Collect() got %d errors, want %d", errorCount, tt.wantErrors)
			}

			for i, v := range values {
				if i < len(tt.wantValues) && v != tt.wantValues[i] {
					t.Errorf("values[%d] = %d, want %d", i, v, tt.wantValues[i])
				}
			}
		})
	}
}

func TestAll(t *testing.T) {
	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 5)
		for i := 1; i <= 5; i++ {
			out <- Ok(i)
		}
		close(out)
		return out
	})

	ctx := context.Background()
	var values []int
	for res := range All(ctx, stream) {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	if len(values) != 5 {
		t.Errorf("All() yielded %d values, want 5", len(values))
	}

	for i, v := range values {
		if v != i+1 {
			t.Errorf("values[%d] = %d, want %d", i, v, i+1)
		}
	}
}

func TestAll_EarlyTermination(t *testing.T) {
	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 100)
		for i := 0; i < 100; i++ {
			out <- Ok(i)
		}
		close(out)
		return out
	})

	ctx := context.Background()
	var count int
	for res := range All(ctx, stream) {
		if res.IsValue() {
			count++
			if count >= 5 {
				break
			}
		}
	}

	if count != 5 {
		t.Errorf("All() early termination got %d values, want 5", count)
	}
}

func TestAll_WithErrors(t *testing.T) {
	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 3)
		out <- Ok(1)
		out <- Err[int](errors.New("test error"))
		out <- Ok(3)
		close(out)
		return out
	})

	ctx := context.Background()
	var values []int
	var errors int
	for res := range All(ctx, stream) {
		if res.IsValue() {
			values = append(values, res.Value())
		} else if res.IsError() {
			errors++
		}
	}

	if len(values) != 2 {
		t.Errorf("All() got %d values, want 2", len(values))
	}
	if errors != 1 {
		t.Errorf("All() got %d errors, want 1", errors)
	}
}
