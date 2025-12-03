package parallel_test

import (
	"context"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/parallel"
)

func TestParallel(t *testing.T) {
	tests := []struct {
		name    string
		input   []int
		workers int
		wantLen int
		wantSum int
	}{
		{
			name:    "parallel with 2 workers",
			input:   []int{1, 2, 3, 4, 5},
			workers: 2,
			wantLen: 5,
			wantSum: 30, // doubled: 2+4+6+8+10
		},
		{
			name:    "parallel with more workers than items",
			input:   []int{1, 2, 3},
			workers: 10,
			wantLen: 3,
			wantSum: 12, // doubled: 2+4+6
		},
		{
			name:    "parallel with 0 workers defaults to 1",
			input:   []int{1, 2, 3},
			workers: 0,
			wantLen: 3,
			wantSum: 12, // doubled: 2+4+6
		},
		{
			name:    "empty stream",
			input:   []int{},
			workers: 4,
			wantLen: 0,
			wantSum: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			doubled := parallel.Map(tt.workers, func(n int) int {
				return n * 2
			}).Apply(ctx, stream)

			got, err := flow.Slice[int](ctx, doubled)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tt.wantLen {
				t.Errorf("got len %d, want %d", len(got), tt.wantLen)
			}

			sum := 0
			for _, v := range got {
				sum += v
			}
			if sum != tt.wantSum {
				t.Errorf("got sum %d, want %d", sum, tt.wantSum)
			}
		})
	}
}

func TestParallelOrdered(t *testing.T) {
	ctx := context.Background()
	input := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stream := flow.FromSlice(input)

	doubled := parallel.Ordered(4, func(n int) int {
		// Add some jitter to test ordering
		time.Sleep(time.Duration(n%3) * time.Millisecond)
		return n * 2
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, doubled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParallelFlatMap(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	expanded := parallel.FlatMap(2, func(n int) []int {
		result := make([]int, n)
		for i := 0; i < n; i++ {
			result[i] = n
		}
		return result
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, expanded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 + 2 + 3 = 6 items
	if len(got) != 6 {
		t.Errorf("got len %d, want 6", len(got))
	}

	// Count occurrences
	counts := make(map[int]int)
	for _, v := range got {
		counts[v]++
	}

	if counts[1] != 1 || counts[2] != 2 || counts[3] != 3 {
		t.Errorf("unexpected counts: %v", counts)
	}
}

func TestParallelContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var processed int32
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	doubled := parallel.Map(4, func(n int) int {
		atomic.AddInt32(&processed, 1)
		time.Sleep(50 * time.Millisecond)
		return n * 2
	}).Apply(ctx, stream)

	// Cancel after a short delay
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	_, _ = flow.Slice[int](ctx, doubled)

	// Should not have processed all items
	if atomic.LoadInt32(&processed) == 10 {
		t.Error("expected cancellation to prevent processing all items")
	}
}

func TestParallelPanicRecovery(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	result := parallel.Map(2, func(n int) int {
		if n == 2 {
			panic("intentional panic")
		}
		return n * 2
	}).Apply(ctx, stream)

	// Collect all results including errors
	var values []int
	var errs []error
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errs = append(errs, res.Error())
		} else if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	// Should get at least 2 results (the non-panicking items)
	if len(values) < 2 {
		t.Errorf("expected at least 2 values, got %d", len(values))
	}

	// Should have 1 error from the panic
	if len(errs) != 1 {
		t.Errorf("expected 1 error from panic, got %d", len(errs))
	}
}

func TestAsyncMap(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	doubled := parallel.AsyncMap(func(ctx context.Context, n int) (int, error) {
		time.Sleep(time.Duration(n) * time.Millisecond)
		return n * 2, nil
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, doubled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 5 {
		t.Errorf("got len %d, want 5", len(got))
	}

	// Verify all values present (may be out of order)
	sort.Ints(got)
	want := []int{2, 4, 6, 8, 10}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestAsyncMapOrdered(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	doubled := parallel.AsyncMapOrdered(func(ctx context.Context, n int) (int, error) {
		// Add jitter - items processed out of natural order
		time.Sleep(time.Duration(5-n) * time.Millisecond)
		return n * 2, nil
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, doubled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should preserve order despite async processing
	want := []int{2, 4, 6, 8, 10}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
