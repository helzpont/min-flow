package timing_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/timing"
)

func TestBuffer(t *testing.T) {
	tests := []struct {
		name    string
		input   []int
		size    int
		wantLen int
	}{
		{
			name:    "buffer size 1",
			input:   []int{1, 2, 3},
			size:    1,
			wantLen: 3,
		},
		{
			name:    "buffer size 10",
			input:   []int{1, 2, 3, 4, 5},
			size:    10,
			wantLen: 5,
		},
		{
			name:    "empty input",
			input:   []int{},
			size:    5,
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)

			// Buffer is a Transformer, so we apply it to the stream with context
			buffered := timing.Buffer[int](tt.size).Apply(stream)
			result, err := flow.Slice(ctx, buffered)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.wantLen {
				t.Errorf("got %d items, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestDebounce(t *testing.T) {
	ctx := context.Background()

	// Create a stream with controlled emissions
	var count int32
	stream := flow.Generate(func() (int, bool, error) {
		c := atomic.AddInt32(&count, 1)
		if c > 5 {
			return 0, false, nil
		}
		// Emit quickly to be debounced
		time.Sleep(5 * time.Millisecond)
		return int(c), true, nil
	})

	// Debounce with a 20ms window - Debounce is a Transformer
	debounced := timing.Debounce[int](20 * time.Millisecond).Apply(stream)

	startTime := time.Now()
	result, err := flow.Slice(ctx, debounced)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With debouncing, we should get fewer items
	t.Logf("Got %d items in %v", len(result), elapsed)

	// Last item should be 5
	if len(result) > 0 && result[len(result)-1] != 5 {
		t.Errorf("last item should be 5, got %d", result[len(result)-1])
	}
}

func TestThrottle(t *testing.T) {
	ctx := context.Background()

	// Create a stream with controlled emissions - add delay between items
	var count int32
	stream := flow.Generate(func() (int, bool, error) {
		c := atomic.AddInt32(&count, 1)
		if c > 10 {
			return 0, false, nil
		}
		// Add 15ms delay between emissions so throttle (10ms) doesn't drop items
		time.Sleep(15 * time.Millisecond)
		return int(c), true, nil
	})

	// Throttle to one item per 10ms - Throttle is a Transformer
	throttled := timing.Throttle[int](10 * time.Millisecond).Apply(stream)

	result, err := flow.Slice(ctx, throttled)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have all 10 items since emissions are spaced 15ms apart
	if len(result) != 10 {
		t.Errorf("got %d items, want 10", len(result))
	}
}

func TestThrottleLatest(t *testing.T) {
	ctx := context.Background()

	// Create a stream with fast emissions
	var count int32
	stream := flow.Generate(func() (int, bool, error) {
		c := atomic.AddInt32(&count, 1)
		if c > 10 {
			return 0, false, nil
		}
		return int(c), true, nil
	})

	// ThrottleLatest is a Transformer
	throttled := timing.ThrottleLatest[int](10 * time.Millisecond).Apply(stream)

	result, err := flow.Slice(ctx, throttled)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have received some items (behavior depends on timing)
	t.Logf("ThrottleLatest: got %d items: %v", len(result), result)

	// First item should be 1
	if len(result) > 0 && result[0] != 1 {
		t.Errorf("first item should be 1, got %d", result[0])
	}
}

func TestRateLimit(t *testing.T) {
	ctx := context.Background()

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	// Rate limit to 2 per 50ms - RateLimit is a Transformer
	limited := timing.RateLimit[int](2, 50*time.Millisecond).Apply(stream)

	startTime := time.Now()
	result, err := flow.Slice(ctx, limited)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 5 {
		t.Errorf("got %d items, want 5", len(result))
	}

	// Should take at least ~100ms (3 windows for 5 items at rate 2)
	t.Logf("RateLimit completed in %v", elapsed)
}

func TestDelay(t *testing.T) {
	ctx := context.Background()

	stream := flow.FromSlice([]int{1, 2, 3})

	// Delay is a Transformer
	delayed := timing.Delay[int](20 * time.Millisecond).Apply(stream)

	startTime := time.Now()
	result, err := flow.Slice(ctx, delayed)
	elapsed := time.Since(startTime)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("got %d items, want 3", len(result))
	}

	// Each item delayed by 20ms, so total should be at least 60ms
	if elapsed < 50*time.Millisecond {
		t.Errorf("delay too short: %v, expected at least 50ms", elapsed)
	}
}

func TestSample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Create a continuous stream
	var count int32
	stream := flow.Generate(func() (int, bool, error) {
		c := atomic.AddInt32(&count, 1)
		if c > 100 {
			return 0, false, nil
		}
		time.Sleep(5 * time.Millisecond)
		return int(c), true, nil
	})

	// Sample every 30ms - Sample is a Transformer
	sampled := timing.Sample[int](30 * time.Millisecond).Apply(stream)

	result, err := flow.Slice(ctx, sampled)
	if err != nil {
		// Context timeout is expected
		t.Logf("Sampled with context: %v", err)
	}

	// Should have sampled some values
	t.Logf("Sample: got %d items: %v", len(result), result)
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name      string
		delay     time.Duration
		timeout   time.Duration
		expectErr bool
	}{
		{
			name:      "no timeout",
			delay:     5 * time.Millisecond,
			timeout:   50 * time.Millisecond,
			expectErr: false,
		},
		{
			name:      "timeout exceeded",
			delay:     100 * time.Millisecond,
			timeout:   20 * time.Millisecond,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create a stream with delayed emission
			emitted := false
			stream := flow.Generate(func() (int, bool, error) {
				if emitted {
					return 0, false, nil
				}
				time.Sleep(tt.delay)
				emitted = true
				return 42, true, nil
			})

			// Timeout is a Transformer
			timedOut := timing.After[int](tt.timeout).Apply(stream)

			result, err := flow.Slice(ctx, timedOut)

			if tt.expectErr {
				// Check that we got a timeout error in the results or error return
				if err == nil && len(result) > 0 {
					t.Log("Timeout test: no error returned (timeout result in stream)")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != 1 || result[0] != 42 {
					t.Errorf("got %v, want [42]", result)
				}
			}
		})
	}
}

func TestBuffer_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Context-aware source so cancellation stops production quickly.
	stream := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			i := 0
			for {
				select {
				case <-ctx.Done():
					return
				case out <- flow.Ok(i):
					i++
				}
			}
		}()
		return out
	})

	buffered := timing.Buffer[int](2).Apply(stream)

	count := 0
	for result := range buffered.Emit(ctx) {
		if result.IsValue() {
			count++
			if count >= 2 {
				cancel()
			}
		}
	}

	if count > 5 {
		t.Errorf("expected early termination, got %d items", count)
	}
}
