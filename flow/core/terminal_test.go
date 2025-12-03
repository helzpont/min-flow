package core

import (
	"context"
	"errors"
	"testing"
)

func TestSlice(t *testing.T) {
	tests := []struct {
		name       string
		stream     Stream[int]
		wantValues []int
		wantErr    bool
	}{
		{
			name: "collects all values",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Ok(2)
				out <- Ok(3)
				close(out)
				return out
			}),
			wantValues: []int{1, 2, 3},
			wantErr:    false,
		},
		{
			name: "empty stream",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int])
				close(out)
				return out
			}),
			wantValues: nil,
			wantErr:    false,
		},
		{
			name: "stops on error",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Err[int](errors.New("test error"))
				out <- Ok(3) // Should not be collected
				close(out)
				return out
			}),
			wantValues: nil,
			wantErr:    true,
		},
		{
			name: "includes sentinel values (zero value)",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Ok(2)
				out <- EndOfStream[int]() // sentinel has zero value
				close(out)
				return out
			}),
			wantValues: []int{1, 2, 0}, // sentinel's zero value is included
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			values, err := Slice(ctx, tt.stream)

			if (err != nil) != tt.wantErr {
				t.Errorf("Slice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(values) != len(tt.wantValues) {
					t.Errorf("Slice() got %d values, want %d", len(values), len(tt.wantValues))
					return
				}
				for i, v := range values {
					if v != tt.wantValues[i] {
						t.Errorf("Slice()[%d] = %v, want %v", i, v, tt.wantValues[i])
					}
				}
			}
		})
	}
}

func TestFirst(t *testing.T) {
	tests := []struct {
		name      string
		stream    Stream[int]
		wantValue int
		wantErr   bool
	}{
		{
			name: "returns first value",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(42)
				out <- Ok(2)
				out <- Ok(3)
				close(out)
				return out
			}),
			wantValue: 42,
			wantErr:   false,
		},
		{
			name: "empty stream returns error",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int])
				close(out)
				return out
			}),
			wantValue: 0,
			wantErr:   true,
		},
		{
			name: "error result returns error",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 1)
				out <- Err[int](errors.New("first is error"))
				close(out)
				return out
			}),
			wantValue: 0,
			wantErr:   true,
		},
		{
			name: "sentinel first returns error",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 1)
				out <- EndOfStream[int]()
				close(out)
				return out
			}),
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			value, err := First(ctx, tt.stream)

			if (err != nil) != tt.wantErr {
				t.Errorf("First() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && value != tt.wantValue {
				t.Errorf("First() = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		stream  Stream[int]
		wantErr bool
	}{
		{
			name: "runs all values",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Ok(2)
				out <- Ok(3)
				close(out)
				return out
			}),
			wantErr: false,
		},
		{
			name: "empty stream succeeds",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int])
				close(out)
				return out
			}),
			wantErr: false,
		},
		{
			name: "stops on error",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 3)
				out <- Ok(1)
				out <- Err[int](errors.New("run error"))
				out <- Ok(3)
				close(out)
				return out
			}),
			wantErr: true,
		},
		{
			name: "ignores sentinels",
			stream: Emit(func(ctx context.Context) <-chan Result[int] {
				out := make(chan Result[int], 2)
				out <- Ok(1)
				out <- Sentinel[int](errors.New("marker"))
				close(out)
				return out
			}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := Run(ctx, tt.stream)

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSlice_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- Ok(i):
				}
			}
		}()
		return out
	})

	values, err := Slice(ctx, stream)
	// Should terminate early due to context cancellation
	if err != nil {
		t.Errorf("Slice() with cancelled context returned error: %v", err)
	}
	if len(values) >= 100 {
		t.Errorf("Slice() should have terminated early, got %d values", len(values))
	}
}
