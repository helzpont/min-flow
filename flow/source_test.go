package flow_test

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
)

func TestFromSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "single element",
			input:    []int{42},
			expected: []int{42},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3, 4, 5},
			expected: []int{1, 2, 3, 4, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.FromSlice(tt.input)
			result, err := flow.Slice(ctx, stream)
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

func TestFromSlice_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	ch := stream.Emit(ctx)

	// Read first element
	<-ch

	// Cancel context
	cancel()

	// Give goroutine time to notice cancellation
	time.Sleep(10 * time.Millisecond)

	// Channel should be closed or draining
	// Just verify no panic occurs
}

func TestFromChannel(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "empty channel",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "single element",
			input:    []int{42},
			expected: []int{42},
		},
		{
			name:     "multiple elements",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			ch := make(chan int)
			go func() {
				defer close(ch)
				for _, v := range tt.input {
					ch <- v
				}
			}()

			stream := flow.FromChannel(ch)
			result, err := flow.Slice(ctx, stream)
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

func TestFromIter(t *testing.T) {
	tests := []struct {
		name     string
		seq      iter.Seq[int]
		expected []int
	}{
		{
			name: "empty iterator",
			seq: func(yield func(int) bool) {
			},
			expected: []int{},
		},
		{
			name: "single element",
			seq: func(yield func(int) bool) {
				yield(42)
			},
			expected: []int{42},
		},
		{
			name: "multiple elements",
			seq: func(yield func(int) bool) {
				for i := 1; i <= 3; i++ {
					if !yield(i) {
						return
					}
				}
			},
			expected: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.FromIter(tt.seq)
			result, err := flow.Slice(ctx, stream)
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

func TestEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream := flow.Empty[int]()
	result, err := flow.Slice(ctx, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(result))
	}
}

func TestOnce(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		expected int
	}{
		{
			name:     "zero value",
			value:    0,
			expected: 0,
		},
		{
			name:     "positive value",
			value:    42,
			expected: 42,
		},
		{
			name:     "negative value",
			value:    -1,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.Once(tt.value)
			result, err := flow.First(ctx, stream)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	t.Run("generates values until done", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		count := 0
		stream := flow.Generate(func() (int, bool, error) {
			count++
			if count > 5 {
				return 0, false, nil
			}
			return count, true, nil
		})

		result, err := flow.Slice(ctx, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3, 4, 5}
		if len(result) != len(expected) {
			t.Fatalf("expected %d elements, got %d", len(expected), len(result))
		}

		for i, v := range result {
			if v != expected[i] {
				t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
			}
		}
	})

	t.Run("handles errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		expectedErr := errors.New("test error")
		callCount := 0
		stream := flow.Generate(func() (int, bool, error) {
			callCount++
			if callCount == 2 {
				return 0, true, expectedErr
			}
			if callCount > 3 {
				return 0, false, nil
			}
			return callCount, true, nil
		})

		results := flow.Collect(ctx, stream)
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		// First result should be value
		if !results[0].IsValue() || results[0].Value() != 1 {
			t.Errorf("expected value 1, got %v", results[0])
		}

		// Second result should be error
		if !results[1].IsError() {
			t.Errorf("expected error, got value")
		}

		// Third result should be value
		if !results[2].IsValue() || results[2].Value() != 3 {
			t.Errorf("expected value 3, got %v", results[2])
		}
	})
}

func TestRepeat(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		n        int
		expected []int
	}{
		{
			name:     "repeat zero times",
			value:    42,
			n:        0,
			expected: []int{},
		},
		{
			name:     "repeat once",
			value:    42,
			n:        1,
			expected: []int{42},
		},
		{
			name:     "repeat multiple times",
			value:    7,
			n:        3,
			expected: []int{7, 7, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.Repeat(tt.value, tt.n)
			result, err := flow.Slice(ctx, stream)
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

func TestRepeat_Infinite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stream := flow.Repeat(1, -1) // infinite
	ch := stream.Emit(ctx)

	count := 0
	for range ch {
		count++
		if count >= 10 {
			cancel()
		}
	}

	if count < 10 {
		t.Errorf("expected at least 10 elements, got %d", count)
	}
}

func TestRange(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		end      int
		expected []int
	}{
		{
			name:     "empty range (start == end)",
			start:    5,
			end:      5,
			expected: []int{},
		},
		{
			name:     "empty range (start > end)",
			start:    10,
			end:      5,
			expected: []int{},
		},
		{
			name:     "single element",
			start:    0,
			end:      1,
			expected: []int{0},
		},
		{
			name:     "positive range",
			start:    1,
			end:      5,
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "negative to positive",
			start:    -2,
			end:      2,
			expected: []int{-2, -1, 0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.Range(tt.start, tt.end)
			result, err := flow.Slice(ctx, stream)
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

func TestTimer(t *testing.T) {
	t.Run("emits after delay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		stream := flow.Timer(50 * time.Millisecond)
		result, err := flow.First(ctx, stream)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if elapsed < 40*time.Millisecond {
			t.Errorf("expected at least 40ms delay, got %v", elapsed)
		}

		if result.IsZero() {
			t.Error("expected non-zero time value")
		}
	})

	t.Run("cancellation before delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		stream := flow.Timer(time.Second)
		ch := stream.Emit(ctx)

		// Cancel immediately
		cancel()

		// Drain channel - should close without emitting
		count := 0
		for range ch {
			count++
		}

		if count != 0 {
			t.Errorf("expected no values, got %d", count)
		}
	})
}

func TestTimerValue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	stream := flow.TimerValue(10*time.Millisecond, "hello")
	result, err := flow.First(ctx, stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestInterval(t *testing.T) {
	t.Run("emits sequential values", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		stream := flow.Interval(30 * time.Millisecond)
		result, _ := flow.Slice(ctx, stream)

		// Should get at least 3 values in 150ms with 30ms interval
		if len(result) < 3 {
			t.Errorf("expected at least 3 values, got %d", len(result))
		}

		// Values should be sequential starting from 0
		for i, v := range result {
			if v != i {
				t.Errorf("expected value %d at index %d, got %d", i, i, v)
			}
		}
	})
}

func TestIntervalWithDelay(t *testing.T) {
	t.Run("respects initial delay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		stream := flow.IntervalWithDelay(50*time.Millisecond, 30*time.Millisecond)
		result, _ := flow.First(ctx, stream)
		elapsed := time.Since(start)

		if err := ctx.Err(); err == context.DeadlineExceeded {
			t.Fatal("timed out waiting for first value")
		}

		if elapsed < 40*time.Millisecond {
			t.Errorf("expected at least 40ms delay, got %v", elapsed)
		}

		if result != 0 {
			t.Errorf("expected first value to be 0, got %d", result)
		}
	})
}

func TestFromMap(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]int
		count int
	}{
		{
			name:  "empty map",
			input: map[string]int{},
			count: 0,
		},
		{
			name:  "single entry",
			input: map[string]int{"a": 1},
			count: 1,
		},
		{
			name:  "multiple entries",
			input: map[string]int{"a": 1, "b": 2, "c": 3},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.FromMap(tt.input)
			result, err := flow.Slice(ctx, stream)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.count {
				t.Fatalf("expected %d elements, got %d", tt.count, len(result))
			}

			// Verify all key-value pairs are present
			for _, kv := range result {
				expected, ok := tt.input[kv.Key]
				if !ok {
					t.Errorf("unexpected key: %s", kv.Key)
				}
				if kv.Value != expected {
					t.Errorf("key %s: expected value %d, got %d", kv.Key, expected, kv.Value)
				}
			}
		})
	}
}

func TestFromError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	expectedErr := errors.New("test error")
	stream := flow.FromError[int](expectedErr)
	results := flow.Collect(ctx, stream)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if !results[0].IsError() {
		t.Error("expected error result")
	}

	if results[0].Error().Error() != expectedErr.Error() {
		t.Errorf("expected error '%v', got '%v'", expectedErr, results[0].Error())
	}
}

func TestNever(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	stream := flow.Never[int]()
	ch := stream.Emit(ctx)

	// Should receive nothing until context cancellation
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("Never stream should not emit values")
		}
		// Channel closed due to context cancellation - expected
	case <-time.After(100 * time.Millisecond):
		t.Error("channel should have closed after context timeout")
	}
}

func TestDefer(t *testing.T) {
	t.Run("creates stream lazily", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		callCount := 0
		factory := func() flow.Stream[int] {
			callCount++
			return flow.FromSlice([]int{1, 2, 3})
		}

		stream := flow.Defer(factory)

		// Factory should not be called until Emit
		if callCount != 0 {
			t.Errorf("expected factory not to be called yet, but was called %d times", callCount)
		}

		result, err := flow.Slice(ctx, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if callCount != 1 {
			t.Errorf("expected factory to be called once, but was called %d times", callCount)
		}

		if len(result) != 3 {
			t.Errorf("expected 3 elements, got %d", len(result))
		}
	})

	t.Run("calls factory each subscription", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		callCount := 0
		factory := func() flow.Stream[int] {
			callCount++
			return flow.Once(callCount)
		}

		stream := flow.Defer(factory)

		// First subscription
		result1, _ := flow.First(ctx, stream)
		// Second subscription
		result2, _ := flow.First(ctx, stream)

		if callCount != 2 {
			t.Errorf("expected factory to be called twice, but was called %d times", callCount)
		}

		if result1 != 1 || result2 != 2 {
			t.Errorf("expected results 1 and 2, got %d and %d", result1, result2)
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("emits values", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		stream := flow.Create(func(ctx context.Context, emit func(int), emitError func(error)) error {
			emit(1)
			emit(2)
			emit(3)
			return nil
		})

		result, err := flow.Slice(ctx, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3}
		if len(result) != len(expected) {
			t.Fatalf("expected %d elements, got %d", len(expected), len(result))
		}

		for i, v := range result {
			if v != expected[i] {
				t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
			}
		}
	})

	t.Run("handles errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		testErr := errors.New("test error")
		stream := flow.Create(func(ctx context.Context, emit func(int), emitError func(error)) error {
			emit(1)
			emitError(testErr)
			emit(2)
			return nil
		})

		results := flow.Collect(ctx, stream)
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		if !results[0].IsValue() || results[0].Value() != 1 {
			t.Errorf("expected value 1, got %v", results[0])
		}
		if !results[1].IsError() {
			t.Errorf("expected error, got %v", results[1])
		}
		if !results[2].IsValue() || results[2].Value() != 2 {
			t.Errorf("expected value 2, got %v", results[2])
		}
	})

	t.Run("final error is emitted", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		finalErr := errors.New("final error")
		stream := flow.Create(func(ctx context.Context, emit func(int), emitError func(error)) error {
			emit(1)
			return finalErr
		})

		results := flow.Collect(ctx, stream)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		if !results[0].IsValue() || results[0].Value() != 1 {
			t.Errorf("expected value 1, got %v", results[0])
		}
		if !results[1].IsError() || results[1].Error().Error() != finalErr.Error() {
			t.Errorf("expected final error, got %v", results[1])
		}
	})
}

func TestRangeStep(t *testing.T) {
	tests := []struct {
		name     string
		start    int
		end      int
		step     int
		expected []int
	}{
		{
			name:     "positive step",
			start:    0,
			end:      10,
			step:     2,
			expected: []int{0, 2, 4, 6, 8},
		},
		{
			name:     "negative step",
			start:    10,
			end:      0,
			step:     -2,
			expected: []int{10, 8, 6, 4, 2},
		},
		{
			name:     "step of 1",
			start:    1,
			end:      4,
			step:     1,
			expected: []int{1, 2, 3},
		},
		{
			name:     "zero step",
			start:    0,
			end:      5,
			step:     0,
			expected: []int{},
		},
		{
			name:     "invalid direction positive",
			start:    10,
			end:      5,
			step:     1,
			expected: []int{},
		},
		{
			name:     "invalid direction negative",
			start:    5,
			end:      10,
			step:     -1,
			expected: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.RangeStep(tt.start, tt.end, tt.step)
			result, err := flow.Slice(ctx, stream)
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

func TestConcat(t *testing.T) {
	tests := []struct {
		name     string
		streams  []flow.Stream[int]
		expected []int
	}{
		{
			name:     "no streams",
			streams:  []flow.Stream[int]{},
			expected: []int{},
		},
		{
			name:     "single stream",
			streams:  []flow.Stream[int]{flow.FromSlice([]int{1, 2, 3})},
			expected: []int{1, 2, 3},
		},
		{
			name: "multiple streams",
			streams: []flow.Stream[int]{
				flow.FromSlice([]int{1, 2}),
				flow.FromSlice([]int{3, 4}),
				flow.FromSlice([]int{5}),
			},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name: "with empty stream",
			streams: []flow.Stream[int]{
				flow.FromSlice([]int{1}),
				flow.Empty[int](),
				flow.FromSlice([]int{2}),
			},
			expected: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.Concat(tt.streams...)
			result, err := flow.Slice(ctx, stream)
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

func TestStartWith(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		prepend  []int
		expected []int
	}{
		{
			name:     "prepend to empty",
			input:    []int{},
			prepend:  []int{1, 2},
			expected: []int{1, 2},
		},
		{
			name:     "prepend nothing",
			input:    []int{3, 4},
			prepend:  []int{},
			expected: []int{3, 4},
		},
		{
			name:     "prepend values",
			input:    []int{3, 4},
			prepend:  []int{1, 2},
			expected: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.FromSlice(tt.input)
			result := flow.StartWith[int](tt.prepend...).Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(got))
			}

			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("element %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestEndWith(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		append   []int
		expected []int
	}{
		{
			name:     "append to empty",
			input:    []int{},
			append:   []int{1, 2},
			expected: []int{1, 2},
		},
		{
			name:     "append nothing",
			input:    []int{1, 2},
			append:   []int{},
			expected: []int{1, 2},
		},
		{
			name:     "append values",
			input:    []int{1, 2},
			append:   []int{3, 4},
			expected: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.FromSlice(tt.input)
			result := flow.EndWith[int](tt.append...).Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d elements, got %d", len(tt.expected), len(got))
			}

			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("element %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}

func TestFromFunc(t *testing.T) {
	t.Run("generates values until error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		count := 0
		stream := flow.FromFunc(func() (int, error) {
			count++
			if count > 3 {
				return 0, errors.New("done")
			}
			return count, nil
		})

		result, err := flow.Slice(ctx, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3}
		if len(result) != len(expected) {
			t.Fatalf("expected %d elements, got %d", len(expected), len(result))
		}

		for i, v := range result {
			if v != expected[i] {
				t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
			}
		}
	})
}

func TestUnfold(t *testing.T) {
	t.Run("fibonacci sequence", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Generate first 8 Fibonacci numbers
		type state struct{ a, b, count int }
		stream := flow.Unfold(state{0, 1, 0}, func(s state) (int, state, bool, error) {
			if s.count >= 8 {
				return 0, s, false, nil
			}
			return s.a, state{s.b, s.a + s.b, s.count + 1}, true, nil
		})

		result, err := flow.Slice(ctx, stream)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{0, 1, 1, 2, 3, 5, 8, 13}
		if len(result) != len(expected) {
			t.Fatalf("expected %d elements, got %d", len(expected), len(result))
		}

		for i, v := range result {
			if v != expected[i] {
				t.Errorf("element %d: expected %d, got %d", i, expected[i], v)
			}
		}
	})

	t.Run("handles errors", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		testErr := errors.New("test error")
		stream := flow.Unfold(0, func(s int) (int, int, bool, error) {
			if s == 2 {
				return 0, s + 1, true, testErr
			}
			if s >= 4 {
				return 0, s, false, nil
			}
			return s, s + 1, true, nil
		})

		results := flow.Collect(ctx, stream)
		if len(results) != 4 {
			t.Fatalf("expected 4 results, got %d", len(results))
		}

		// 0, 1, error, 3
		if !results[0].IsValue() || results[0].Value() != 0 {
			t.Errorf("expected value 0, got %v", results[0])
		}
		if !results[1].IsValue() || results[1].Value() != 1 {
			t.Errorf("expected value 1, got %v", results[1])
		}
		if !results[2].IsError() {
			t.Errorf("expected error, got %v", results[2])
		}
		if !results[3].IsValue() || results[3].Value() != 3 {
			t.Errorf("expected value 3, got %v", results[3])
		}
	})
}

func TestIterate(t *testing.T) {
	t.Run("generates sequence", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		stream := flow.Iterate(1, func(n int) int { return n * 2 })
		ch := stream.Emit(ctx)

		var result []int
		for r := range ch {
			if r.IsValue() {
				result = append(result, r.Value())
				if len(result) >= 5 {
					cancel()
				}
			}
		}

		expected := []int{1, 2, 4, 8, 16}
		for i, v := range expected {
			if i >= len(result) || result[i] != v {
				t.Errorf("element %d: expected %d", i, v)
			}
		}
	})
}

func TestIterateN(t *testing.T) {
	tests := []struct {
		name     string
		seed     int
		fn       func(int) int
		n        int
		expected []int
	}{
		{
			name:     "powers of 2",
			seed:     1,
			fn:       func(n int) int { return n * 2 },
			n:        5,
			expected: []int{1, 2, 4, 8, 16},
		},
		{
			name:     "zero iterations",
			seed:     1,
			fn:       func(n int) int { return n + 1 },
			n:        0,
			expected: []int{},
		},
		{
			name:     "single iteration",
			seed:     10,
			fn:       func(n int) int { return n + 1 },
			n:        1,
			expected: []int{10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			stream := flow.IterateN(tt.seed, tt.fn, tt.n)
			result, err := flow.Slice(ctx, stream)
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
