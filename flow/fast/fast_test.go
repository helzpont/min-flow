package fast

import (
	"context"
	"testing"
)

func TestFromSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{"empty slice", []int{}, []int{}},
		{"single element", []int{42}, []int{42}},
		{"multiple elements", []int{1, 2, 3, 4, 5}, []int{1, 2, 3, 4, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, FromSlice(tt.input))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		end      int
		expected []int
	}{
		{"empty range", 5, 5, []int{}},
		{"single element", 0, 1, []int{0}},
		{"multiple elements", 1, 5, []int{1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, Range(tt.start, tt.end))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		fn       func(int) int
		expected []int
	}{
		{"double", []int{1, 2, 3}, func(n int) int { return n * 2 }, []int{2, 4, 6}},
		{"square", []int{1, 2, 3, 4}, func(n int) int { return n * n }, []int{1, 4, 9, 16}},
		{"empty", []int{}, func(n int) int { return n }, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, Map(tt.fn).Apply(ctx, FromSlice(tt.input)))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		pred     func(int) bool
		expected []int
	}{
		{"even numbers", []int{1, 2, 3, 4, 5, 6}, func(n int) bool { return n%2 == 0 }, []int{2, 4, 6}},
		{"all match", []int{2, 4, 6}, func(n int) bool { return n%2 == 0 }, []int{2, 4, 6}},
		{"none match", []int{1, 3, 5}, func(n int) bool { return n%2 == 0 }, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, Filter(tt.pred).Apply(ctx, FromSlice(tt.input)))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestFuse(t *testing.T) {
	ctx := context.Background()

	addOne := Map(func(n int) int { return n + 1 })
	double := Map(func(n int) int { return n * 2 })

	fused := Fuse(addOne, double)

	result := Slice(ctx, fused.Apply(ctx, FromSlice([]int{1, 2, 3})))

	expected := []int{4, 6, 8} // (1+1)*2, (2+1)*2, (3+1)*2

	if len(result) != len(expected) {
		t.Fatalf("expected %d elements, got %d", len(expected), len(result))
	}
	for i, v := range result {
		if v != expected[i] {
			t.Errorf("index %d: expected %d, got %d", i, expected[i], v)
		}
	}
}

func TestReduce(t *testing.T) {
	ctx := context.Background()

	result := Slice(ctx, Reduce(func(a, b int) int { return a + b }).Apply(ctx, FromSlice([]int{1, 2, 3, 4, 5})))

	if len(result) != 1 {
		t.Fatalf("expected 1 element, got %d", len(result))
	}
	if result[0] != 15 {
		t.Errorf("expected 15, got %d", result[0])
	}
}

func TestFold(t *testing.T) {
	ctx := context.Background()

	result := Slice(ctx, Fold(10, func(acc, v int) int { return acc + v }).Apply(ctx, FromSlice([]int{1, 2, 3, 4, 5})))

	if len(result) != 1 {
		t.Fatalf("expected 1 element, got %d", len(result))
	}
	if result[0] != 25 { // 10 + 1+2+3+4+5
		t.Errorf("expected 25, got %d", result[0])
	}
}

func TestTake(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		n        int
		expected []int
	}{
		{"take 3", []int{1, 2, 3, 4, 5}, 3, []int{1, 2, 3}},
		{"take more than available", []int{1, 2}, 5, []int{1, 2}},
		{"take zero", []int{1, 2, 3}, 0, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, Take[int](tt.n).Apply(ctx, FromSlice(tt.input)))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestSkip(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		n        int
		expected []int
	}{
		{"skip 2", []int{1, 2, 3, 4, 5}, 2, []int{3, 4, 5}},
		{"skip more than available", []int{1, 2}, 5, []int{}},
		{"skip zero", []int{1, 2, 3}, 0, []int{1, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := Slice(ctx, Skip[int](tt.n).Apply(ctx, FromSlice(tt.input)))

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestFirst(t *testing.T) {
	ctx := context.Background()

	v, ok := First(ctx, FromSlice([]int{42, 1, 2}))
	if !ok || v != 42 {
		t.Errorf("expected (42, true), got (%d, %v)", v, ok)
	}

	_, ok = First(ctx, FromSlice([]int{}))
	if ok {
		t.Error("expected ok=false for empty stream")
	}
}

func TestCount(t *testing.T) {
	ctx := context.Background()

	if c := Count(ctx, FromSlice([]int{1, 2, 3, 4, 5})); c != 5 {
		t.Errorf("expected 5, got %d", c)
	}

	if c := Count(ctx, FromSlice([]int{})); c != 0 {
		t.Errorf("expected 0, got %d", c)
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stream := Range(0, 1000000)

	count := 0
	for range stream.Emit(ctx) {
		count++
		if count >= 10 {
			cancel()
			break
		}
	}

	if count < 10 {
		t.Errorf("expected at least 10 items before cancel, got %d", count)
	}
}
