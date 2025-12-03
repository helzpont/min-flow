// Package benchmarks provides comparative benchmarks of min-flow against
// popular Go stream processing libraries.
package benchmarks

import (
	"context"
	"errors"
	"strconv"
)

// Common test errors
var errTest = errors.New("test error")

// Test data sizes
const (
	SmallSize  = 100
	MediumSize = 1_000
	LargeSize  = 10_000
)

// generateInts creates a slice of integers for benchmarking.
func generateInts(n int) []int {
	data := make([]int, n)
	for i := range data {
		data[i] = i
	}
	return data
}

// generateStrings creates a slice of strings for benchmarking.
func generateStrings(n int) []string {
	data := make([]string, n)
	for i := range data {
		data[i] = strconv.Itoa(i)
	}
	return data
}

// Common transformation functions used across benchmarks
// Note: min-flow's Map expects func(IN) (OUT, error) signature

// squareWithErr returns the square of an integer (min-flow compatible).
func squareWithErr(x int) (int, error) {
	return x * x, nil
}

// square returns the square of an integer (for other libraries).
func square(x int) int {
	return x * x
}

// isEven returns true if the number is even.
func isEven(x int) bool {
	return x%2 == 0
}

// add returns the sum of two integers.
func add(a, b int) int {
	return a + b
}

// stringLen returns the length of a string.
func stringLen(s string) int {
	return len(s)
}

// Background context for benchmarks
var ctx = context.Background()
