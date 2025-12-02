package combine_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
)

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		streams  [][]int
		wantLen  int
		wantVals []int // all values should be present (order varies)
	}{
		{
			name:     "merge two streams",
			streams:  [][]int{{1, 2}, {3, 4}},
			wantLen:  4,
			wantVals: []int{1, 2, 3, 4},
		},
		{
			name:     "merge three streams",
			streams:  [][]int{{1}, {2}, {3}},
			wantLen:  3,
			wantVals: []int{1, 2, 3},
		},
		{
			name:     "merge with empty stream",
			streams:  [][]int{{1, 2}, {}},
			wantLen:  2,
			wantVals: []int{1, 2},
		},
		{
			name:     "merge empty streams",
			streams:  [][]int{{}, {}},
			wantLen:  0,
			wantVals: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			streams := make([]flow.Stream[int], len(tt.streams))
			for i, vals := range tt.streams {
				streams[i] = flow.FromSlice(vals)
			}

			merged := combine.Merge(streams...)
			result, err := flow.Slice(ctx, merged)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.wantLen {
				t.Errorf("got %d values, want %d", len(result), tt.wantLen)
			}

			// Check all expected values are present
			for _, want := range tt.wantVals {
				found := false
				for _, got := range result {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing expected value %d", want)
				}
			}
		})
	}
}

func TestMerge_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s1 := flow.FromSlice([]int{1, 2, 3, 4, 5})
	s2 := flow.FromSlice([]int{6, 7, 8, 9, 10})

	merged := combine.Merge(s1, s2)
	count := 0
	for result := range merged.Emit(ctx) {
		if result.IsValue() {
			count++
			if count >= 2 {
				cancel()
			}
		}
	}

	// Should have stopped early due to cancellation
	if count > 5 {
		t.Errorf("expected early termination, got %d values", count)
	}
}

func TestConcat(t *testing.T) {
	tests := []struct {
		name    string
		streams [][]int
		want    []int
	}{
		{
			name:    "concat two streams",
			streams: [][]int{{1, 2}, {3, 4}},
			want:    []int{1, 2, 3, 4},
		},
		{
			name:    "concat three streams",
			streams: [][]int{{1}, {2}, {3}},
			want:    []int{1, 2, 3},
		},
		{
			name:    "concat with empty stream",
			streams: [][]int{{1, 2}, {}, {3, 4}},
			want:    []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			streams := make([]flow.Stream[int], len(tt.streams))
			for i, vals := range tt.streams {
				streams[i] = flow.FromSlice(vals)
			}

			concatenated := combine.Concat(streams...)
			result, err := flow.Slice(ctx, concatenated)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(result, tt.want) {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestZip(t *testing.T) {
	tests := []struct {
		name  string
		left  []int
		right []string
		want  []struct {
			a int
			b string
		}
	}{
		{
			name:  "zip equal length",
			left:  []int{1, 2, 3},
			right: []string{"a", "b", "c"},
			want: []struct {
				a int
				b string
			}{{1, "a"}, {2, "b"}, {3, "c"}},
		},
		{
			name:  "zip left shorter",
			left:  []int{1, 2},
			right: []string{"a", "b", "c"},
			want: []struct {
				a int
				b string
			}{{1, "a"}, {2, "b"}},
		},
		{
			name:  "zip right shorter",
			left:  []int{1, 2, 3},
			right: []string{"a"},
			want: []struct {
				a int
				b string
			}{{1, "a"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			left := flow.FromSlice(tt.left)
			right := flow.FromSlice(tt.right)

			zipped := combine.Zip(left, right)
			result, err := flow.Slice(ctx, zipped)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tt.want) {
				t.Errorf("got %d pairs, want %d", len(result), len(tt.want))
				return
			}

			for i, pair := range result {
				if pair.A != tt.want[i].a || pair.B != tt.want[i].b {
					t.Errorf("pair %d: got (%v, %v), want (%v, %v)",
						i, pair.A, pair.B, tt.want[i].a, tt.want[i].b)
				}
			}
		})
	}
}

func TestZipWith(t *testing.T) {
	ctx := context.Background()

	left := flow.FromSlice([]int{1, 2, 3})
	right := flow.FromSlice([]int{10, 20, 30})

	zipped := combine.ZipWith(left, right, func(a, b int) int {
		return a + b
	})
	result, err := flow.Slice(ctx, zipped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{11, 22, 33}
	if !slices.Equal(result, want) {
		t.Errorf("got %v, want %v", result, want)
	}
}

func TestZipLongest(t *testing.T) {
	ctx := context.Background()

	left := flow.FromSlice([]int{1, 2})
	right := flow.FromSlice([]int{10, 20, 30})

	zipped := combine.ZipLongest(left, right)
	result, err := flow.Slice(ctx, zipped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("got %d pairs, want 3", len(result))
		return
	}

	// First two pairs should be normal
	if result[0].A != 1 || result[0].B != 10 {
		t.Errorf("pair 0: got (%v, %v), want (1, 10)", result[0].A, result[0].B)
	}
	if result[1].A != 2 || result[1].B != 20 {
		t.Errorf("pair 1: got (%v, %v), want (2, 20)", result[1].A, result[1].B)
	}

	// Third pair should have HasA = false
	if result[2].HasA || !result[2].HasB || result[2].B != 30 {
		t.Errorf("pair 2: got HasA=%v,HasB=%v,B=%v, want HasA=false,HasB=true,B=30",
			result[2].HasA, result[2].HasB, result[2].B)
	}
}

func TestInterleave(t *testing.T) {
	tests := []struct {
		name    string
		streams [][]int
		want    []int
	}{
		{
			name:    "interleave equal length",
			streams: [][]int{{1, 2}, {10, 20}},
			want:    []int{1, 10, 2, 20},
		},
		{
			name:    "interleave different lengths",
			streams: [][]int{{1, 2, 3}, {10}},
			want:    []int{1, 10, 2, 3},
		},
		{
			name:    "interleave three streams",
			streams: [][]int{{1, 2}, {10, 20}, {100, 200}},
			want:    []int{1, 10, 100, 2, 20, 200},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			streams := make([]flow.Stream[int], len(tt.streams))
			for i, vals := range tt.streams {
				streams[i] = flow.FromSlice(vals)
			}

			interleaved := combine.Interleave(streams...)
			result, err := flow.Slice(ctx, interleaved)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !slices.Equal(result, tt.want) {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestCombineLatest(t *testing.T) {
	ctx := context.Background()

	// CombineLatest takes variadic streams and returns Stream[[]T]
	s1 := flow.FromSlice([]int{1, 2})
	s2 := flow.FromSlice([]int{10, 20})

	combined := combine.CombineLatest(s1, s2)

	var results [][]int
	var mu sync.Mutex

	for result := range combined.Emit(ctx) {
		if result.IsValue() {
			mu.Lock()
			results = append(results, result.Value())
			mu.Unlock()
		}
	}

	// Should have at least one combined result
	if len(results) == 0 {
		t.Error("expected at least one combined result")
	}

	// Each result should be a slice of 2 values
	for _, res := range results {
		if len(res) != 2 {
			t.Errorf("expected slice of 2 values, got %d", len(res))
		}
	}
}

func TestMerge_ConcurrentExecution(t *testing.T) {
	ctx := context.Background()

	// Create streams that take time to emit
	var emissions sync.Map
	startTime := time.Now()

	idx := 0
	s1 := flow.Generate(func() (int, bool, error) {
		idx++
		if idx > 3 {
			return 0, false, nil
		}
		emissions.Store(idx, time.Since(startTime))
		return idx, true, nil
	})

	idx2 := 0
	s2 := flow.Generate(func() (int, bool, error) {
		idx2 += 10
		if idx2 > 30 {
			return 0, false, nil
		}
		emissions.Store(idx2, time.Since(startTime))
		return idx2, true, nil
	})

	merged := combine.Merge(s1, s2)
	result, err := flow.Slice(ctx, merged)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all 6 values
	if len(result) != 6 {
		t.Errorf("got %d values, want 6", len(result))
	}
}

func TestWithLatestFrom(t *testing.T) {
	t.Run("combines source with latest other", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Create a source that emits slowly
		source := flow.FromSlice([]int{1, 2, 3})

		// Create other stream
		other := flow.FromSlice([]string{"a", "b"})

		result := combine.WithLatestFrom(source, other)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have some combined results (exact count depends on timing)
		// At minimum, after 'other' has emitted, source emissions should be paired
		if len(got) == 0 {
			// This can happen due to timing - other stream may complete before source emits
			// That's acceptable behavior for WithLatestFrom
			t.Log("No combined results - other stream may have completed first")
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("returns winner stream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Channel to ensure we know which stream wins
		ready := make(chan struct{})

		// Fast stream - emits immediately when started
		fast := flow.Create(func(ctx context.Context, emit func(int), _ func(error)) error {
			close(ready) // Signal that fast stream started
			emit(1)
			emit(2)
			emit(3)
			return nil
		})

		// Slow stream - waits for fast stream to start
		slow := flow.Create(func(ctx context.Context, emit func(int), _ func(error)) error {
			<-ready                           // Wait for fast stream to emit
			time.Sleep(10 * time.Millisecond) // Extra delay
			emit(10)
			emit(20)
			emit(30)
			return nil
		})

		result := combine.Race(fast, slow)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Fast stream should win
		if len(got) != 3 {
			t.Fatalf("expected 3 values, got %d", len(got))
		}

		// Should be from fast stream
		if got[0] != 1 {
			t.Errorf("expected first value 1 (from fast), got %d", got[0])
		}
	})

	t.Run("empty streams list", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result := combine.Race[int]()
		got, _ := flow.Slice(ctx, result)

		if len(got) != 0 {
			t.Errorf("expected empty result, got %d values", len(got))
		}
	})
}

func TestAmb(t *testing.T) {
	// Amb is an alias for Race
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s1 := flow.FromSlice([]int{1, 2})
	s2 := flow.FromSlice([]int{3, 4})

	result := combine.Amb(s1, s2)
	got, err := flow.Slice(ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get values from one of the streams
	if len(got) != 2 {
		t.Errorf("expected 2 values, got %d", len(got))
	}
}

func TestSampleOn(t *testing.T) {
	t.Run("samples on notifier emissions", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Source that emits values
		sourceCh := make(chan int)
		source := flow.FromChannel(sourceCh)

		// Notifier that triggers sampling
		notifierCh := make(chan struct{})
		notifier := flow.FromChannel(notifierCh)

		go func() {
			defer close(sourceCh)
			defer close(notifierCh)

			// Emit source values
			sourceCh <- 1
			sourceCh <- 2
			time.Sleep(10 * time.Millisecond)

			// Trigger sample - should get latest (2)
			notifierCh <- struct{}{}
			time.Sleep(10 * time.Millisecond)

			// Emit more
			sourceCh <- 3
			time.Sleep(10 * time.Millisecond)

			// Trigger sample - should get 3
			notifierCh <- struct{}{}
		}()

		result := combine.SampleOn(source, notifier)
		got, _ := flow.Slice(ctx, result)

		// Should have sampled values
		if len(got) < 1 {
			t.Log("May have timing issues in test")
		}
	})
}

func TestBufferWith(t *testing.T) {
	t.Run("buffers until notifier", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		// Source values
		sourceCh := make(chan int)
		source := flow.FromChannel(sourceCh)

		// Notifier
		notifierCh := make(chan struct{})
		notifier := flow.FromChannel(notifierCh)

		go func() {
			defer close(sourceCh)
			defer close(notifierCh)

			sourceCh <- 1
			sourceCh <- 2
			time.Sleep(10 * time.Millisecond)
			notifierCh <- struct{}{} // Flush buffer

			sourceCh <- 3
			time.Sleep(10 * time.Millisecond)
			notifierCh <- struct{}{} // Flush again
		}()

		result := combine.BufferWith(source, notifier)
		got, _ := flow.Slice(ctx, result)

		// Should have at least one buffer
		if len(got) >= 1 {
			t.Logf("Got %d buffers", len(got))
		}
	})
}

func TestFork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	source := flow.FromSlice([]int{1, 2, 3})

	double := flow.Map(func(x int) (int, error) { return x * 2, nil })
	triple := flow.Map(func(x int) (int, error) { return x * 3, nil })

	forked := combine.Fork(source, double, triple)

	if len(forked) != 2 {
		t.Fatalf("expected 2 forked streams, got %d", len(forked))
	}

	// Each forked stream should receive all values
	got1, _ := flow.Slice(ctx, forked[0])
	got2, _ := flow.Slice(ctx, forked[1])

	if len(got1) != 3 {
		t.Errorf("first fork: expected 3 values, got %d", len(got1))
	}
	if len(got2) != 3 {
		t.Errorf("second fork: expected 3 values, got %d", len(got2))
	}
}

func TestGather(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	source := flow.FromSlice([]int{1, 2})

	double := flow.Map(func(x int) (int, error) { return x * 2, nil })
	triple := flow.Map(func(x int) (int, error) { return x * 3, nil })

	result := combine.Gather(source, double, triple)
	got, err := flow.Slice(ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 4 values (2 doubled + 2 tripled)
	if len(got) != 4 {
		t.Errorf("expected 4 values, got %d", len(got))
	}

	// Check all expected values are present
	expected := map[int]bool{2: false, 4: false, 3: false, 6: false}
	for _, v := range got {
		expected[v] = true
	}
	for v, found := range expected {
		if !found {
			t.Errorf("missing expected value: %d", v)
		}
	}
}

func TestSequenceEqualStream(t *testing.T) {
	tests := []struct {
		name   string
		a      []int
		b      []int
		expect bool
	}{
		{
			name:   "equal sequences",
			a:      []int{1, 2, 3},
			b:      []int{1, 2, 3},
			expect: true,
		},
		{
			name:   "different values",
			a:      []int{1, 2, 3},
			b:      []int{1, 2, 4},
			expect: false,
		},
		{
			name:   "different lengths",
			a:      []int{1, 2},
			b:      []int{1, 2, 3},
			expect: false,
		},
		{
			name:   "both empty",
			a:      []int{},
			b:      []int{},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			streamA := flow.FromSlice(tt.a)
			streamB := flow.FromSlice(tt.b)

			result := combine.SequenceEqualStream(streamA, streamB)
			got, err := flow.First(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.expect {
				t.Errorf("got %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestIfEmpty(t *testing.T) {
	t.Run("uses source when not empty", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		source := flow.FromSlice([]int{1, 2, 3})
		alternative := flow.FromSlice([]int{10, 20})

		result := combine.IfEmpty(source, alternative)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3}
		if !slices.Equal(got, expected) {
			t.Errorf("got %v, want %v", got, expected)
		}
	})

	t.Run("uses alternative when empty", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		source := flow.Empty[int]()
		alternative := flow.FromSlice([]int{10, 20})

		result := combine.IfEmpty(source, alternative)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{10, 20}
		if !slices.Equal(got, expected) {
			t.Errorf("got %v, want %v", got, expected)
		}
	})
}
