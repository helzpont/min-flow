// Package main demonstrates advanced error handling patterns including
// retry with backoff, circuit breakers, error collection, and fallbacks.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
)

func main() {
	fmt.Println("=== Advanced Error Handling ===")
	fmt.Println()

	retryWithJitterExample()
	circuitBreakerExample()
	fallbackWithTransformerExample()
	errorAggregationExample()
	partialFailureExample()
}

// retryWithJitterExample shows exponential backoff with jitter
func retryWithJitterExample() {
	fmt.Println("--- Retry with Exponential Backoff + Jitter ---")
	ctx := context.Background()

	var attemptCount int32

	// Simulate a flaky service that fails 70% of the time
	// Note: RetryWithBackoff requires same input/output type
	flakyMultiplier := func(n int) (int, error) {
		attempt := atomic.AddInt32(&attemptCount, 1)
		if rand.Float64() < 0.7 {
			return 0, fmt.Errorf("service unavailable (attempt %d)", attempt)
		}
		return n * 10, nil
	}

	ids := flow.FromSlice([]int{1, 2, 3})

	// Exponential backoff: 10ms * 2^attempt, with jitter, max 500ms
	backoff := flowerrors.ExponentialBackoff(10*time.Millisecond, 500*time.Millisecond)

	retried := flowerrors.RetryWithBackoff(5, backoff, flakyMultiplier).Apply(ctx, ids)

	fmt.Println("Processing with exponential backoff:")
	start := time.Now()
	for res := range retried.All(ctx) {
		if res.IsError() {
			fmt.Printf("  FAILED: %v\n", res.Error())
		} else {
			fmt.Printf("  OK: %d\n", res.Value())
		}
	}
	fmt.Printf("Total time: %v, Total attempts: %d\n\n", time.Since(start), attemptCount)
}

// circuitBreakerExample demonstrates the circuit breaker pattern using hooks
func circuitBreakerExample() {
	fmt.Println("--- Circuit Breaker Pattern ---")

	ctx := context.Background()

	// Attach a circuit breaker monitor that opens after 3 failures
	ctx, cbMonitor := flowerrors.WithCircuitBreakerMonitor[int](ctx, 3, func() {
		fmt.Println("  [Circuit breaker OPENED]")
	})

	var callCount int
	failingService := func(n int) (int, error) {
		callCount++
		if n < 5 {
			return 0, fmt.Errorf("service error for %d", n)
		}
		return n * 10, nil
	}

	fmt.Println("Processing (should trigger circuit breaker after 3 failures):")
	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})
	processed := flow.Map(failingService).Apply(ctx, numbers)

	for res := range processed.All(ctx) {
		if res.IsError() {
			fmt.Printf("  Error: %v\n", res.Error())
		} else {
			fmt.Printf("  Result: %d\n", res.Value())
		}
	}
	fmt.Printf("Total calls made: %d\n", callCount)
	fmt.Printf("Circuit is open: %v\n\n", cbMonitor.IsOpen())
}

// fallbackWithTransformerExample shows using Fallback transformer for error recovery
func fallbackWithTransformerExample() {
	fmt.Println("--- Fallback Transformer ---")
	ctx := context.Background()

	// Service that fails for even numbers
	failingService := func(n int) (int, error) {
		if n%2 == 0 {
			return 0, fmt.Errorf("failed for %d", n)
		}
		return n * 10, nil
	}

	// Fallback function receives the original value and error
	fallbackFn := func(n int, err error) int {
		fmt.Printf("  Using fallback for %d (error: %v)\n", n, err)
		return n * -1 // Return negative value as fallback
	}

	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})

	// Apply service, then use Fallback transformer to handle errors
	processed := flow.Map(failingService).Apply(ctx, numbers)
	withFallback := flowerrors.Fallback(fallbackFn).Apply(ctx, processed)

	fmt.Println("Results with fallback:")
	for res := range withFallback.All(ctx) {
		if res.IsValue() {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}
	fmt.Println()
}

// errorAggregationExample shows collecting and summarizing errors using hooks
func errorAggregationExample() {
	fmt.Println("--- Error Aggregation ---")
	ctx := context.Background()

	// Create an error collector using hooks
	ctx, collector := flowerrors.WithErrorCollector[int](ctx)

	// Process some items, some of which will fail
	items := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	processed := flow.Map(func(n int) (int, error) {
		if n%3 == 0 {
			return 0, fmt.Errorf("divisible by 3: %d", n)
		}
		if n%5 == 0 {
			return 0, fmt.Errorf("divisible by 5: %d", n)
		}
		return n * 10, nil
	}).Apply(ctx, items)

	// Consume the stream
	var successCount int
	for res := range processed.All(ctx) {
		if res.IsValue() {
			successCount++
		}
	}

	// Get error summary
	collectedErrors := collector.Errors()
	fmt.Printf("Processed: %d successful, %d errors\n", successCount, len(collectedErrors))
	fmt.Println("Error details:")
	for i, err := range collectedErrors {
		fmt.Printf("  %d. %v\n", i+1, err)
	}
	fmt.Println()
}

// partialFailureExample shows handling partial failures in a batch
func partialFailureExample() {
	fmt.Println("--- Partial Failure Handling ---")
	ctx := context.Background()

	// Track error count with hooks
	ctx, errorCounter := flowerrors.WithErrorCounter[int](ctx, nil)

	// Process items where some will fail
	ids := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	processed := flow.Map(func(n int) (int, error) {
		if n == 5 || n == 8 {
			return 0, fmt.Errorf("failed for id %d", n)
		}
		return n * 100, nil
	}).Apply(ctx, ids)

	fmt.Println("Processing batch:")
	var successful []int
	for res := range processed.All(ctx) {
		if res.IsValue() {
			successful = append(successful, res.Value())
		} else {
			fmt.Printf("  Error: %v\n", res.Error())
		}
	}

	fmt.Printf("Successful results: %v\n", successful)
	fmt.Printf("Total errors: %d\n", errorCounter.Count())
}
