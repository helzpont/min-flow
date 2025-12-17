package observe

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
)

func TestMetricsInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		input      []int
		wantTotal  int64
		wantValues int64
	}{
		{
			name:       "counts values",
			input:      []int{1, 2, 3, 4, 5},
			wantTotal:  5,
			wantValues: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMetrics StreamMetrics
			interceptor := NewMetricsInterceptor(func(m StreamMetrics) {
				gotMetrics = m
			})

			ctx, registry := core.WithRegistry(context.Background())
			if err := registry.Register(interceptor); err != nil {
				t.Fatalf("failed to register interceptor: %v", err)
			}

			stream := flow.FromSlice(tt.input)
			intercepted := core.Intercept[int]().Apply(stream)

			for range intercepted.All(ctx) {
			}

			if gotMetrics.TotalItems != tt.wantTotal {
				t.Errorf("TotalItems = %d, want %d", gotMetrics.TotalItems, tt.wantTotal)
			}
			if gotMetrics.ValueCount != tt.wantValues {
				t.Errorf("ValueCount = %d, want %d", gotMetrics.ValueCount, tt.wantValues)
			}
		})
	}
}

func TestLiveMetricsInterceptor(t *testing.T) {
	metrics := &LiveMetrics{}
	interceptor := NewLiveMetricsInterceptor(metrics)

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
	intercepted := core.Intercept[int]().Apply(stream)

	for range intercepted.All(ctx) {
	}

	if got := metrics.TotalItems(); got != 5 {
		t.Errorf("TotalItems() = %d, want 5", got)
	}
	if got := metrics.ValueCount(); got != 5 {
		t.Errorf("ValueCount() = %d, want 5", got)
	}
}

func TestCounterInterceptor(t *testing.T) {
	counter := NewCounterInterceptor()

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(counter); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	stream := flow.FromSlice([]int{1, 2, 3})
	intercepted := core.Intercept[int]().Apply(stream)

	for range intercepted.All(ctx) {
	}

	if got := counter.Values(); got != 3 {
		t.Errorf("Values() = %d, want 3", got)
	}
	if got := counter.Total(); got != 3 {
		t.Errorf("Total() = %d, want 3", got)
	}
}

func TestCallbackInterceptor(t *testing.T) {
	var startCalled, completeCalled bool
	var valueCount int64

	interceptor := NewCallbackInterceptor(
		WithOnStart(func() { startCalled = true }),
		WithOnComplete(func() { completeCalled = true }),
		WithOnValue(func(v any) { atomic.AddInt64(&valueCount, 1) }),
	)

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	stream := flow.FromSlice([]int{1, 2, 3})
	intercepted := core.Intercept[int]().Apply(stream)

	for range intercepted.All(ctx) {
	}

	if !startCalled {
		t.Error("OnStart was not called")
	}
	if !completeCalled {
		t.Error("OnComplete was not called")
	}
	if valueCount != 3 {
		t.Errorf("value count = %d, want 3", valueCount)
	}
}

func TestLogInterceptor(t *testing.T) {
	var logs []string
	logger := func(format string, args ...any) {
		logs = append(logs, format)
	}

	interceptor := NewLogInterceptor(logger, core.StreamStart, core.StreamEnd)

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	stream := flow.FromSlice([]int{1})
	intercepted := core.Intercept[int]().Apply(stream)

	for range intercepted.All(ctx) {
	}

	if len(logs) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(logs))
	}
}

func TestInterceptorWithErrors(t *testing.T) {
	counter := NewCounterInterceptor()

	ctx, registry := core.WithRegistry(context.Background())
	if err := registry.Register(counter); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int])
		go func() {
			defer close(ch)
			ch <- core.Ok(1)
			ch <- core.Err[int](errors.New("test error"))
			ch <- core.Ok(3)
		}()
		return ch
	})

	intercepted := core.Intercept[int]().Apply(errStream)

	for range intercepted.All(ctx) {
	}

	if got := counter.Values(); got != 2 {
		t.Errorf("Values() = %d, want 2", got)
	}
	if got := counter.Errors(); got != 1 {
		t.Errorf("Errors() = %d, want 1", got)
	}
	if got := counter.Total(); got != 3 {
		t.Errorf("Total() = %d, want 3", got)
	}
}

func TestMultipleInterceptors(t *testing.T) {
	counter := NewCounterInterceptor()
	var metricsReceived bool
	metricsInterceptor := NewMetricsInterceptor(func(m StreamMetrics) {
		metricsReceived = true
	})

	ctx, registry := core.WithRegistry(context.Background())
	_ = registry.Register(counter)
	_ = registry.Register(metricsInterceptor)

	stream := flow.FromSlice([]int{1, 2, 3})
	intercepted := core.Intercept[int]().Apply(stream)

	for range intercepted.All(ctx) {
	}

	if counter.Total() != 3 {
		t.Errorf("counter.Total() = %d, want 3", counter.Total())
	}
	if !metricsReceived {
		t.Error("metrics interceptor was not called")
	}
}

func TestMetricsInterceptorTiming(t *testing.T) {
	var gotMetrics StreamMetrics
	interceptor := NewMetricsInterceptor(func(m StreamMetrics) {
		gotMetrics = m
	})

	ctx, registry := core.WithRegistry(context.Background())
	_ = registry.Register(interceptor)

	slowStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int])
		go func() {
			defer close(ch)
			for i := 0; i < 3; i++ {
				time.Sleep(10 * time.Millisecond)
				ch <- core.Ok(i)
			}
		}()
		return ch
	})

	intercepted := core.Intercept[int]().Apply(slowStream)

	for range intercepted.All(ctx) {
	}

	if gotMetrics.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}
	if gotMetrics.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}
	if gotMetrics.EndTime.Before(gotMetrics.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}
