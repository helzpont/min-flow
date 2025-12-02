package combine_test

import (
	"context"
	"sort"
	"sync/atomic"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
)

func TestFanOut(t *testing.T) {
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	outputs := combine.FanOut(3, stream)

	if len(outputs) != 3 {
		t.Fatalf("expected 3 output streams, got %d", len(outputs))
	}

	// Each output should receive all items
	ctx := context.Background()
	for i, out := range outputs {
		got, err := flow.Slice[int](ctx, out)
		if err != nil {
			t.Fatalf("output %d: unexpected error: %v", i, err)
		}

		want := []int{1, 2, 3, 4, 5}
		if len(got) != len(want) {
			t.Errorf("output %d: got len %d, want %d", i, len(got), len(want))
		}
	}
}

func TestFanOutZero(t *testing.T) {
	stream := flow.FromSlice([]int{1, 2, 3})
	outputs := combine.FanOut(0, stream)

	if outputs != nil {
		t.Error("expected nil for n <= 0")
	}
}

func TestFanOutWith(t *testing.T) {
	stream := flow.FromSlice([]int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// Route based on modulo 3
	outputs := combine.FanOutWith(3, func(n int) int {
		return n % 3
	}, stream)

	if len(outputs) != 3 {
		t.Fatalf("expected 3 output streams, got %d", len(outputs))
	}

	ctx := context.Background()

	// Output 0 should have 0, 3, 6, 9
	got0, _ := flow.Slice[int](ctx, outputs[0])
	sort.Ints(got0)
	if len(got0) != 4 {
		t.Errorf("output 0: got %v, want [0, 3, 6, 9]", got0)
	}

	// Output 1 should have 1, 4, 7
	got1, _ := flow.Slice[int](ctx, outputs[1])
	sort.Ints(got1)
	if len(got1) != 3 {
		t.Errorf("output 1: got %v, want [1, 4, 7]", got1)
	}

	// Output 2 should have 2, 5, 8
	got2, _ := flow.Slice[int](ctx, outputs[2])
	sort.Ints(got2)
	if len(got2) != 3 {
		t.Errorf("output 2: got %v, want [2, 5, 8]", got2)
	}
}

func TestFanIn(t *testing.T) {
	ctx := context.Background()

	stream1 := flow.FromSlice([]int{1, 2, 3})
	stream2 := flow.FromSlice([]int{4, 5, 6})

	merged := combine.FanIn(stream1, stream2)
	got, err := flow.Slice[int](ctx, merged)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 6 {
		t.Fatalf("got len %d, want 6", len(got))
	}

	sort.Ints(got)
	want := []int{1, 2, 3, 4, 5, 6}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBroadcast(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	var count1, count2 int32
	broadcast := combine.Broadcast(
		func(n int) { atomic.AddInt32(&count1, 1) },
		func(n int) { atomic.AddInt32(&count2, 1) },
	).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, broadcast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original items should pass through
	if len(got) != 5 {
		t.Errorf("got len %d, want 5", len(got))
	}

	// Both handlers should have been called for each item
	if atomic.LoadInt32(&count1) != 5 {
		t.Errorf("handler 1 called %d times, want 5", count1)
	}
	if atomic.LoadInt32(&count2) != 5 {
		t.Errorf("handler 2 called %d times, want 5", count2)
	}
}

func TestRoundRobin(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})

	doubled := combine.RoundRobin(3, func(n int) int {
		return n * 2
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, doubled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 6 {
		t.Fatalf("got len %d, want 6", len(got))
	}

	// All items should be doubled (order may vary due to concurrent workers)
	sort.Ints(got)
	want := []int{2, 4, 6, 8, 10, 12}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestTee(t *testing.T) {
	stream := flow.FromSlice([]int{1, 2, 3})
	out1, out2 := combine.Tee(stream)

	ctx := context.Background()

	got1, err := flow.Slice[int](ctx, out1)
	if err != nil {
		t.Fatalf("output 1: unexpected error: %v", err)
	}

	got2, err := flow.Slice[int](ctx, out2)
	if err != nil {
		t.Fatalf("output 2: unexpected error: %v", err)
	}

	want := []int{1, 2, 3}

	if len(got1) != len(want) {
		t.Errorf("output 1: got %v, want %v", got1, want)
	}
	if len(got2) != len(want) {
		t.Errorf("output 2: got %v, want %v", got2, want)
	}
}

func TestPartitionStream(t *testing.T) {
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})
	evens, odds := combine.PartitionStream(func(n int) bool {
		return n%2 == 0
	}, stream)

	ctx := context.Background()

	gotEvens, err := flow.Slice[int](ctx, evens)
	if err != nil {
		t.Fatalf("evens: unexpected error: %v", err)
	}

	gotOdds, err := flow.Slice[int](ctx, odds)
	if err != nil {
		t.Fatalf("odds: unexpected error: %v", err)
	}

	sort.Ints(gotEvens)
	sort.Ints(gotOdds)

	wantEvens := []int{2, 4, 6}
	wantOdds := []int{1, 3, 5}

	if len(gotEvens) != len(wantEvens) {
		t.Errorf("evens: got %v, want %v", gotEvens, wantEvens)
	}
	if len(gotOdds) != len(wantOdds) {
		t.Errorf("odds: got %v, want %v", gotOdds, wantOdds)
	}
}
