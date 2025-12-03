package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
)

func main() {
	fmt.Println("=== Resilience Examples ===")
	fmt.Println()

	retryExample()
	retryWithBackoffExample()
	fallbackExample()
	circuitBreakerInterceptorExample()
	errorCollectorExample()
}

// retryExample demonstrates automatic retry of failed operations
func retryExample() {
	fmt.Println("--- Retry Example ---")
	ctx := context.Background()

	// Simulate an unreliable operation
	attempts := make(map[int]int)
	unreliableOp := func(n int) (int, error) {
		attempts[n]++
		// Fail on first attempt for even numbers
		if n%2 == 0 && attempts[n] < 2 {
			return 0, fmt.Errorf("temporary failure for %d", n)
		}
		return n * 10, nil
	}

	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5})

	// Apply retry - will retry failed operations up to 3 times
	retried := flowerrors.Retry(3, unreliableOp).Apply(ctx, numbers)

	fmt.Println("Processing with retry:")
	for res := range retried.All(ctx) {
		if res.IsError() {
			fmt.Printf("  Failed: %v\n", res.Error())
		} else {
			fmt.Printf("  Result: %d (attempts: %d)\n", res.Value(), attempts[res.Value()/10])
		}
	}
	fmt.Println()
}

// retryWithBackoffExample shows exponential backoff on retries
func retryWithBackoffExample() {
	fmt.Println("--- Retry with Backoff Example ---")
	ctx := context.Background()

	attemptCount := 0
	failingOp := func(n int) (int, error) {
		attemptCount++
		if attemptCount < 3 {
			return 0, errors.New("not ready yet")
		}
		return n * 100, nil
	}

	numbers := flow.FromSlice([]int{42})

	// Use exponential backoff: 10ms, 20ms, 40ms...
	backoff := flowerrors.ExponentialBackoff(10*time.Millisecond, 0)
	retried := flowerrors.RetryWithBackoff(5, backoff, failingOp).Apply(ctx, numbers)

	start := time.Now()
	for res := range retried.All(ctx) {
		if res.IsValue() {
			fmt.Printf("Success after %d attempts in %v: %d\n",
				attemptCount, time.Since(start), res.Value())
		}
	}
	fmt.Println()
}

// fallbackExample demonstrates providing fallback values on errors
func fallbackExample() {
	fmt.Println("--- Fallback Example ---")
	ctx := context.Background()

	// Operation that fails for certain values
	riskyOp := func(n int) (int, error) {
		if n < 0 {
			return 0, errors.New("negative numbers not allowed")
		}
		return n * 2, nil
	}

	numbers := flow.FromSlice([]int{5, -1, 10, -3, 15})
	mapped := flow.Map(riskyOp).Apply(ctx, numbers)

	// Use fallback to provide default value on error
	withFallback := flowerrors.FallbackValue(-999).Apply(ctx, mapped)

	fmt.Println("Results with fallback:")
	for res := range withFallback.All(ctx) {
		fmt.Printf("  %d\n", res.Value())
	}
	fmt.Println()
}

// circuitBreakerInterceptorExample shows circuit breaker pattern via interceptors
func circuitBreakerInterceptorExample() {
	fmt.Println("--- Circuit Breaker Interceptor Example ---")

	ctx, registry := core.WithRegistry(context.Background())

	// Create circuit breaker that trips after 3 errors
	var circuitTripped bool
	cb := flowerrors.NewCircuitBreakerInterceptor(3, func() {
		circuitTripped = true
		fmt.Println("  ⚠️  Circuit breaker TRIPPED!")
	})
	_ = registry.Register(cb)

	// Create a stream with some errors
	errStream := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		ch := make(chan core.Result[int])
		go func() {
			defer close(ch)
			for i := 1; i <= 10; i++ {
				// Simulate random failures
				if rand.Float32() < 0.4 {
					ch <- core.Err[int](fmt.Errorf("error at item %d", i))
				} else {
					ch <- core.Ok(i)
				}
			}
		}()
		return ch
	})

	intercepted := core.Intercept[int]().Apply(ctx, errStream)

	successCount := 0
	errorCount := 0
	for res := range intercepted.All(ctx) {
		if res.IsValue() {
			successCount++
		} else {
			errorCount++
		}
	}

	fmt.Printf("Processed: %d successes, %d errors\n", successCount, errorCount)
	fmt.Printf("Circuit tripped: %v (failure count: %d)\n", circuitTripped, cb.FailureCount())
	fmt.Println()
}

// errorCollectorExample shows collecting errors for analysis
func errorCollectorExample() {
	fmt.Println("--- Error Collector Example ---")

	ctx, registry := core.WithRegistry(context.Background())

	// Create collector with max 5 errors
	collector := flowerrors.NewErrorCollectorInterceptor(
		flowerrors.WithMaxErrors(5),
	)
	_ = registry.Register(collector)

	// Also add an error counter
	counter := flowerrors.NewErrorCounterInterceptor(nil)
	_ = registry.Register(counter)

	// Stream with multiple errors
	errStream := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		ch := make(chan core.Result[string])
		go func() {
			defer close(ch)
			ch <- core.Ok("hello")
			ch <- core.Err[string](errors.New("error 1: connection timeout"))
			ch <- core.Ok("world")
			ch <- core.Err[string](errors.New("error 2: invalid response"))
			ch <- core.Err[string](errors.New("error 3: rate limited"))
			ch <- core.Ok("!")
		}()
		return ch
	})

	intercepted := core.Intercept[string]().Apply(ctx, errStream)

	// Process stream
	var values []string
	for res := range intercepted.All(ctx) {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	fmt.Printf("Values: %v\n", values)
	fmt.Printf("Total errors: %d\n", counter.Count())
	fmt.Println("Collected errors:")
	for i, err := range collector.Errors() {
		fmt.Printf("  %d. %v\n", i+1, err)
	}
}
