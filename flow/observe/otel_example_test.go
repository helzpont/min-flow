package observe_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/observe"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// Demonstrates wiring typed hooks to OpenTelemetry counters and histograms.
func TestOtelHooksIntegration(t *testing.T) {
	meter := noop.NewMeterProvider().Meter("minflow/observability")

	values, err := meter.Int64Counter("flow.items", metric.WithDescription("count of values"))
	if err != nil {
		t.Fatalf("create values counter: %v", err)
	}
	errors, err := meter.Int64Counter("flow.errors", metric.WithDescription("count of errors"))
	if err != nil {
		t.Fatalf("create errors counter: %v", err)
	}
	latency, err := meter.Int64Histogram("flow.latency_ms", metric.WithDescription("latency between values"))
	if err != nil {
		t.Fatalf("create latency histogram: %v", err)
	}

	var seen atomic.Int64
	var errs atomic.Int64
	var last time.Time

	ctx := context.Background()
	ctx = observe.WithValueHook[int](ctx, func(int) {
		seen.Add(1)
		now := time.Now()
		if !last.IsZero() {
			latency.Record(ctx, now.Sub(last).Milliseconds())
		}
		last = now
		values.Add(ctx, 1)
	})
	ctx = observe.WithErrorHook[int](ctx, func(error) {
		errs.Add(1)
		errors.Add(ctx, 1)
	})

	mapper := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, fmt.Errorf("boom")
		}
		return n * 2, nil
	})

	stream := mapper.Apply(flow.FromSlice([]int{1, 0, 2}))
	results := flow.Collect(ctx, stream)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if seen.Load() != 2 {
		t.Fatalf("expected 2 values, got %d", seen.Load())
	}
	if errs.Load() != 1 {
		t.Fatalf("expected 1 error, got %d", errs.Load())
	}
}
