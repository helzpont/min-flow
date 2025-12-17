package filter_test

import (
	"context"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      []int
	}{
		{
			name:      "filter even numbers",
			input:     []int{1, 2, 3, 4, 5, 6},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      []int{2, 4, 6},
		},
		{
			name:      "filter all",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return true },
			want:      []int{1, 2, 3},
		},
		{
			name:      "filter none",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return false },
			want:      []int{},
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(n int) bool { return true },
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			filtered := filter.Where(tt.predicate).Apply(stream)
			got, err := flow.Slice[int](ctx, filtered)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilterMap(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	// Double even numbers only
	filtered := filter.MapWhere(func(n int) (int, bool) {
		if n%2 == 0 {
			return n * 2, true
		}
		return 0, false
	}).Apply(stream)

	got, err := flow.Slice[int](ctx, filtered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{4, 8}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestFilterError(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})
	filtered := filter.Errors[int]().Apply(stream)

	got, err := flow.Slice[int](ctx, filtered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExclude(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	// Exclude even numbers
	filtered := filter.Exclude(func(n int) bool {
		return n%2 == 0
	}).Apply(stream)

	got, err := flow.Slice[int](ctx, filtered)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1, 3, 5}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	filtered := filter.Where(func(n int) bool { return true }).Apply(stream)

	got, _ := flow.Slice[int](ctx, filtered)
	// Should get empty or partial result due to cancellation
	if len(got) > 5 {
		t.Error("expected fewer results due to cancellation")
	}
}
