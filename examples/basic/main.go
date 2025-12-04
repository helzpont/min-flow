package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func main() {
	ctx := context.Background()

	// Create a stream from a slice of numbers
	numbers := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

	// Filter to keep only even numbers
	evens := filter.Where(func(n int) bool {
		return n%2 == 0
	}).Apply(ctx, numbers)

	// Double each number
	doubled := flow.Map(func(n int) (int, error) {
		return n * 2, nil
	}).Apply(ctx, evens)

	// Collect all results into a slice (function style)
	results, err := flow.Slice(ctx, doubled)
	if err != nil {
		panic(err)
	}
	fmt.Println("Doubled even numbers:", results)

	// Same thing using Sink style - mirrors Transformer.Apply with Sink.From
	numbers2 := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	pipeline := flow.Map(func(n int) (int, error) {
		if n%2 == 0 {
			return n * 2, nil
		}
		return n, nil
	}).Apply(ctx, numbers2)

	// ToSlice() returns a Sink; From() consumes the stream
	results2, _ := flow.ToSlice[int]().From(ctx, pipeline)
	fmt.Println("With Sink API:", results2)

	// Example with chained transformers
	numbers3 := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	filtered := filter.Where(func(n int) bool { return n > 5 }).Apply(ctx, numbers3)
	taken := filter.Take[int](3).Apply(ctx, filtered)

	// Get first value using Sink
	first, _ := flow.ToFirst[int]().From(ctx, taken)
	fmt.Println("First number > 5:", first)

	// Example with time-based operations
	ctx2, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	ticker := flow.Interval(100 * time.Millisecond)
	limited := filter.Take[int](3).Apply(ctx2, ticker)
	ticks, _ := flow.ToSlice[int]().From(ctx2, limited)
	fmt.Println("Ticks received:", ticks)
}
