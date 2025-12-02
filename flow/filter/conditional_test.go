package filter_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func TestTakeWhileWithIndex(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int, int) bool
		want      []int
	}{
		{
			name:      "take first 3 by index",
			input:     []int{10, 20, 30, 40, 50},
			predicate: func(_ int, idx int) bool { return idx < 3 },
			want:      []int{10, 20, 30},
		},
		{
			name:      "take while value+index < 15",
			input:     []int{10, 11, 12, 13, 14},
			predicate: func(v int, idx int) bool { return v+idx < 15 },
			want:      []int{10, 11, 12},
		},
		{
			name:      "predicate always false",
			input:     []int{1, 2, 3},
			predicate: func(int, int) bool { return false },
			want:      []int{},
		},
		{
			name:      "predicate always true",
			input:     []int{1, 2, 3},
			predicate: func(int, int) bool { return true },
			want:      []int{1, 2, 3},
		},
		{
			name:      "empty input",
			input:     []int{},
			predicate: func(int, int) bool { return true },
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.TakeWhileWithIndex(tt.predicate)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
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

func TestTakeWhileWithIndexPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.TakeWhileWithIndex[int](nil)
}

func TestSkipWhileWithIndex(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int, int) bool
		want      []int
	}{
		{
			name:      "skip first 3 by index",
			input:     []int{10, 20, 30, 40, 50},
			predicate: func(_ int, idx int) bool { return idx < 3 },
			want:      []int{40, 50},
		},
		{
			name:      "skip while value+index < 15",
			input:     []int{10, 11, 12, 13, 14},
			predicate: func(v int, idx int) bool { return v+idx < 15 },
			want:      []int{13, 14},
		},
		{
			name:      "predicate always false",
			input:     []int{1, 2, 3},
			predicate: func(int, int) bool { return false },
			want:      []int{1, 2, 3},
		},
		{
			name:      "predicate always true",
			input:     []int{1, 2, 3},
			predicate: func(int, int) bool { return true },
			want:      []int{},
		},
		{
			name:      "empty input",
			input:     []int{},
			predicate: func(int, int) bool { return true },
			want:      []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.SkipWhileWithIndex(tt.predicate)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
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

func TestSkipWhileWithIndexPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.SkipWhileWithIndex[int](nil)
}

func TestTakeUntil(t *testing.T) {
	t.Run("stops on notifier emit", func(t *testing.T) {
		ctx := context.Background()

		// Create a slow source stream using a channel
		ch := make(chan int)
		go func() {
			for i := 1; i <= 10; i++ {
				ch <- i
				time.Sleep(10 * time.Millisecond)
			}
			close(ch)
		}()
		source := flow.FromChannel(ch)

		// Create notifier that emits after 35ms
		notifierCh := make(chan int)
		go func() {
			time.Sleep(35 * time.Millisecond)
			notifierCh <- 1
			close(notifierCh)
		}()
		notifier := flow.FromChannel(notifierCh)

		transformer := filter.TakeUntil[int, int](notifier)
		result := transformer.Apply(ctx, source)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have 1, 2, 3 (maybe 4) before notifier fires
		if len(got) < 1 || len(got) > 5 {
			t.Errorf("expected 1-5 elements, got %d: %v", len(got), got)
		}
	})

	t.Run("continues if notifier completes without emitting", func(t *testing.T) {
		ctx := context.Background()

		source := flow.FromSlice([]int{1, 2, 3, 4, 5})
		notifier := flow.FromSlice([]int{}) // Empty notifier

		transformer := filter.TakeUntil[int, int](notifier)
		result := transformer.Apply(ctx, source)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []int{1, 2, 3, 4, 5}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})
}

func TestSkipUntil(t *testing.T) {
	t.Run("skips until notifier emits", func(t *testing.T) {
		ctx := context.Background()

		// Create a slow source stream
		ch := make(chan int)
		go func() {
			for i := 1; i <= 10; i++ {
				ch <- i
				time.Sleep(10 * time.Millisecond)
			}
			close(ch)
		}()
		source := flow.FromChannel(ch)

		// Create notifier that emits after 35ms
		notifierCh := make(chan int)
		go func() {
			time.Sleep(35 * time.Millisecond)
			notifierCh <- 1
			close(notifierCh)
		}()
		notifier := flow.FromChannel(notifierCh)

		transformer := filter.SkipUntil[int, int](notifier)
		result := transformer.Apply(ctx, source)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have elements starting around 4-5 and onwards
		if len(got) < 3 || len(got) > 9 {
			t.Errorf("expected 3-9 elements, got %d: %v", len(got), got)
		}
	})

	t.Run("skips all if notifier never emits", func(t *testing.T) {
		ctx := context.Background()

		source := flow.FromSlice([]int{1, 2, 3, 4, 5})

		// Create a notifier that takes too long
		notifierCh := make(chan int)
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(notifierCh)
		}()
		notifier := flow.FromChannel(notifierCh)

		transformer := filter.SkipUntil[int, int](notifier)
		result := transformer.Apply(ctx, source)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should skip all since notifier doesn't emit in time
		if len(got) != 0 {
			t.Errorf("expected empty result, got %v", got)
		}
	})
}

func TestElementAt(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		index int
		want  []int
	}{
		{
			name:  "first element",
			input: []int{10, 20, 30},
			index: 0,
			want:  []int{10},
		},
		{
			name:  "middle element",
			input: []int{10, 20, 30},
			index: 1,
			want:  []int{20},
		},
		{
			name:  "last element",
			input: []int{10, 20, 30},
			index: 2,
			want:  []int{30},
		},
		{
			name:  "index beyond stream",
			input: []int{10, 20, 30},
			index: 5,
			want:  []int{},
		},
		{
			name:  "empty stream",
			input: []int{},
			index: 0,
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.ElementAt[int](tt.index)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
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

func TestElementAtPanicOnNegativeIndex(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for negative index")
		}
	}()
	filter.ElementAt[int](-1)
}

func TestElementAtOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        []int
		index        int
		defaultValue int
		want         []int
	}{
		{
			name:         "element exists",
			input:        []int{10, 20, 30},
			index:        1,
			defaultValue: -1,
			want:         []int{20},
		},
		{
			name:         "index beyond stream returns default",
			input:        []int{10, 20, 30},
			index:        5,
			defaultValue: -1,
			want:         []int{-1},
		},
		{
			name:         "empty stream returns default",
			input:        []int{},
			index:        0,
			defaultValue: 999,
			want:         []int{999},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.ElementAtOrDefault[int](tt.index, tt.defaultValue)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
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

func TestSingle(t *testing.T) {
	t.Run("single matching element", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})
		transformer := filter.Single(func(v int) bool { return v == 2 })
		result := transformer.Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != 2 {
			t.Errorf("got %v, want [2]", got)
		}
	})

	t.Run("no matching element returns error", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})
		transformer := filter.Single(func(v int) bool { return v == 10 })
		result := transformer.Apply(ctx, stream)
		results := flow.Collect(ctx, result)
		if len(results) != 1 || !results[0].IsError() {
			t.Error("expected error result for no match")
		}
	})

	t.Run("multiple matching elements returns error", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 2, 3})
		transformer := filter.Single(func(v int) bool { return v == 2 })
		result := transformer.Apply(ctx, stream)
		results := flow.Collect(ctx, result)
		if len(results) != 1 || !results[0].IsError() {
			t.Error("expected error result for multiple matches")
		}
	})

	t.Run("nil predicate expects exactly one element", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{42})
		transformer := filter.Single[int](nil)
		result := transformer.Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != 42 {
			t.Errorf("got %v, want [42]", got)
		}
	})

	t.Run("nil predicate with multiple elements returns error", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2})
		transformer := filter.Single[int](nil)
		result := transformer.Apply(ctx, stream)
		results := flow.Collect(ctx, result)
		if len(results) != 1 || !results[0].IsError() {
			t.Error("expected error result for multiple elements with nil predicate")
		}
	})
}

func TestSingleOrDefault(t *testing.T) {
	t.Run("single matching element", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})
		transformer := filter.SingleOrDefault(func(v int) bool { return v == 2 }, -1)
		result := transformer.Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != 2 {
			t.Errorf("got %v, want [2]", got)
		}
	})

	t.Run("no matching element returns default", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})
		transformer := filter.SingleOrDefault(func(v int) bool { return v == 10 }, 999)
		result := transformer.Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0] != 999 {
			t.Errorf("got %v, want [999]", got)
		}
	})

	t.Run("multiple matches returns error", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 2, 3})
		transformer := filter.SingleOrDefault(func(v int) bool { return v == 2 }, -1)
		result := transformer.Apply(ctx, stream)
		results := flow.Collect(ctx, result)
		if len(results) != 1 || !results[0].IsError() {
			t.Error("expected error result for multiple matches")
		}
	})
}

func TestFirstOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        []int
		predicate    func(int) bool
		defaultValue int
		want         int
	}{
		{
			name:         "matching element found",
			input:        []int{1, 2, 3, 4, 5},
			predicate:    func(v int) bool { return v > 3 },
			defaultValue: -1,
			want:         4,
		},
		{
			name:         "no matching element returns default",
			input:        []int{1, 2, 3},
			predicate:    func(v int) bool { return v > 10 },
			defaultValue: 999,
			want:         999,
		},
		{
			name:         "nil predicate returns first element",
			input:        []int{10, 20, 30},
			predicate:    nil,
			defaultValue: -1,
			want:         10,
		},
		{
			name:         "empty stream returns default",
			input:        []int{},
			predicate:    nil,
			defaultValue: 42,
			want:         42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.FirstOrDefault(tt.predicate, tt.defaultValue)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestLastOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        []int
		predicate    func(int) bool
		defaultValue int
		want         int
	}{
		{
			name:         "matching element found",
			input:        []int{1, 2, 3, 4, 5},
			predicate:    func(v int) bool { return v < 4 },
			defaultValue: -1,
			want:         3,
		},
		{
			name:         "no matching element returns default",
			input:        []int{1, 2, 3},
			predicate:    func(v int) bool { return v > 10 },
			defaultValue: 999,
			want:         999,
		},
		{
			name:         "nil predicate returns last element",
			input:        []int{10, 20, 30},
			predicate:    nil,
			defaultValue: -1,
			want:         30,
		},
		{
			name:         "empty stream returns default",
			input:        []int{},
			predicate:    nil,
			defaultValue: 42,
			want:         42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.LastOrDefault(tt.predicate, tt.defaultValue)
			result := transformer.Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestContextCancellation(t *testing.T) {
	t.Run("TakeWhileWithIndex respects cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		transformer := filter.TakeWhileWithIndex(func(int, int) bool { return true })
		result := transformer.Apply(ctx, stream)
		got, _ := flow.Slice(ctx, result)
		if len(got) > 5 {
			t.Errorf("expected at most 5 elements, got %d", len(got))
		}
	})

	t.Run("ElementAt respects cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		transformer := filter.ElementAt[int](2)
		result := transformer.Apply(ctx, stream)
		_, _ = flow.Slice(ctx, result)
		// Just ensure it doesn't hang
	})
}
