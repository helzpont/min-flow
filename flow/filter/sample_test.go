package filter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func TestSampleWith(t *testing.T) {
	ctx := context.Background()

	// Create a sampler stream that emits periodically
	sampler := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 3; i++ {
				time.Sleep(30 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	// Create source that emits faster than sampler
	source := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 1; i <= 10; i++ {
				time.Sleep(10 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	result := filter.SampleWith[int, int](sampler).Apply(ctx, source)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get sampled values when sampler emits
	if len(got) < 1 {
		t.Errorf("expected at least 1 sampled value, got %d", len(got))
	}
}

func TestSampleWithPreservesErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	sampler := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 3; i++ {
				time.Sleep(30 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	source := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			time.Sleep(15 * time.Millisecond)
			out <- flow.Err[int](testErr)
			time.Sleep(15 * time.Millisecond)
			out <- flow.Ok(2)
			time.Sleep(50 * time.Millisecond)
		}()
		return out
	})

	result := filter.SampleWith[int, int](sampler).Apply(ctx, source)

	var values []int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			values = append(values, res.Value())
		}
	}
	_ = values

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestAuditTime(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= 10; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- i:
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	stream := flow.FromChannel(ch)
	result := filter.AuditTime[int](50*time.Millisecond).Apply(ctx, stream)
	got, _ := flow.Slice(ctx, result)

	// Should emit the most recent value periodically
	if len(got) < 1 {
		t.Errorf("expected at least 1 audited value, got %d", len(got))
	}
}

func TestThrottleLast(t *testing.T) {
	ctx := context.Background()

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= 5; i++ {
			ch <- i
			time.Sleep(10 * time.Millisecond)
		}
		time.Sleep(60 * time.Millisecond) // Wait past throttle period
		for i := 6; i <= 10; i++ {
			ch <- i
			time.Sleep(10 * time.Millisecond)
		}
	}()

	stream := flow.FromChannel(ch)
	result := filter.ThrottleLast[int](50*time.Millisecond).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should emit last value of each throttle window
	if len(got) < 1 {
		t.Errorf("expected at least 1 value, got %d: %v", len(got), got)
	}
}

func TestDistinctUntilChanged(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{
			name:  "removes consecutive duplicates",
			input: []int{1, 1, 2, 2, 3, 3, 2, 2, 1},
			want:  []int{1, 2, 3, 2, 1},
		},
		{
			name:  "no duplicates",
			input: []int{1, 2, 3, 4, 5},
			want:  []int{1, 2, 3, 4, 5},
		},
		{
			name:  "all same",
			input: []int{1, 1, 1, 1},
			want:  []int{1},
		},
		{
			name:  "empty",
			input: []int{},
			want:  []int{},
		},
		{
			name:  "single",
			input: []int{42},
			want:  []int{42},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)

			result := filter.DistinctUntilChanged[int]().Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d] = %d, want %d", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestDistinctUntilChangedBy(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	ctx := context.Background()
	input := []Person{
		{"Alice", 30},
		{"Bob", 30},
		{"Charlie", 25},
		{"Dave", 25},
		{"Eve", 30},
	}

	stream := flow.FromSlice(input)

	// Compare by age
	keyFn := func(p Person) int { return p.Age }

	result := filter.DistinctUntilChangedBy(keyFn).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should keep: Alice(30), Charlie(25), Eve(30)
	if len(got) != 3 {
		t.Errorf("got %d items, want 3", len(got))
	}

	if len(got) >= 3 {
		if got[0].Name != "Alice" || got[1].Name != "Charlie" || got[2].Name != "Eve" {
			t.Errorf("got %v", got)
		}
	}
}

func TestDistinctUntilChangedWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Ok(1) // Duplicate
			out <- flow.Err[int](testErr)
			out <- flow.Ok(2)
			out <- flow.Ok(2) // Duplicate
		}()
		return out
	})

	result := filter.DistinctUntilChanged[int]().Apply(ctx, emitter)

	var values []int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			values = append(values, res.Value())
		}
	}

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}

	want := []int{1, 2}
	if len(values) != len(want) {
		t.Errorf("got %v, want %v", values, want)
	}
}

func TestRandomSample(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Sample 50%
	result := filter.RandomSample[int](0.5).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get roughly half, but with randomness
	// Just check it's less than original
	if len(got) > 10 {
		t.Errorf("got more items than input: %d", len(got))
	}
}

func TestRandomSampleEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("rate 0 takes nothing", func(t *testing.T) {
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		result := filter.RandomSample[int](0).Apply(ctx, stream)
		got, _ := flow.Slice(ctx, result)

		if len(got) != 0 {
			t.Errorf("got %d items, want 0", len(got))
		}
	})

	t.Run("rate 1 takes everything", func(t *testing.T) {
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		result := filter.RandomSample[int](1.0).Apply(ctx, stream)
		got, _ := flow.Slice(ctx, result)

		if len(got) != 5 {
			t.Errorf("got %d items, want 5", len(got))
		}
	})
}

func TestReservoirSample(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Sample 3 items
	result := filter.ReservoirSample[int](3).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ReservoirSample emits a single slice at the end
	if len(got) != 1 {
		t.Errorf("got %d results, want 1 slice", len(got))
	}

	if len(got) > 0 && len(got[0]) != 3 {
		t.Errorf("got %d items in reservoir, want 3", len(got[0]))
	}

	// All items should be from original
	if len(got) > 0 {
		for _, v := range got[0] {
			if v < 1 || v > 10 {
				t.Errorf("got unexpected value: %d", v)
			}
		}
	}
}

func TestReservoirSampleSmallInput(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2})

	// Sample more than available
	result := filter.ReservoirSample[int](5).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get all available
	if len(got) != 1 {
		t.Errorf("got %d results, want 1", len(got))
	}
	if len(got) > 0 && len(got[0]) != 2 {
		t.Errorf("got %d items, want 2", len(got[0]))
	}
}

func TestEveryNth(t *testing.T) {
	tests := []struct {
		name  string
		n     int
		input []int
		want  []int
	}{
		{
			name:  "every 2nd",
			n:     2,
			input: []int{1, 2, 3, 4, 5, 6},
			want:  []int{2, 4, 6},
		},
		{
			name:  "every 3rd",
			n:     3,
			input: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
			want:  []int{3, 6, 9},
		},
		{
			name:  "n=1 takes all",
			n:     1,
			input: []int{1, 2, 3},
			want:  []int{1, 2, 3},
		},
		{
			name:  "n larger than input",
			n:     10,
			input: []int{1, 2, 3},
			want:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)

			result := filter.EveryNth[int](tt.n).Apply(ctx, stream)
			got, err := flow.Slice(ctx, result)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d] = %d, want %d", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestTakeEvery(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})

	// TakeEvery is alias for EveryNth
	result := filter.TakeEvery[int](2).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []int{2, 4, 6}
	if len(got) != len(want) {
		t.Errorf("got %v, want %v", got, want)
		return
	}

	for i, v := range got {
		if v != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, v, want[i])
		}
	}
}

func TestEveryNthWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 1; i <= 6; i++ {
				if i == 3 {
					out <- flow.Err[int](testErr)
				} else {
					out <- flow.Ok(i)
				}
			}
		}()
		return out
	})

	result := filter.EveryNth[int](2).Apply(ctx, emitter)

	var values []int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			values = append(values, res.Value())
		}
	}
	_ = values

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}

	// Should get 2, 4, 6 (every 2nd, error doesn't count as position)
	// The exact behavior depends on implementation
}

func TestSampleWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	sampler := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for {
				time.Sleep(20 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(1):
				}
			}
		}()
		return out
	})

	source := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	result := filter.SampleWith[int, int](sampler).Apply(ctx, source)
	outCh := result.Emit(ctx)

	// Get a few items
	<-outCh
	<-outCh

	// Cancel
	cancel()

	// Should complete quickly
	done := make(chan struct{})
	go func() {
		for range outCh {
		}
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(100 * time.Millisecond):
		t.Error("sampler did not stop on cancellation")
	}
}

func TestReservoirSampleWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 1; i <= 10; i++ {
				if i == 5 {
					out <- flow.Err[int](testErr)
				} else {
					out <- flow.Ok(i)
				}
			}
		}()
		return out
	})

	result := filter.ReservoirSample[int](3).Apply(ctx, emitter)

	var slices [][]int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			slices = append(slices, res.Value())
		}
	}
	_ = slices

	// Errors should be passed through
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestThrottleLastWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Ok(2)
			out <- flow.Err[int](testErr)
			out <- flow.Ok(3)
			time.Sleep(60 * time.Millisecond)
		}()
		return out
	})

	result := filter.ThrottleLast[int](50*time.Millisecond).Apply(ctx, emitter)

	var values []int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			values = append(values, res.Value())
		}
	}
	_ = values

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestAuditTimeWithErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 1; i <= 10; i++ {
				if i == 5 {
					out <- flow.Err[int](testErr)
				} else {
					out <- flow.Ok(i)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
		return out
	})

	result := filter.AuditTime[int](50*time.Millisecond).Apply(ctx, emitter)

	var values []int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			values = append(values, res.Value())
		}
	}
	_ = values

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}
