package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
	"github.com/lguimbarda/min-flow/flow/parallel"
)

// This example demonstrates parallel processing:
// - Parallel map for CPU-bound work
// - Parallel ordered for maintaining order
// - Fan-out/Fan-in patterns

func main() {
	ctx := context.Background()

	// Simulate CPU-bound work
	heavyComputation := func(n int) int {
		time.Sleep(50 * time.Millisecond)
		return n * n
	}

	numbers := flow.Range(1, 10)

	// Sequential processing (slow)
	start := time.Now()
	sequential := flow.Map(func(n int) (int, error) {
		return heavyComputation(n), nil
	}).Apply(numbers)
	seqResults, _ := flow.Slice(ctx, sequential)
	seqDuration := time.Since(start)
	fmt.Printf("Sequential: %v in %v\n", seqResults, seqDuration)

	// Parallel processing (fast)
	numbers2 := flow.Range(1, 10)
	start = time.Now()
	par := parallel.Map(4, heavyComputation).Apply(numbers2)
	parResults, _ := flow.Slice(ctx, par)
	parDuration := time.Since(start)
	fmt.Printf("Parallel (4 workers): %v in %v\n", parResults, parDuration)

	// Parallel ordered (maintains input order)
	numbers3 := flow.Range(1, 10)
	start = time.Now()
	ordered := parallel.Ordered(4, heavyComputation).Apply(numbers3)
	ordResults, _ := flow.Slice(ctx, ordered)
	ordDuration := time.Since(start)
	fmt.Printf("Parallel ordered: %v in %v\n", ordResults, ordDuration)

	// Show speedup
	fmt.Printf("\nSpeedup: %.2fx\n", float64(seqDuration)/float64(parDuration))

	// Fan-out/Fan-in pattern
	numbers4 := flow.Range(1, 6)
	fanStreams := combine.FanOut(3, numbers4)
	fmt.Printf("\nFan-out to %d streams\n", len(fanStreams))

	// Process each fan-out stream
	var processed []flow.Stream[int]
	for i, s := range fanStreams {
		workerID := i
		p := flow.Map(func(n int) (int, error) {
			fmt.Printf("  Worker %d processing %d\n", workerID, n)
			return n * 10, nil
		}).Apply(s)
		processed = append(processed, p)
	}

	// Fan-in the results
	merged := combine.FanIn(processed...)
	results, _ := flow.Slice(ctx, merged)
	fmt.Printf("Fan-in results: %v\n", results)
}
