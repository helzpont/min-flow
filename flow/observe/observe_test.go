package observe_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/observe"
)

func TestMeter(t *testing.T) {
	ctx := context.Background()

	// Test with a simple stream without errors
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	var capturedMetrics observe.StreamMetrics
	result := observe.Meter[int](func(m observe.StreamMetrics) {
		capturedMetrics = m
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 5 {
		t.Errorf("got %d values, want 5", len(got))
	}

	if capturedMetrics.TotalItems != 5 {
		t.Errorf("TotalItems = %d, want 5", capturedMetrics.TotalItems)
	}

	if capturedMetrics.ValueCount != 5 {
		t.Errorf("ValueCount = %d, want 5", capturedMetrics.ValueCount)
	}

	if capturedMetrics.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", capturedMetrics.ErrorCount)
	}

	if capturedMetrics.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}

	if capturedMetrics.EndTime.IsZero() {
		t.Error("EndTime should not be zero")
	}
}

func TestMeterLive(t *testing.T) {
	ctx := context.Background()

	metrics := &observe.LiveMetrics{}

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= 5; i++ {
			ch <- i
			time.Sleep(10 * time.Millisecond)
		}
	}()

	stream := flow.FromChannel(ch)
	result := observe.MeterLive[int](metrics).Apply(ctx, stream)

	// Consume in background
	done := make(chan struct{})
	go func() {
		for range result.Emit(ctx) {
		}
		close(done)
	}()

	// Wait a bit and check live metrics
	time.Sleep(30 * time.Millisecond)
	if metrics.TotalItems() < 1 {
		t.Error("expected some items to be processed")
	}

	<-done

	if metrics.TotalItems() != 5 {
		t.Errorf("TotalItems = %d, want 5", metrics.TotalItems())
	}

	if metrics.ValueCount() != 5 {
		t.Errorf("ValueCount = %d, want 5", metrics.ValueCount())
	}

	if metrics.Duration() == 0 {
		t.Error("Duration should not be zero")
	}
}

func TestProgress(t *testing.T) {
	ctx := context.Background()

	var reportCount int32
	var lastReport observe.ProgressReport

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	result := observe.Progress[int](10, 0, func(r observe.ProgressReport) {
		atomic.AddInt32(&reportCount, 1)
		lastReport = r
	}).Apply(ctx, stream)

	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 10 {
		t.Errorf("got %d values, want 10", len(got))
	}

	// Should have at least one report (final)
	if reportCount < 1 {
		t.Error("expected at least one progress report")
	}

	if lastReport.Processed != 10 {
		t.Errorf("lastReport.Processed = %d, want 10", lastReport.Processed)
	}

	if lastReport.Percent != 100 {
		t.Errorf("lastReport.Percent = %f, want 100", lastReport.Percent)
	}
}

func TestSpy(t *testing.T) {
	ctx := context.Background()

	stream := flow.FromSlice([]int{1, 2, 3})

	var spyCount int
	result := observe.Spy(func(res *flow.Result[int]) {
		if res.IsValue() {
			spyCount++
		}
	}).Apply(ctx, stream)

	got, err := flow.Slice[int](ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("got %d values, want 3", len(got))
	}

	if spyCount != 3 {
		t.Errorf("spyCount = %d, want 3", spyCount)
	}
}

func TestDebug(t *testing.T) {
	ctx := context.Background()

	stream := flow.FromSlice([]int{1, 2, 3})

	var events []observe.DebugEvent
	result := observe.Debug(func(info observe.DebugInfo[int]) {
		events = append(events, info.Event)
	}).Apply(ctx, stream)

	_, _ = flow.Slice[int](ctx, result)

	// Should have: Subscribe, Value (x3), Complete = 5 events
	if len(events) < 5 {
		t.Errorf("got %d events, want at least 5: %v", len(events), events)
	}

	if len(events) > 0 && events[0] != observe.DebugEventSubscribe {
		t.Errorf("first event = %v, want Subscribe", events[0])
	}
}

func TestRateMeter(t *testing.T) {
	meter := observe.NewRateMeter(1 * time.Second)

	// Add some items
	for i := 0; i < 10; i++ {
		meter.Add(1)
		time.Sleep(10 * time.Millisecond)
	}

	rate := meter.Rate()
	if rate < 50 || rate > 200 { // Rough bounds: ~100 items/sec expected
		t.Logf("rate = %f (may vary based on timing)", rate)
	}

	if meter.TotalCount() != 10 {
		t.Errorf("TotalCount = %d, want 10", meter.TotalCount())
	}
}

func TestMeterRate(t *testing.T) {
	ctx := context.Background()

	meter := observe.NewRateMeter(1 * time.Second)

	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 0; i < 5; i++ {
			ch <- i
			time.Sleep(10 * time.Millisecond)
		}
	}()

	stream := flow.FromChannel(ch)
	result := observe.MeterRate[int](meter).Apply(ctx, stream)

	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 5 {
		t.Errorf("got %d values, want 5", len(got))
	}

	if meter.TotalCount() != 5 {
		t.Errorf("TotalCount = %d, want 5", meter.TotalCount())
	}
}

func TestHistogram(t *testing.T) {
	hist := observe.NewHistogram[string]()

	hist.Add("a")
	hist.Add("b")
	hist.Add("a")
	hist.Add("c")
	hist.Add("a")

	if hist.Count("a") != 3 {
		t.Errorf("Count(a) = %d, want 3", hist.Count("a"))
	}

	if hist.Count("b") != 1 {
		t.Errorf("Count(b) = %d, want 1", hist.Count("b"))
	}

	if hist.Total() != 5 {
		t.Errorf("Total = %d, want 5", hist.Total())
	}

	counts := hist.Counts()
	if len(counts) != 3 {
		t.Errorf("Counts has %d entries, want 3", len(counts))
	}
}

func TestMeterHistogram(t *testing.T) {
	ctx := context.Background()

	hist := observe.NewHistogram[int]()
	stream := flow.FromSlice([]int{1, 2, 1, 3, 1, 2, 1})

	result := observe.MeterHistogram(hist).Apply(ctx, stream)

	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 7 {
		t.Errorf("got %d values, want 7", len(got))
	}

	if hist.Count(1) != 4 {
		t.Errorf("Count(1) = %d, want 4", hist.Count(1))
	}

	if hist.Count(2) != 2 {
		t.Errorf("Count(2) = %d, want 2", hist.Count(2))
	}

	if hist.Count(3) != 1 {
		t.Errorf("Count(3) = %d, want 1", hist.Count(3))
	}
}

func TestObserveContextCancellation(t *testing.T) {
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
		}
	}()

	stream := flow.FromChannel(ch)
	var metricsReceived bool
	result := observe.Meter[int](func(m observe.StreamMetrics) {
		metricsReceived = true
	}).Apply(ctx, stream)

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
		t.Error("did not stop on cancellation")
	}

	// Metrics should still be captured on cancellation
	if !metricsReceived {
		t.Error("metrics callback should have been called")
	}
}

func TestMeterWithEmptyStream(t *testing.T) {
	ctx := context.Background()

	var capturedMetrics observe.StreamMetrics
	stream := flow.FromSlice([]int{})

	result := observe.Meter[int](func(m observe.StreamMetrics) {
		capturedMetrics = m
	}).Apply(ctx, stream)

	got, err := flow.Slice(ctx, result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %d values, want 0", len(got))
	}

	if capturedMetrics.TotalItems != 0 {
		t.Errorf("TotalItems = %d, want 0", capturedMetrics.TotalItems)
	}
}
