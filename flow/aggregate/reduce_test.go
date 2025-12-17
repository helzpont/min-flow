package aggregate_test

import (
	"context"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
)

func TestReduce(t *testing.T) {
	tests := []struct {
		name    string
		input   []int
		reducer func(acc, item int) int
		want    []int
	}{
		{
			name:    "sum",
			input:   []int{1, 2, 3, 4, 5},
			reducer: func(acc, item int) int { return acc + item },
			want:    []int{15},
		},
		{
			name:    "product",
			input:   []int{1, 2, 3, 4},
			reducer: func(acc, item int) int { return acc * item },
			want:    []int{24},
		},
		{
			name:    "single item",
			input:   []int{42},
			reducer: func(acc, item int) int { return acc + item },
			want:    []int{42},
		},
		{
			name:    "empty stream",
			input:   []int{},
			reducer: func(acc, item int) int { return acc + item },
			want:    []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			reduced := aggregate.Reduce(tt.reducer).Apply(stream)
			got, err := flow.Slice[int](ctx, reduced)
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

func TestFold(t *testing.T) {
	tests := []struct {
		name    string
		input   []int
		initial int
		folder  func(acc int, item int) int
		want    int
	}{
		{
			name:    "sum with initial",
			input:   []int{1, 2, 3},
			initial: 10,
			folder:  func(acc, item int) int { return acc + item },
			want:    16,
		},
		{
			name:    "empty stream returns initial",
			input:   []int{},
			initial: 42,
			folder:  func(acc, item int) int { return acc + item },
			want:    42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			folded := aggregate.Fold(tt.initial, tt.folder).Apply(stream)
			got, err := flow.Slice[int](ctx, folded)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestScan(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4})

	// Running sum
	scanned := aggregate.Scan(0, func(acc, item int) int {
		return acc + item
	}).Apply(stream)

	got, err := flow.Slice[int](ctx, scanned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{1, 3, 6, 10}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  int
	}{
		{
			name:  "count 5 items",
			input: []int{1, 2, 3, 4, 5},
			want:  5,
		},
		{
			name:  "empty stream",
			input: []int{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			counted := aggregate.Count[int]().Apply(stream)
			got, err := flow.Slice[int](ctx, counted)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestSum(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  int
	}{
		{
			name:  "sum integers",
			input: []int{1, 2, 3, 4, 5},
			want:  15,
		},
		{
			name:  "empty stream",
			input: []int{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			summed := aggregate.Sum[int]().Apply(stream)
			got, err := flow.Slice[int](ctx, summed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestSumFloats(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]float64{1.5, 2.5, 3.0})
	summed := aggregate.Sum[float64]().Apply(stream)

	got, err := flow.Slice[float64](ctx, summed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0] != 7.0 {
		t.Errorf("got %v, want 7.0", got[0])
	}
}

func TestAverage(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  float64
	}{
		{
			name:  "average integers",
			input: []int{2, 4, 6, 8},
			want:  5.0,
		},
		{
			name:  "empty stream",
			input: []int{},
			want:  0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			averaged := aggregate.Average[int]().Apply(stream)
			got, err := flow.Slice[float64](ctx, averaged)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{
			name:  "find minimum",
			input: []int{3, 1, 4, 1, 5, 9},
			want:  []int{1},
		},
		{
			name:  "single item",
			input: []int{42},
			want:  []int{42},
		},
		{
			name:  "empty stream",
			input: []int{},
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			minStream := aggregate.Min(func(a, b int) bool { return a < b }).Apply(stream)
			got, err := flow.Slice[int](ctx, minStream)
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

func TestMax(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{
			name:  "find maximum",
			input: []int{3, 1, 4, 1, 5, 9},
			want:  []int{9},
		},
		{
			name:  "single item",
			input: []int{42},
			want:  []int{42},
		},
		{
			name:  "empty stream",
			input: []int{},
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			maxStream := aggregate.Max(func(a, b int) bool { return a < b }).Apply(stream)
			got, err := flow.Slice[int](ctx, maxStream)
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

func TestAll(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      bool
	}{
		{
			name:      "all even",
			input:     []int{2, 4, 6, 8},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      true,
		},
		{
			name:      "not all even",
			input:     []int{2, 3, 6, 8},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      false,
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(n int) bool { return false },
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			allStream := aggregate.All(tt.predicate).Apply(stream)
			got, err := flow.Slice[bool](ctx, allStream)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestAny(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      bool
	}{
		{
			name:      "has even",
			input:     []int{1, 3, 4, 7},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      true,
		},
		{
			name:      "no even",
			input:     []int{1, 3, 5, 7},
			predicate: func(n int) bool { return n%2 == 0 },
			want:      false,
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(n int) bool { return true },
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			anyStream := aggregate.Any(tt.predicate).Apply(stream)
			got, err := flow.Slice[bool](ctx, anyStream)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestNone(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      bool
	}{
		{
			name:      "none negative",
			input:     []int{1, 2, 3, 4},
			predicate: func(n int) bool { return n < 0 },
			want:      true,
		},
		{
			name:      "has negative",
			input:     []int{1, -2, 3, 4},
			predicate: func(n int) bool { return n < 0 },
			want:      false,
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(n int) bool { return true },
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			noneStream := aggregate.None(tt.predicate).Apply(stream)
			got, err := flow.Slice[bool](ctx, noneStream)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0] != tt.want {
				t.Errorf("got %v, want %v", got[0], tt.want)
			}
		})
	}
}

func TestReduceContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	reduced := aggregate.Reduce(func(acc, item int) int {
		return acc + item
	}).Apply(stream)

	got, _ := flow.Slice[int](ctx, reduced)
	// Should get empty or partial result due to cancellation
	if len(got) > 1 {
		t.Error("expected fewer results due to cancellation")
	}
}
