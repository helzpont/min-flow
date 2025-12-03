package core

import (
	"context"
	"testing"
)

func TestTransformConfig(t *testing.T) {
	tests := []struct {
		name           string
		opts           []TransformOption
		wantBufferSize int
	}{
		{
			name:           "default config",
			opts:           nil,
			wantBufferSize: DefaultBufferSize,
		},
		{
			name:           "custom buffer size",
			opts:           []TransformOption{WithBufferSize(128)},
			wantBufferSize: 128,
		},
		{
			name:           "zero buffer size (unbuffered)",
			opts:           []TransformOption{WithBufferSize(0)},
			wantBufferSize: 0,
		},
		{
			name:           "multiple options last wins",
			opts:           []TransformOption{WithBufferSize(32), WithBufferSize(256)},
			wantBufferSize: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyOptions(tt.opts...)
			if cfg.BufferSize != tt.wantBufferSize {
				t.Errorf("BufferSize = %d, want %d", cfg.BufferSize, tt.wantBufferSize)
			}
		})
	}
}

func TestMapper_ApplyWith(t *testing.T) {
	ctx := context.Background()
	double := Map(func(x int) (int, error) { return x * 2, nil })

	tests := []struct {
		name       string
		input      []int
		opts       []TransformOption
		wantValues []int
	}{
		{
			name:       "default buffer",
			input:      []int{1, 2, 3},
			opts:       nil,
			wantValues: []int{2, 4, 6},
		},
		{
			name:       "custom buffer size",
			input:      []int{1, 2, 3, 4, 5},
			opts:       []TransformOption{WithBufferSize(2)},
			wantValues: []int{2, 4, 6, 8, 10},
		},
		{
			name:       "unbuffered",
			input:      []int{1, 2},
			opts:       []TransformOption{WithBufferSize(0)},
			wantValues: []int{2, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := Emit(func(ctx context.Context) <-chan Result[int] {
				ch := make(chan Result[int], len(tt.input))
				for _, v := range tt.input {
					ch <- Ok(v)
				}
				close(ch)
				return ch
			})

			result := double.ApplyWith(ctx, stream, tt.opts...)
			collected := result.Collect(ctx)

			if len(collected) != len(tt.wantValues) {
				t.Fatalf("got %d results, want %d", len(collected), len(tt.wantValues))
			}

			for i, res := range collected {
				if res.IsError() {
					t.Errorf("result[%d] is error: %v", i, res.Error())
					continue
				}
				if res.Value() != tt.wantValues[i] {
					t.Errorf("result[%d] = %d, want %d", i, res.Value(), tt.wantValues[i])
				}
			}
		})
	}
}

func TestFlatMapper_ApplyWith(t *testing.T) {
	ctx := context.Background()
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })

	tests := []struct {
		name       string
		input      []int
		opts       []TransformOption
		wantValues []int
	}{
		{
			name:       "default buffer",
			input:      []int{1, 2},
			opts:       nil,
			wantValues: []int{1, 1, 2, 2},
		},
		{
			name:       "custom buffer size",
			input:      []int{1, 2, 3},
			opts:       []TransformOption{WithBufferSize(4)},
			wantValues: []int{1, 1, 2, 2, 3, 3},
		},
		{
			name:       "unbuffered",
			input:      []int{5},
			opts:       []TransformOption{WithBufferSize(0)},
			wantValues: []int{5, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := Emit(func(ctx context.Context) <-chan Result[int] {
				ch := make(chan Result[int], len(tt.input))
				for _, v := range tt.input {
					ch <- Ok(v)
				}
				close(ch)
				return ch
			})

			result := duplicate.ApplyWith(ctx, stream, tt.opts...)
			collected := result.Collect(ctx)

			if len(collected) != len(tt.wantValues) {
				t.Fatalf("got %d results, want %d", len(collected), len(tt.wantValues))
			}

			for i, res := range collected {
				if res.IsError() {
					t.Errorf("result[%d] is error: %v", i, res.Error())
					continue
				}
				if res.Value() != tt.wantValues[i] {
					t.Errorf("result[%d] = %d, want %d", i, res.Value(), tt.wantValues[i])
				}
			}
		})
	}
}
