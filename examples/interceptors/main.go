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
	fmt.Println("=== Hooks Examples ===")
	fmt.Println()

	counterExample()
	callbackExample()
	loggingExample()
}

// counterExample demonstrates counting stream values with hooks
func counterExample() {
	fmt.Println("--- Counter Hook ---")

	ctx := context.Background()

	// Attach counter hooks for int streams
	ctx, counter := observe.WithCounter[int](ctx)

	// Create a stream
	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Apply a mapper to trigger hooks
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	mapped := mapper.Apply(ctx, numbers)

	// Process the stream
	sum := 0
	for res := range mapped.All(ctx) {
		if res.IsValue() {
			sum += res.Value()
		}
	}

	fmt.Printf("Sum: %d\n", sum)
	fmt.Printf("Counter stats:\n")
	fmt.Printf("  Values: %d\n", counter.Values())
	fmt.Printf("  Errors: %d\n", counter.Errors())
	fmt.Printf("  Total: %d\n", counter.Total())
	fmt.Println()
}

// callbackExample demonstrates using callback hooks for event handling
func callbackExample() {
	fmt.Println("--- Callback Hooks ---")

	ctx := context.Background()

	var startTime time.Time
	valueCount := 0

	// Attach individual hooks
	ctx = observe.WithStartHook[int](ctx, func() {
		startTime = time.Now()
		fmt.Println("Stream started!")
	})
	ctx = observe.WithCompleteHook[int](ctx, func() {
		fmt.Printf("Stream completed in %v\n", time.Since(startTime))
	})
	ctx = observe.WithValueHook(ctx, func(v int) {
		valueCount++
	})
	ctx = observe.WithErrorHook[int](ctx, func(err error) {
		fmt.Printf("Error occurred: %v\n", err)
	})

	// Process stream
	numbers := flow.FromSlice([]int{10, 20, 30, 40, 50})
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	mapped := mapper.Apply(ctx, numbers)

	for range mapped.All(ctx) {
	}

	fmt.Printf("Processed %d values\n", valueCount)
	fmt.Println()
}

// loggingExample shows how to use logging hooks
func loggingExample() {
	fmt.Println("--- Logging Hook ---")

	ctx := context.Background()

	// Attach logging hooks
	ctx = observe.WithLogging[int](ctx, func(format string, args ...any) {
		fmt.Printf("  LOG: "+format+"\n", args...)
	})

	// Process stream
	numbers := flow.FromSlice([]int{1, 2, 3})
	mapper := core.Map(func(x int) (int, error) { return x, nil })
	mapped := mapper.Apply(ctx, numbers)

	results := []int{}
	for res := range mapped.All(ctx) {
		if res.IsValue() {
			results = append(results, res.Value())
		}
	}

	fmt.Printf("Results: %v\n", results)
}
