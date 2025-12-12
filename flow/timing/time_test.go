package timing_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/timing"
)

func TestThrottleWithTrailing(t *testing.T) {
	t.Run("emits trailing value", func(t *testing.T) {
		ctx := context.Background()

		ch := make(chan int)
		go func() {
			ch <- 1
			ch <- 2
			ch <- 3
			time.Sleep(100 * time.Millisecond)
			close(ch)
		}()

		stream := flow.FromChannel(ch)
		result := timing.ThrottleWithTrailing[int](50*time.Millisecond).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should emit 1 (first) and 3 (trailing)
		if len(got) < 1 {
			t.Errorf("got %d items, want at least 1", len(got))
		}
		if got[0] != 1 {
			t.Errorf("first item = %d, want 1", got[0])
		}
	})

	t.Run("emits trailing on close", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]int{1, 2, 3})

		result := timing.ThrottleWithTrailing[int](1*time.Second).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should emit 1 (first, immediately) and 3 (trailing, on close)
		if len(got) < 2 {
			t.Errorf("got %d items, want at least 2", len(got))
		}
	})
}

func TestTimeoutWithError(t *testing.T) {
	t.Run("custom timeout error", func(t *testing.T) {
		ctx := context.Background()
		customErr := errors.New("custom timeout")

		ch := make(chan int)
		go func() {
			time.Sleep(200 * time.Millisecond)
			close(ch)
		}()

		stream := flow.FromChannel(ch)
		result := timing.AfterWithError[int](50*time.Millisecond, customErr).Apply(ctx, stream)
		_, err := flow.Slice(ctx, result)

		if !errors.Is(err, customErr) {
			t.Errorf("got error %v, want %v", err, customErr)
		}
	})

	t.Run("passes through on fast stream", func(t *testing.T) {
		ctx := context.Background()
		customErr := errors.New("custom timeout")
		stream := flow.FromSlice([]int{1, 2, 3})

		result := timing.AfterWithError[int](1*time.Second, customErr).Apply(ctx, stream)
		got, err := flow.Slice(ctx, result)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []int{1, 2, 3}
		if len(got) != len(expected) {
			t.Fatalf("got %d items, want %d", len(got), len(expected))
		}
	})
}

func TestInterval(t *testing.T) {
	t.Run("emits sequential integers", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()

		stream := timing.Interval(50 * time.Millisecond)
		got, _ := flow.Slice(ctx, stream)

		// Should emit approximately 4-5 values in 250ms at 50ms intervals
		if len(got) < 3 || len(got) > 6 {
			t.Errorf("got %d items, expected 3-6", len(got))
		}

		// Values should be sequential starting from 0
		for i, v := range got {
			if v != i {
				t.Errorf("got[%d] = %d, want %d", i, v, i)
			}
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		stream := timing.Interval(10 * time.Millisecond)
		ch := stream.Emit(ctx)

		var count int32
		done := make(chan struct{})
		go func() {
			for range ch {
				atomic.AddInt32(&count, 1)
			}
			close(done)
		}()

		time.Sleep(50 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Error("interval did not stop on cancellation")
		}

		if atomic.LoadInt32(&count) < 2 {
			t.Errorf("expected at least 2 emissions, got %d", count)
		}
	})
}

func TestTimer(t *testing.T) {
	t.Run("emits single value after delay", func(t *testing.T) {
		ctx := context.Background()

		start := time.Now()
		stream := timing.Once(50 * time.Millisecond)
		got, err := flow.Slice(ctx, stream)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if elapsed < 50*time.Millisecond {
			t.Errorf("elapsed %v < 50ms", elapsed)
		}

		if len(got) != 1 || got[0] != 0 {
			t.Errorf("got %v, want [0]", got)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		stream := timing.Once(1 * time.Second)
		ch := stream.Emit(ctx)

		cancel()

		// Should complete quickly after cancellation
		select {
		case <-ch:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Error("timer did not respond to cancellation")
		}
	})
}

func TestTimerWithValue(t *testing.T) {
	ctx := context.Background()

	stream := timing.OnceWith(50*time.Millisecond, "hello")
	got, err := flow.Slice(ctx, stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("got %v, want [hello]", got)
	}
}

func TestTimestamp(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	start := time.Now()
	result := timing.Stamped[int]().Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}

	for i, ts := range got {
		if ts.Value != i+1 {
			t.Errorf("got[%d].Value = %d, want %d", i, ts.Value, i+1)
		}
		if ts.Timestamp.Before(start) {
			t.Errorf("timestamp %v is before start %v", ts.Timestamp, start)
		}
	}
}

func TestTimestampPassesErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Err[int](testErr)
			out <- flow.Ok(2)
		}()
		return out
	})

	result := timing.Stamped[int]().Apply(ctx, emitter)

	var values []int
	var gotErr error
	for res := range result.Emit(ctx) {
		if res.IsError() {
			gotErr = res.Error()
		} else {
			values = append(values, res.Value().Value)
		}
	}

	if gotErr == nil || gotErr.Error() != testErr.Error() {
		t.Errorf("expected error %v, got %v", testErr, gotErr)
	}

	expected := []int{1, 2}
	if len(values) != len(expected) {
		t.Fatalf("got %d values, want %d", len(values), len(expected))
	}
}

func TestTimeIntervalOp(t *testing.T) {
	ctx := context.Background()

	ch := make(chan int)
	go func() {
		ch <- 1
		time.Sleep(50 * time.Millisecond)
		ch <- 2
		time.Sleep(50 * time.Millisecond)
		ch <- 3
		close(ch)
	}()

	stream := flow.FromChannel(ch)
	result := timing.Elapsed[int]().Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}

	// Second and third items should have intervals of approximately 50ms
	for i := 1; i < len(got); i++ {
		if got[i].Interval < 40*time.Millisecond || got[i].Interval > 100*time.Millisecond {
			t.Errorf("got[%d].Interval = %v, expected ~50ms", i, got[i].Interval)
		}
	}
}

func TestDelayWhen(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	delayFn := func(v int) time.Duration {
		return time.Duration(v*10) * time.Millisecond
	}

	start := time.Now()
	result := timing.DelayWhen(delayFn).Apply(ctx, stream)
	got, err := flow.Slice(ctx, result)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total delay should be at least 10+20+30 = 60ms
	if elapsed < 60*time.Millisecond {
		t.Errorf("elapsed %v < 60ms", elapsed)
	}

	expected := []int{1, 2, 3}
	if len(got) != len(expected) {
		t.Fatalf("got %d items, want %d", len(got), len(expected))
	}
	for i, v := range got {
		if v != expected[i] {
			t.Errorf("got[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestDelayWhenPassesErrors(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("test error")

	emitter := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			out <- flow.Ok(1)
			out <- flow.Err[int](testErr)
			out <- flow.Ok(2)
		}()
		return out
	})

	delayFn := func(v int) time.Duration {
		return 10 * time.Millisecond
	}

	result := timing.DelayWhen(delayFn).Apply(ctx, emitter)

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

	if len(values) != 2 {
		t.Errorf("got %d values, want 2", len(values))
	}
}

func TestDelayWhenCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	delayFn := func(v int) time.Duration {
		return 100 * time.Millisecond
	}

	result := timing.DelayWhen(delayFn).Apply(ctx, stream)
	ch := result.Emit(ctx)

	// Get first item
	<-ch

	// Cancel during delay
	cancel()

	// Should complete
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(200 * time.Millisecond):
		t.Error("did not respond to cancellation")
	}
}
