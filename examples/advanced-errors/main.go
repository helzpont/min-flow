// Package main demonstrates advanced error handling patterns including
// retry with backoff, circuit breakers, error collection, and fallbacks.
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
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

	retried := flowerrors.RetryWithBackoff(5, backoff, flakyMultiplier).Apply(ids)

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

// circuitBreakerExample demonstrates the circuit breaker pattern
func circuitBreakerExample() {
	fmt.Println("--- Circuit Breaker Pattern ---")

	var circuitOpen atomic.Bool

	// Create a circuit breaker interceptor
	// Opens after 3 failures and calls the callback
	cb := flowerrors.NewCircuitBreakerInterceptor(3, func() {
		circuitOpen.Store(true)
		fmt.Println("  [Circuit breaker OPENED]")
	})

	ctx, registry := flow.WithRegistry(context.Background())
	_ = registry.Register(cb)

	var callCount int
	failingService := func(n int) (int, error) {
		callCount++
		if n < 5 {
			return 0, errors.New("service error")
		}
		return n * 10, nil
	}

	// First batch: will trigger circuit breaker
	fmt.Println("First batch (should trigger circuit breaker):")
	numbers1 := flow.FromSlice([]int{1, 2, 3, 4, 5, 6})
	processed1 := core.Intercept[int]().Apply(
		flow.Map(failingService).Apply(numbers1))

	for res := range processed1.All(ctx) {
		if res.IsError() {
			fmt.Printf("  Error: %v\n", res.Error())
		} else {
			fmt.Printf("  Result: %d\n", res.Value())
		}
	}
	fmt.Printf("Total calls made: %d\n", callCount)
	fmt.Printf("Circuit is open: %v\n\n", circuitOpen.Load())
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
	processed := flow.Map(failingService).Apply(numbers)
	withFallback := flowerrors.Fallback(fallbackFn).Apply(processed)

	fmt.Println("Results with fallback:")
	for res := range withFallback.All(ctx) {
		if res.IsValue() {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}
	fmt.Println()
}

// errorAggregationExample shows collecting and summarizing errors
func errorAggregationExample() {
	fmt.Println("--- Error Aggregation ---")
	ctx := context.Background()

	// Create an error collector
	collector := flowerrors.NewErrorCollectorInterceptor()

	ctxWithRegistry, registry := flow.WithRegistry(ctx)
	_ = registry.Register(collector)

	// Process some items, some of which will fail
	items := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	processed := core.Intercept[int]().Apply(
		flow.Map(func(n int) (int, error) {
			if n%3 == 0 {
				return 0, fmt.Errorf("divisible by 3: %d", n)
			}
			if n%5 == 0 {
				return 0, fmt.Errorf("divisible by 5: %d", n)
			}
			return n * 10, nil
		}).Apply(items))

	// Consume the stream
	var successCount int
	for res := range processed.All(ctxWithRegistry) {
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

// partialFailureExample demonstrates processing despite some failures
func partialFailureExample() {
	fmt.Println("--- Partial Failure Handling ---")
	ctx := context.Background()

	// Simulate batch processing where some items fail
	items := flow.FromSlice([]string{
		"valid-1",
		"invalid",
		"valid-2",
		"error-trigger",
		"valid-3",
	})

	// Process items, converting errors to a special value
	type ProcessResult struct {
		Input   string
		Output  string
		Success bool
		Error   string
	}

	processed := flow.FlatMap(func(item string) ([]ProcessResult, error) {
		// Simulate processing
		if item == "invalid" {
			return []ProcessResult{{
				Input:   item,
				Success: false,
				Error:   "invalid format",
			}}, nil
		}
		if item == "error-trigger" {
			return []ProcessResult{{
				Input:   item,
				Success: false,
				Error:   "processing error",
			}}, nil
		}
		return []ProcessResult{{
			Input:   item,
			Output:  "processed-" + item,
			Success: true,
		}}, nil
	}).Apply(items)

	// Separate successes and failures
	var successes, failures []ProcessResult
	for res := range processed.All(ctx) {
		if res.IsValue() {
			r := res.Value()
			if r.Success {
				successes = append(successes, r)
			} else {
				failures = append(failures, r)
			}
		}
	}

	fmt.Printf("Successes (%d):\n", len(successes))
	for _, r := range successes {
		fmt.Printf("  %s -> %s\n", r.Input, r.Output)
	}

	fmt.Printf("Failures (%d):\n", len(failures))
	for _, r := range failures {
		fmt.Printf("  %s: %s\n", r.Input, r.Error)
	}
	fmt.Println()
}
