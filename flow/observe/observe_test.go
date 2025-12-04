package observe_test

import (
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow/observe"
)

func TestRateMeter(t *testing.T) {
	meter := observe.NewRateMeter(time.Second)

	// Add some items
	for i := 0; i < 10; i++ {
		meter.Add(1)
		time.Sleep(10 * time.Millisecond)
	}

	rate := meter.Rate()
	if rate <= 0 {
		t.Errorf("expected positive rate, got %f", rate)
	}

	if meter.TotalCount() != 10 {
		t.Errorf("expected total count 10, got %d", meter.TotalCount())
	}
}

func TestHistogram(t *testing.T) {
	histogram := observe.NewHistogram[string]()

	histogram.Add("a")
	histogram.Add("b")
	histogram.Add("a")
	histogram.Add("c")
	histogram.Add("a")

	if histogram.Count("a") != 3 {
		t.Errorf("expected count for 'a' = 3, got %d", histogram.Count("a"))
	}
	if histogram.Count("b") != 1 {
		t.Errorf("expected count for 'b' = 1, got %d", histogram.Count("b"))
	}
	if histogram.Total() != 5 {
		t.Errorf("expected total = 5, got %d", histogram.Total())
	}

	counts := histogram.Counts()
	if len(counts) != 3 {
		t.Errorf("expected 3 unique values, got %d", len(counts))
	}
}

func TestLiveMetrics(t *testing.T) {
	metrics := &observe.LiveMetrics{}

	// Initial values should be zero
	if metrics.TotalItems() != 0 {
		t.Errorf("expected TotalItems = 0, got %d", metrics.TotalItems())
	}
	if metrics.ValueCount() != 0 {
		t.Errorf("expected ValueCount = 0, got %d", metrics.ValueCount())
	}
	if metrics.ErrorCount() != 0 {
		t.Errorf("expected ErrorCount = 0, got %d", metrics.ErrorCount())
	}
}

func TestStreamMetricsDuration(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(100 * time.Millisecond)
	metrics := observe.StreamMetrics{
		StartTime: start,
		EndTime:   end,
	}

	duration := metrics.Duration()
	if duration != 100*time.Millisecond {
		t.Errorf("expected duration = 100ms, got %v", duration)
	}
}
