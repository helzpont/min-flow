package transform_test

import (
	"context"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/transform"
)

func TestPairwise(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected [][2]int
	}{
		{
			name:     "empty stream",
			input:    []int{},
			expected: nil,
		},
		{
			name:     "single element",
			input:    []int{1},
			expected: nil,
		},
		{
			name:     "two elements",
			input:    []int{1, 2},
			expected: [][2]int{{1, 2}},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: [][2]int{{1, 2}, {2, 3}, {3, 4}, {4, 5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			pairwise := transform.Pairwise[int]()
			result := pairwise.Apply(stream)

			got, err := flow.Slice[[2]int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Errorf("got %d pairs, expected %d", len(got), len(tt.expected))
				return
			}

			for i, pair := range got {
				if pair != tt.expected[i] {
					t.Errorf("pair %d: got %v, expected %v", i, pair, tt.expected[i])
				}
			}
		})
	}
}

func TestStartWith(t *testing.T) {
	tests := []struct {
		name     string
		prefix   []int
		input    []int
		expected []int
	}{
		{
			name:     "empty prefix",
			prefix:   []int{},
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "single prefix",
			prefix:   []int{0},
			input:    []int{1, 2, 3},
			expected: []int{0, 1, 2, 3},
		},
		{
			name:     "multiple prefix",
			prefix:   []int{-2, -1, 0},
			input:    []int{1, 2, 3},
			expected: []int{-2, -1, 0, 1, 2, 3},
		},
		{
			name:     "empty input",
			prefix:   []int{1, 2},
			input:    []int{},
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			startWith := transform.StartWith(tt.prefix...)
			result := startWith.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestEndWith(t *testing.T) {
	tests := []struct {
		name     string
		suffix   []int
		input    []int
		expected []int
	}{
		{
			name:     "empty suffix",
			suffix:   []int{},
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "single suffix",
			suffix:   []int{4},
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "multiple suffix",
			suffix:   []int{4, 5, 6},
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3, 4, 5, 6},
		},
		{
			name:     "empty input",
			suffix:   []int{1, 2},
			input:    []int{},
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			endWith := transform.EndWith(tt.suffix...)
			result := endWith.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	tests := []struct {
		name         string
		defaultValue int
		input        []int
		expected     []int
	}{
		{
			name:         "empty stream uses default",
			defaultValue: 42,
			input:        []int{},
			expected:     []int{42},
		},
		{
			name:         "non-empty stream ignores default",
			defaultValue: 42,
			input:        []int{1, 2, 3},
			expected:     []int{1, 2, 3},
		},
		{
			name:         "single element ignores default",
			defaultValue: 42,
			input:        []int{1},
			expected:     []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			defaultIfEmpty := transform.DefaultIfEmpty(tt.defaultValue)
			result := defaultIfEmpty.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestConcatMap(t *testing.T) {
	t.Run("basic concatenation", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		// Each number n maps to [n, n*10]
		concatMap := transform.ConcatMap(func(n int) core.Stream[int] {
			return flow.FromSlice([]int{n, n * 10})
		})

		result := concatMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 10, 2, 20, 3, 30}
		if len(got) != len(expected) {
			t.Fatalf("got %v, want %v", got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], expected[i])
			}
		}
	})

	t.Run("empty inner streams", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		concatMap := transform.ConcatMap(func(n int) core.Stream[int] {
			if n == 2 {
				return flow.FromSlice([]int{})
			}
			return flow.FromSlice([]int{n})
		})

		result := concatMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 3}
		if len(got) != len(expected) {
			t.Fatalf("got %v, want %v", got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], expected[i])
			}
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

		count := 0
		concatMap := transform.ConcatMap(func(n int) core.Stream[int] {
			count++
			if count >= 2 {
				cancel()
			}
			return flow.FromSlice([]int{n})
		})

		result := concatMap.Apply(stream)
		_, _ = flow.Slice[int](ctx, result)

		// Should have processed at most 2-3 items before cancellation
		if count > 3 {
			t.Errorf("processed too many items: %d", count)
		}
	})
}

func TestMergeMap(t *testing.T) {
	t.Run("basic merge", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		mergeMap := transform.MergeMap(func(n int) core.Stream[int] {
			return flow.FromSlice([]int{n, n * 10})
		}, 0)

		result := mergeMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// MergeMap doesn't guarantee order, so check length and contents
		if len(got) != 6 {
			t.Errorf("expected 6 elements, got %d", len(got))
		}

		// Verify all expected values are present
		counts := make(map[int]int)
		for _, v := range got {
			counts[v]++
		}

		expected := map[int]int{1: 1, 10: 1, 2: 1, 20: 1, 3: 1, 30: 1}
		for k, v := range expected {
			if counts[k] != v {
				t.Errorf("expected %d to appear %d times, got %d", k, v, counts[k])
			}
		}
	})

	t.Run("with concurrency limit", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

		mergeMap := transform.MergeMap(func(n int) core.Stream[int] {
			return flow.FromSlice([]int{n})
		}, 2) // Max 2 concurrent

		result := mergeMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 5 {
			t.Errorf("expected 5 elements, got %d", len(got))
		}
	})
}

func TestSwitchMap(t *testing.T) {
	t.Run("switches to new stream", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		// Each input creates a stream that emits immediately
		switchMap := transform.SwitchMap(func(n int) core.Stream[int] {
			return flow.FromSlice([]int{n * 100})
		})

		result := switchMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Since we're using immediate streams, all should be processed
		if len(got) < 1 {
			t.Errorf("expected at least 1 element, got %d", len(got))
		}
	})
}

func TestExhaustMap(t *testing.T) {
	t.Run("ignores while processing", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		exhaustMap := transform.ExhaustMap(func(n int) core.Stream[int] {
			return flow.FromSlice([]int{n, n * 10})
		})

		result := exhaustMap.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get at least the first stream's output
		if len(got) < 2 {
			t.Errorf("expected at least 2 elements, got %d", len(got))
		}
	})
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		input    []int
		expected []int
	}{
		{
			name:     "repeat once",
			count:    1,
			input:    []int{1, 2},
			expected: []int{1, 2},
		},
		{
			name:     "repeat twice",
			count:    2,
			input:    []int{1, 2},
			expected: []int{1, 2, 1, 2},
		},
		{
			name:     "repeat three times",
			count:    3,
			input:    []int{1},
			expected: []int{1, 1, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			stream := flow.FromSlice(tt.input)
			repeat := transform.Repeat[int](tt.count)
			result := repeat.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		stream := flow.FromSlice([]int{1, 2, 3})
		repeat := transform.Repeat[int](0) // 0 means infinite
		result := repeat.Apply(stream)

		done := make(chan struct{})
		go func() {
			_, _ = flow.Slice[int](ctx, result)
			close(done)
		}()

		select {
		case <-done:
			// OK
		case <-time.After(200 * time.Millisecond):
			t.Error("did not stop on cancellation")
		}
	})
}

func TestIgnoreElements(t *testing.T) {
	tests := []struct {
		name  string
		input []int
	}{
		{
			name:  "empty stream",
			input: []int{},
		},
		{
			name:  "single element",
			input: []int{1},
		},
		{
			name:  "multiple elements",
			input: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			ignore := transform.IgnoreElements[int]()
			result := ignore.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 0 {
				t.Errorf("expected empty slice, got %v", got)
			}
		})
	}
}

func TestToSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty stream",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "single element",
			input:    []int{1},
			expected: []int{1},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			toSlice := transform.ToSlice[int]()
			result := toSlice.Apply(stream)

			got, err := flow.Slice[[]int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("expected 1 slice, got %d", len(got))
			}

			if len(got[0]) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got[0], tt.expected)
			}
			for i := range got[0] {
				if got[0][i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[0][i], tt.expected[i])
				}
			}
		})
	}
}

func TestToMap(t *testing.T) {
	t.Run("basic map creation", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]string{"a", "bb", "ccc"})

		// ToMap uses keyFn and stores the value itself
		toMap := transform.ToMap(func(s string) int { return len(s) })

		result := toMap.Apply(stream)
		got, err := flow.Slice[map[int]string](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("expected 1 map, got %d", len(got))
		}

		m := got[0]
		if m[1] != "a" || m[2] != "bb" || m[3] != "ccc" {
			t.Errorf("unexpected map: %v", m)
		}
	})

	t.Run("duplicate keys overwrites", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

		// Key by even/odd (0 or 1)
		toMap := transform.ToMap(func(n int) int { return n % 2 })

		result := toMap.Apply(stream)
		got, err := flow.Slice[map[int]int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("expected 1 map, got %d", len(got))
		}

		m := got[0]
		// Last even (n%2==0) is 4, last odd (n%2==1) is 5
		if m[0] != 4 {
			t.Errorf("expected even=4, got %d", m[0])
		}
		if m[1] != 5 {
			t.Errorf("expected odd=5, got %d", m[1])
		}
	})
}

func TestToSet(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty stream",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "no duplicates",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "with duplicates",
			input:    []int{1, 2, 2, 3, 3, 3},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			toSet := transform.ToSet[int]()
			result := toSet.Apply(stream)

			got, err := flow.Slice[map[int]struct{}](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("expected 1 set, got %d", len(got))
			}

			set := got[0]
			if len(set) != len(tt.expected) {
				t.Errorf("expected set size %d, got %d", len(tt.expected), len(set))
			}

			for _, v := range tt.expected {
				if _, exists := set[v]; !exists {
					t.Errorf("expected %d in set", v)
				}
			}
		})
	}
}

func TestDistinct(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty stream",
			input:    []int{},
			expected: nil,
		},
		{
			name:     "no duplicates",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "consecutive duplicates",
			input:    []int{1, 1, 2, 2, 3, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "non-consecutive duplicates",
			input:    []int{1, 2, 3, 1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "all same",
			input:    []int{5, 5, 5, 5, 5},
			expected: []int{5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			distinct := transform.Distinct[int]()
			result := distinct.Apply(stream)

			got, err := flow.Slice[int](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("got %v, want %v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestDistinctBy(t *testing.T) {
	type person struct {
		name string
		age  int
	}

	t.Run("distinct by age", func(t *testing.T) {
		ctx := context.Background()
		input := []person{
			{"Alice", 30},
			{"Bob", 25},
			{"Charlie", 30},
			{"Diana", 25},
			{"Eve", 35},
		}

		stream := flow.FromSlice(input)
		distinctBy := transform.DistinctBy(func(p person) int { return p.age })
		result := distinctBy.Apply(stream)

		got, err := flow.Slice[person](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get first person of each unique age: Alice(30), Bob(25), Eve(35)
		if len(got) != 3 {
			t.Errorf("expected 3 distinct, got %d: %v", len(got), got)
		}
	})
}

func TestWithIndex(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []transform.Indexed[string]
	}{
		{
			name:     "empty stream",
			input:    []string{},
			expected: nil,
		},
		{
			name:  "single element",
			input: []string{"a"},
			expected: []transform.Indexed[string]{
				{Index: 0, Value: "a"},
			},
		},
		{
			name:  "multiple elements",
			input: []string{"a", "b", "c"},
			expected: []transform.Indexed[string]{
				{Index: 0, Value: "a"},
				{Index: 1, Value: "b"},
				{Index: 2, Value: "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			withIndex := transform.WithIndex[string]()
			result := withIndex.Apply(stream)

			got, err := flow.Slice[transform.Indexed[string]](ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Errorf("got %d elements, expected %d", len(got), len(tt.expected))
				return
			}

			for i, idx := range got {
				if idx.Index != tt.expected[i].Index || idx.Value != tt.expected[i].Value {
					t.Errorf("element %d: got %v, expected %v", i, idx, tt.expected[i])
				}
			}
		})
	}
}

func TestRepeatWhen(t *testing.T) {
	t.Run("repeats based on notifier", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		callCount := 0
		stream := flow.FromSlice([]int{1, 2})

		// Notifier that emits 2 times then closes
		repeatWhen := transform.RepeatWhen[int, int](func() core.Stream[int] {
			callCount++
			if callCount <= 2 {
				return flow.FromSlice([]int{1}) // Signal to repeat
			}
			return flow.FromSlice([]int{}) // Empty = stop
		})

		result := repeatWhen.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil && err != context.DeadlineExceeded {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get at least original stream [1, 2]
		if len(got) < 2 {
			t.Errorf("expected at least 2 elements, got %d", len(got))
		}
	})

	t.Run("stops when notifier stream is empty", func(t *testing.T) {
		ctx := context.Background()

		stream := flow.FromSlice([]int{1, 2, 3})

		// Notifier returns empty stream immediately = no repeat
		repeatWhen := transform.RepeatWhen[int, int](func() core.Stream[int] {
			return flow.FromSlice([]int{}) // Empty = stop immediately
		})

		result := repeatWhen.Apply(stream)
		got, err := flow.Slice[int](ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should get exactly the original stream
		expected := []int{1, 2, 3}
		if len(got) != len(expected) {
			t.Fatalf("got %v, want %v", got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], expected[i])
			}
		}
	})
}
