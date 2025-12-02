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
	}).Apply(ctx, numbers)

	// Collect results - includes errors
	fmt.Println("Processing with errors:")
	for res := range divided.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("  Error: %v\n", res.Error())
		} else {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}

	// Using OnError to handle errors
	numbers2 := flow.FromSlice([]int{1, 0, 2})
	errCount := 0

	mapped2 := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, errors.New("zero!")
		}
		return n, nil
	}).Apply(ctx, numbers2)

	handled := flowerrors.OnError[int](func(err error) {
		errCount++
		fmt.Printf("Handled error: %v\n", err)
	}).Apply(ctx, mapped2)

	// Collect values only (errors are filtered out after handling)
	filtered := flowerrors.FilterErrors[int](func(err error) bool {
		return false // Filter out all errors
	}).Apply(ctx, handled)
	results, _ := flow.Slice(ctx, filtered)
	fmt.Printf("Valid results: %v (errors handled: %d)\n", results, errCount)

	// Using MapErrors to transform errors
	numbers3 := flow.FromSlice([]int{1, 0, 2})
	mapped3 := flow.Map(func(n int) (int, error) {
		if n == 0 {
			return 0, errors.New("original error")
		}
		return n, nil
	}).Apply(ctx, numbers3)

	mapped := flowerrors.MapErrors[int](func(err error) error {
		return fmt.Errorf("transformed: %w", err)
	}).Apply(ctx, mapped3)

	fmt.Println("\nWith transformed errors:")
	for res := range mapped.Emit(ctx) {
		if res.IsError() {
			fmt.Printf("  Transformed: %v\n", res.Error())
		} else {
			fmt.Printf("  Value: %d\n", res.Value())
		}
	}
}
