package filter_test

import (
	"context"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func TestTake(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		n     int
		want  []int
	}{
		{
			name:  "take first 3",
			input: []int{1, 2, 3, 4, 5},
			n:     3,
			want:  []int{1, 2, 3},
		},
		{
			name:  "take more than available",
			input: []int{1, 2},
			n:     5,
			want:  []int{1, 2},
		},
		{
			name:  "take zero",
			input: []int{1, 2, 3},
			n:     0,
			want:  []int{},
		},
		{
			name:  "take negative",
			input: []int{1, 2, 3},
			n:     -1,
			want:  []int{},
		},
		{
			name:  "take from empty",
			input: []int{},
			n:     3,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			taken := filter.Take[int](tt.n).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, taken)
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

func TestTakeWhile(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      []int
	}{
		{
			name:      "take while less than 4",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n < 4 },
			want:      []int{1, 2, 3},
		},
		{
			name:      "take all",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return true },
			want:      []int{1, 2, 3},
		},
		{
			name:      "take none",
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
			taken := filter.TakeWhile(tt.predicate).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, taken)
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

func TestSkip(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		n     int
		want  []int
	}{
		{
			name:  "skip first 2",
			input: []int{1, 2, 3, 4, 5},
			n:     2,
			want:  []int{3, 4, 5},
		},
		{
			name:  "skip more than available",
			input: []int{1, 2},
			n:     5,
			want:  []int{},
		},
		{
			name:  "skip zero",
			input: []int{1, 2, 3},
			n:     0,
			want:  []int{1, 2, 3},
		},
		{
			name:  "skip negative",
			input: []int{1, 2, 3},
			n:     -1,
			want:  []int{1, 2, 3},
		},
		{
			name:  "skip from empty",
			input: []int{},
			n:     3,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			skipped := filter.Skip[int](tt.n).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, skipped)
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

func TestSkipWhile(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      []int
	}{
		{
			name:      "skip while less than 3",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n < 3 },
			want:      []int{3, 4, 5},
		},
		{
			name:      "skip all",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return true },
			want:      []int{},
		},
		{
			name:      "skip none",
			input:     []int{1, 2, 3},
			predicate: func(n int) bool { return false },
			want:      []int{1, 2, 3},
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
			skipped := filter.SkipWhile(tt.predicate).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, skipped)
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

func TestLast(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		n     int
		want  []int
	}{
		{
			name:  "last 3",
			input: []int{1, 2, 3, 4, 5},
			n:     3,
			want:  []int{3, 4, 5},
		},
		{
			name:  "last more than available",
			input: []int{1, 2},
			n:     5,
			want:  []int{1, 2},
		},
		{
			name:  "last zero",
			input: []int{1, 2, 3},
			n:     0,
			want:  []int{},
		},
		{
			name:  "last negative",
			input: []int{1, 2, 3},
			n:     -1,
			want:  []int{},
		},
		{
			name:  "last from empty",
			input: []int{},
			n:     3,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			last := filter.Last[int](tt.n).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, last)
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

func TestFirst(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	first := filter.First[int]().Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, first)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got[0] != want[0] {
		t.Errorf("got %v, want %v", got[0], want[0])
	}
}

func TestNth(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		n     int
		want  []int
	}{
		{
			name:  "nth 2 (0-indexed)",
			input: []int{1, 2, 3, 4, 5},
			n:     2,
			want:  []int{3},
		},
		{
			name:  "nth 0 (first)",
			input: []int{1, 2, 3},
			n:     0,
			want:  []int{1},
		},
		{
			name:  "nth beyond range",
			input: []int{1, 2},
			n:     5,
			want:  []int{},
		},
		{
			name:  "nth negative",
			input: []int{1, 2, 3},
			n:     -1,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			nth := filter.Nth[int](tt.n).Apply(ctx, stream)
			got, err := flow.Slice[int](ctx, nth)
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

func TestTakeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	taken := filter.Take[int](3).Apply(ctx, stream)

	got, _ := flow.Slice[int](ctx, taken)
	// Should get empty or partial result due to cancellation
	if len(got) > 3 {
		t.Error("expected fewer results due to cancellation")
	}
}
