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
	evens := filter.Filter(func(n int) bool {
		return n%2 == 0
	}).Apply(ctx, numbers)

	// Double each number
	doubled := flow.Map(func(n int) (int, error) {
		return n * 2, nil
	}).Apply(ctx, evens)

	// Collect all results into a slice
	results, err := flow.Slice(ctx, doubled)
	if err != nil {
		panic(err)
	}

	fmt.Println("Doubled even numbers:", results)

	// Example with chained transformers using Apply
	numbers2 := flow.FromSlice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	filtered := filter.Filter(func(n int) bool { return n > 5 }).Apply(ctx, numbers2)
	taken := filter.Take[int](3).Apply(ctx, filtered)
	result2, _ := flow.Slice(ctx, taken)
	fmt.Println("First 3 numbers > 5:", result2)

	// Example with time-based operations
	ctx2, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	ticker := flow.Interval(100 * time.Millisecond)
	limited := filter.Take[int](3).Apply(ctx2, ticker)
	ticks, _ := flow.Slice(ctx2, limited)
	fmt.Println("Ticks received:", ticks)
}
