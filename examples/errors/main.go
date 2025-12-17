package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
)

func main() {
	ctx := context.Background()

	// Create a stream where some operations might fail
	numbers := flow.FromSlice([]int{1, 2, 0, 4, 5})

	// Map with potential errors
	divided := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, errors.New("cannot divide by zero")
		}
		return 100 / n, nil
	}).Apply(numbers)

	// Collect results - includes errors
	fmt.Println("Processing with errors:")
	for res := range divided.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("  Error: %v\n", res.Error())
		} else {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}

	// Using CatchError to handle errors
	numbers2 := flow.FromSlice([]int{1, 0, 2})
	errCount := 0

	mapped2 := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, errors.New("zero!")
		}
		return n, nil
	}).Apply(numbers2)

	// CatchError handles errors matching a predicate with a handler that provides fallback values
	handled := flowerrors.CatchError(
		func(err error) bool { return true }, // catch all errors
		func(err error) (int, error) {
			errCount++
			fmt.Printf("Handled error: %v\n", err)
			return 0, nil // return fallback value (0) and nil to not propagate error
		},
	).Apply(mapped2)

	// IgnoreErrors filters out any remaining errors
	filtered := flowerrors.IgnoreErrors[int]().Apply(handled)
	results, _ := flow.Slice(ctx, filtered)
	fmt.Printf("Valid results: %v (errors handled: %d)\n", results, errCount)

	// Using MapErrors to transform errors
	numbers3 := flow.FromSlice([]int{1, 0, 2})
	mapped3 := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, errors.New("original error")
		}
		return n, nil
	}).Apply(numbers3)

	mapped := flowerrors.MapErrors[int](func(err error) error {
		return fmt.Errorf("transformed: %w", err)
	}).Apply(mapped3)

	fmt.Println("\nWith transformed errors:")
	for res := range mapped.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("  Transformed: %v\n", res.Error())
		} else {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}
}
