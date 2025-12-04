package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/observe"
)

func main() {
	fmt.Println("=== Interceptor Examples ===")
	fmt.Println()

	metricsExample()
	callbackExample()
	multipleInterceptorsExample()
}

// metricsExample demonstrates collecting stream metrics via interceptors
func metricsExample() {
	fmt.Println("--- Metrics Interceptor ---")

	// Create a context with registry for interceptors
	ctx, registry := flow.WithRegistry(context.Background())

	// Create and register a metrics interceptor
	var finalMetrics observe.StreamMetrics
	metricsInterceptor := observe.NewMetricsInterceptor(func(m observe.StreamMetrics) {
		finalMetrics = m
	})
	_ = registry.Register(metricsInterceptor)

	// Create a stream
	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Apply interception to collect metrics
	intercepted := core.Intercept[int]().Apply(ctx, numbers)

	// Process the stream
	sum := 0
	for res := range intercepted.All(ctx) {
		if res.IsValue() {
			sum += res.Value()
		}
	}

	fmt.Printf("Sum: %d\n", sum)
	fmt.Printf("Metrics:\n")
	fmt.Printf("  Total items: %d\n", finalMetrics.TotalItems)
	fmt.Printf("  Values: %d\n", finalMetrics.ValueCount)
	fmt.Printf("  Duration: %v\n", finalMetrics.EndTime.Sub(finalMetrics.StartTime))
	fmt.Printf("  Items/sec: %.2f\n", finalMetrics.ItemsPerSecond)
	fmt.Println()
}

// callbackExample demonstrates using callback interceptors for event handling
func callbackExample() {
	fmt.Println("--- Callback Interceptor ---")

	ctx, registry := flow.WithRegistry(context.Background())

	// Create callback interceptor with handlers for different events
	var startTime time.Time
	valueCount := 0

	callbackInterceptor := observe.NewCallbackInterceptor(
		observe.WithOnStart(func() {
			startTime = time.Now()
			fmt.Println("Stream started!")
		}),
		observe.WithOnComplete(func() {
			fmt.Printf("Stream completed in %v\n", time.Since(startTime))
		}),
		observe.WithOnValue(func(v any) {
			valueCount++
		}),
		observe.WithOnError(func(err error) {
			fmt.Printf("Error occurred: %v\n", err)
		}),
	)
	_ = registry.Register(callbackInterceptor)

	// Process stream
	numbers := flow.FromSlice([]int{10, 20, 30, 40, 50})
	intercepted := core.Intercept[int]().Apply(ctx, numbers)

	for range intercepted.All(ctx) {
	}

	fmt.Printf("Processed %d values\n", valueCount)
	fmt.Println()
}

// multipleInterceptorsExample shows how multiple interceptors work together
func multipleInterceptorsExample() {
	fmt.Println("--- Multiple Interceptors ---")

	ctx, registry := flow.WithRegistry(context.Background())

	// Counter interceptor for basic counts
	counter := observe.NewCounterInterceptor()
	_ = registry.Register(counter)

	// Log interceptor for stream events
	logInterceptor := observe.NewLogInterceptor(
		func(format string, args ...any) {
			fmt.Printf("  LOG: "+format+"\n", args...)
		},
		flow.StreamStart,
		flow.StreamEnd,
	)
	_ = registry.Register(logInterceptor)

	// Process stream
	numbers := flow.FromSlice([]int{1, 2, 3})
	intercepted := core.Intercept[int]().Apply(ctx, numbers)

	results := []int{}
	for res := range intercepted.All(ctx) {
		if res.IsValue() {
			results = append(results, res.Value())
		}
	}

	fmt.Printf("Results: %v\n", results)
	fmt.Printf("Counter stats - Total: %d, Values: %d, Errors: %d\n",
		counter.Total(), counter.Values(), counter.Errors())
}
