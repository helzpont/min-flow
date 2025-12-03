package aggregate_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
)

func TestWindowTime(t *testing.T) {
	t.Run("groups items into windows", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

		result := aggregate.WindowTime[int](50*time.Millisecond).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have at least 1 window with all items
		if len(got) < 1 {
			t.Errorf("expected at least 1 window, got %d", len(got))
		}

		// Verify all items are present
		var total int
		for _, w := range got {
			total += len(w)
		}
		if total != 5 {
			t.Errorf("got %d total items, want 5", total)
		}
	})

	t.Run("empty stream", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{})

		result := aggregate.WindowTime[int](50*time.Millisecond).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 0 {
			t.Errorf("got %d windows, want 0", len(got))
		}
	})
}

func TestTumblingWindow(t *testing.T) {
	// TumblingWindow is an alias for WindowTime
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	result := aggregate.TumblingWindow[int](50*time.Millisecond).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least one window with all items
	var total int
	for _, w := range got {
		total += len(w)
	}
	if total != 5 {
		t.Errorf("total items = %d, want 5", total)
	}
}

func TestSessionWindow(t *testing.T) {
	t.Run("groups items within session", func(t *testing.T) {
		ctx := context.Background()

		ch := make(chan int)
		go func() {
			// First session
			ch <- 1
			ch <- 2
			ch <- 3
			time.Sleep(100 * time.Millisecond) // Gap exceeds timeout

			// Second session
			ch <- 4
			ch <- 5
			close(ch)
		}()

		stream := flow.FromChannel(ch)
		result := aggregate.SessionWindow[int](50*time.Millisecond).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("got %d sessions, want 2: %v", len(got), got)
		}

		// First session: 1, 2, 3
		if len(got[0]) != 3 {
			t.Errorf("session[0] has %d items, want 3", len(got[0]))
		}

		// Second session: 4, 5
		if len(got[1]) != 2 {
			t.Errorf("session[1] has %d items, want 2", len(got[1]))
		}
	})

	t.Run("emits on close", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		result := aggregate.SessionWindow[int](1*time.Second).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || len(got[0]) != 3 {
			t.Errorf("got %v, want [[1, 2, 3]]", got)
		}
	})
}

func TestWindowWithBoundary(t *testing.T) {
	ctx := context.Background()

	// Create a boundary stream that emits every 50ms
	boundary := flow.Emitter[int](func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 3; i++ {
				time.Sleep(50 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	// Create source that emits items faster than boundary
	source := flow.Emitter[int](func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 1; i <= 6; i++ {
				time.Sleep(20 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
				}
			}
		}()
		return out
	})

	result := aggregate.WindowWithBoundary[int, int](boundary).Apply(ctx, source)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have windows based on boundary emissions
	if len(got) < 1 {
		t.Errorf("expected at least 1 window, got %d", len(got))
	}
}

func TestGroupByTime(t *testing.T) {
	ctx := context.Background()

	// Use a fixed base time that aligns with our bucket size for predictability
	base := time.Unix(0, 0) // Epoch time for deterministic testing
	type Item struct {
		Value int
		Time  time.Time
	}

	items := []Item{
		{1, base.Add(0 * time.Millisecond)},   // bucket 0 [0-50)
		{2, base.Add(10 * time.Millisecond)},  // bucket 0 [0-50)
		{3, base.Add(100 * time.Millisecond)}, // bucket 2 [100-150)
		{4, base.Add(110 * time.Millisecond)}, // bucket 2 [100-150)
	}

	stream := flow.FromSlice(items)

	keyFn := func(item Item) time.Time {
		return item.Time
	}

	result := aggregate.GroupByTime(keyFn, 50*time.Millisecond).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should group into 2 time buckets: items 1,2 and items 3,4
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2", len(got))
	}

	if len(got[0]) != 2 {
		t.Errorf("group[0] has %d items, want 2", len(got[0]))
	}
	if len(got[1]) != 2 {
		t.Errorf("group[1] has %d items, want 2", len(got[1]))
	}
}

func TestHoppingWindow(t *testing.T) {
	t.Run("overlapping windows", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
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
				time.Sleep(20 * time.Millisecond)
			}
		}()

		stream := flow.FromChannel(ch)
		// 100ms window, emit every 50ms
		result := aggregate.HoppingWindow[int](100*time.Millisecond, 50*time.Millisecond).Apply(ctx, stream)
		got, _ := flow.Slice(ctx, result)

		// Should have overlapping windows
		if len(got) < 2 {
			t.Errorf("expected at least 2 windows, got %d", len(got))
		}
	})
}

func TestWindowTimeWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emitter[int](func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Ok(2)
			out <- flow.Err[int](testErr)
			out <- flow.Ok(3)
		}()
		return out
	})

	result := aggregate.WindowTime[int](100*time.Millisecond).Apply(ctx, emitter)

	var windows [][]int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			windows = append(windows, res.Value())
		}
	}

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}

	// Should have window with items
	if len(windows) < 1 {
		t.Errorf("got %d windows, want at least 1", len(windows))
	}
}

func TestWindowTimeContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan int)
	go func() {
		for i := 0; ; i++ {
			select {
			case <-ctx.Done():
				close(ch)
				return
			case ch <- i:
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	stream := flow.FromChannel(ch)
	result := aggregate.WindowTime[int](50*time.Millisecond).Apply(ctx, stream)
	outCh := result.Emit(ctx)

	// Get one window
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
		t.Error("window did not stop on cancellation")
	}
}

func TestSessionWindowWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emitter[int](func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Err[int](testErr)
			out <- flow.Ok(2)
			time.Sleep(100 * time.Millisecond)
		}()
		return out
	})

	result := aggregate.SessionWindow[int](50*time.Millisecond).Apply(ctx, emitter)

	var windows [][]int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			windows = append(windows, res.Value())
		}
	}
	_ = windows

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestGroupByTimeWithErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	now := time.Now()
	type Item struct {
		Value int
		Time  time.Time
	}

	emitter := flow.Emitter[Item](func(ctx context.Context) <-chan flow.Result[Item] {
		out := make(chan flow.Result[Item])
		go func() {
			defer close(out)
			out <- flow.Ok(Item{1, now})
			out <- flow.Err[Item](testErr)
			out <- flow.Ok(Item{2, now.Add(100 * time.Millisecond)})
		}()
		return out
	})

	keyFn := func(item Item) time.Time {
		return item.Time
	}

	result := aggregate.GroupByTime(keyFn, 50*time.Millisecond).Apply(ctx, emitter)

	var groups [][]Item
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			groups = append(groups, res.Value())
		}
	}
	_ = groups

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestHoppingWindowWithErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	testErr := errors.New("test error")

	emitter := flow.Emitter[int](func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 5; i++ {
				if i == 2 {
					out <- flow.Err[int](testErr)
				} else {
					out <- flow.Ok(i)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
		return out
	})

	result := aggregate.HoppingWindow[int](100*time.Millisecond, 50*time.Millisecond).Apply(ctx, emitter)

	var windows [][]int
	var errCount int
	for res := range result.Emit(ctx) {
		if res.IsError() {
			errCount++
		} else {
			windows = append(windows, res.Value())
		}
	}
	_ = windows

	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}
