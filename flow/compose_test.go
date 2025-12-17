package flow_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
)

func TestThrough(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create two transformers: double and add 1
	// Mapper now implements Transformer directly
	double := flow.Map(func(x int) (int, error) {
		return x * 2, nil
	})

	addOne := flow.Map(func(x int) (int, error) {
		return x + 1, nil
	})

	// Chain them: first double, then add 1
	combined := flow.Through(double, addOne)

	stream := flow.FromSlice([]int{1, 2, 3})
	result, err := flow.Slice(ctx, combined.Apply(stream))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: (1*2)+1=3, (2*2)+1=5, (3*2)+1=7
	expected := []int{3, 5, 7}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestChain(t *testing.T) {
	tests := []struct {
		name         string
		transformers []flow.Transformer[int, int]
		input        []int
		expected     []int
	}{
		{
			name:         "empty chain (identity)",
			transformers: []flow.Transformer[int, int]{},
			input:        []int{1, 2, 3},
			expected:     []int{1, 2, 3},
		},
		{
			name: "single transformer",
			transformers: []flow.Transformer[int, int]{
				flow.Map(func(x int) (int, error) {
					return x * 2, nil
				}),
			},
			input:    []int{1, 2, 3},
			expected: []int{2, 4, 6},
		},
		{
			name: "multiple transformers",
			transformers: []flow.Transformer[int, int]{
				flow.Map(func(x int) (int, error) {
					return x * 2, nil
				}),
				flow.Map(func(x int) (int, error) {
					return x + 1, nil
				}),
				flow.Map(func(x int) (int, error) {
					return x * 10, nil
				}),
			},
			input:    []int{1, 2, 3},
			expected: []int{30, 50, 70}, // ((1*2)+1)*10=30, ((2*2)+1)*10=50, ((3*2)+1)*10=70
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			chained := flow.Chain(tt.transformers...)
			stream := flow.FromSlice(tt.input)
			result, err := flow.Slice(ctx, chained.Apply(stream))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("element %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestPipe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	double := flow.Map(func(x int) (int, error) {
		return x * 2, nil
	})

	addOne := flow.Map(func(x int) (int, error) {
		return x + 1, nil
	})

	source := flow.FromSlice([]int{1, 2, 3})
	result, err := flow.Slice(ctx, flow.Pipe(ctx, source, double, addOne))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{3, 5, 7}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestApply(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	transformer := flow.Map(func(x int) (int, error) {
		return x * 2, nil
	})

	stream := flow.FromSlice([]int{1, 2, 3})
	result, err := flow.Slice(ctx, flow.Apply(ctx, stream, transformer))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestMapper(t *testing.T) {
	tests := []struct {
		name     string
		mapper   func(int) (string, error)
		input    []int
		expected []string
	}{
		{
			name: "int to string",
			mapper: func(x int) (string, error) {
				return string(rune('a' + x)), nil
			},
			input:    []int{0, 1, 2},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// Mapper implements Transformer directly
			transformer := flow.Map(tt.mapper)
			stream := flow.FromSlice(tt.input)
			result, err := flow.Slice(ctx, transformer.Apply(stream))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("element %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestFlatMapper(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Expand each number into [n, n, n]
	// FlatMapper implements Transformer directly
	transformer := flow.FlatMap(func(x int) ([]int, error) {
		return []int{x, x, x}, nil
	})

	stream := flow.FromSlice([]int{1, 2})
	result, err := flow.Slice(ctx, transformer.Apply(stream))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{1, 1, 1, 2, 2, 2}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestFlatMapper_Filter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Use FlatMap to filter: only keep even numbers
	transformer := flow.FlatMap(func(x int) ([]int, error) {
		if x%2 == 0 {
			return []int{x}, nil
		}
		return []int{}, nil // filter out odd numbers
	})

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})
	result, err := flow.Slice(ctx, transformer.Apply(stream))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestTransmitter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Create a Transmitter that doubles values
	// Transmitter implements Transformer directly
	transmitter := flow.Transmit(func(ctx context.Context, in <-chan flow.Result[int]) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for res := range in {
				if res.IsValue() {
					out <- flow.Ok(res.Value() * 2)
				} else {
					out <- res
				}
			}
		}()
		return out
	})

	stream := flow.FromSlice([]int{1, 2, 3})
	result, err := flow.Slice(ctx, transmitter.Apply(stream))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6}
	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
		}
	}
}
