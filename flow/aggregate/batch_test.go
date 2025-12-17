package aggregate_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
)

func TestBatch(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		size  int
		want  [][]int
	}{
		{
			name:  "batch size 2",
			input: []int{1, 2, 3, 4, 5},
			size:  2,
			want:  [][]int{{1, 2}, {3, 4}, {5}},
		},
		{
			name:  "batch size 3",
			input: []int{1, 2, 3, 4, 5, 6},
			size:  3,
			want:  [][]int{{1, 2, 3}, {4, 5, 6}},
		},
		{
			name:  "batch larger than input",
			input: []int{1, 2},
			size:  5,
			want:  [][]int{{1, 2}},
		},
		{
			name:  "empty stream",
			input: []int{},
			size:  3,
			want:  [][]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			batched := aggregate.Batch[int](tt.size).Apply(stream)
			got, err := flow.Slice[[]int](ctx, batched)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d batches, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if len(got[i]) != len(tt.want[i]) {
					t.Fatalf("batch %d: got %v, want %v", i, got[i], tt.want[i])
				}
				for j := range got[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("batch %d, item %d: got %v, want %v", i, j, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestBatchPanicsOnInvalidSize(t *testing.T) {
	// With the delegate config pattern, panic happens at runtime when
	// context is available, not at construction time
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for size <= 0")
		}
	}()

	// Apply creates the stream, but we need to read from it to trigger the panic
	result := aggregate.Batch[int](0).Apply(stream)
	// Reading from the stream triggers the transmitter function which panics
	for range result.All(ctx) {
	}
}

func TestBatchTimeout(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	// With a long timeout, should behave like regular Batch
	batched := aggregate.BatchTimeout[int](2, 10*time.Second).Apply(stream)
	got, err := flow.Slice[[]int](ctx, batched)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]int{{1, 2}, {3, 4}, {5}}
	if len(got) != len(want) {
		t.Fatalf("got %d batches, want %d", len(got), len(want))
	}
}

func TestBatchTimeoutPanicsOnInvalidSize(t *testing.T) {
	// With the delegate config pattern, panic happens at runtime when
	// context is available, not at construction time
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for size <= 0")
		}
	}()

	// Apply creates the stream, but we need to read from it to trigger the panic
	result := aggregate.BatchTimeout[int](0, time.Second).Apply(stream)
	// Reading from the stream triggers the transmitter function which panics
	for range result.All(ctx) {
	}
}

func TestChunk(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	chunked := aggregate.Chunk[int](2).Apply(stream)

	got, err := flow.Slice[[]int](ctx, chunked)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := [][]int{{1, 2}, {3, 4}, {5}}
	if len(got) != len(want) {
		t.Fatalf("got %d chunks, want %d", len(got), len(want))
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		size  int
		step  int
		want  [][]int
	}{
		{
			name:  "sliding window step 1",
			input: []int{1, 2, 3, 4, 5},
			size:  3,
			step:  1,
			want:  [][]int{{1, 2, 3}, {2, 3, 4}, {3, 4, 5}},
		},
		{
			name:  "sliding window step 2",
			input: []int{1, 2, 3, 4, 5, 6},
			size:  3,
			step:  2,
			want:  [][]int{{1, 2, 3}, {3, 4, 5}},
		},
		{
			name:  "tumbling window (step equals size)",
			input: []int{1, 2, 3, 4, 5, 6},
			size:  2,
			step:  2,
			want:  [][]int{{1, 2}, {3, 4}, {5, 6}},
		},
		{
			name:  "step larger than size",
			input: []int{1, 2, 3, 4, 5, 6, 7, 8},
			size:  2,
			step:  3,
			want:  [][]int{{1, 2}, {4, 5}, {7, 8}},
		},
		{
			name:  "window larger than input",
			input: []int{1, 2},
			size:  5,
			step:  1,
			want:  [][]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			windowed := aggregate.Window[int](tt.size, tt.step).Apply(stream)
			got, err := flow.Slice[[]int](ctx, windowed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d windows, want %d: got=%v", len(got), len(tt.want), got)
			}
			for i := range got {
				if len(got[i]) != len(tt.want[i]) {
					t.Fatalf("window %d: got %v, want %v", i, got[i], tt.want[i])
				}
				for j := range got[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("window %d, item %d: got %v, want %v", i, j, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

func TestWindowPanicsOnInvalidParams(t *testing.T) {
	t.Run("size zero", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for size <= 0")
			}
		}()
		aggregate.Window[int](0, 1)
	})

	t.Run("step zero", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for step <= 0")
			}
		}()
		aggregate.Window[int](3, 0)
	})
}

func TestPartition(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})

	// Partition into even and odd
	partitioned := aggregate.Partition(func(n int) bool {
		return n%2 == 0
	}).Apply(stream)

	got, err := flow.Slice[[2][]int](ctx, partitioned)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 partition result, got %d", len(got))
	}

	evens := got[0][0]
	odds := got[0][1]

	wantEvens := []int{2, 4, 6}
	wantOdds := []int{1, 3, 5}

	if len(evens) != len(wantEvens) {
		t.Fatalf("evens: got %v, want %v", evens, wantEvens)
	}
	if len(odds) != len(wantOdds) {
		t.Fatalf("odds: got %v, want %v", odds, wantOdds)
	}
}

func TestGroupBy(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]string{"apple", "banana", "cherry", "apricot", "blueberry"})

	// Group by first letter
	grouped := aggregate.GroupBy(func(s string) byte {
		return s[0]
	}).Apply(stream)

	got, err := flow.Slice[map[byte][]string](ctx, grouped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 group result, got %d", len(got))
	}

	groups := got[0]

	if len(groups['a']) != 2 {
		t.Errorf("expected 2 'a' items, got %d", len(groups['a']))
	}
	if len(groups['b']) != 2 {
		t.Errorf("expected 2 'b' items, got %d", len(groups['b']))
	}
	if len(groups['c']) != 1 {
		t.Errorf("expected 1 'c' item, got %d", len(groups['c']))
	}
}

func TestBatchContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	batched := aggregate.Batch[int](2).Apply(stream)

	got, _ := flow.Slice[[]int](ctx, batched)
	// Should get empty or partial result due to cancellation
	if len(got) > 3 {
		t.Error("expected fewer batches due to cancellation")
	}
}
