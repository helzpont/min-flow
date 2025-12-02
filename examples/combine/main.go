package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/combine"
)

// This example demonstrates combining multiple streams:
// - Merge: interleave values from multiple streams
// - Concat: sequential combination
// - Zip: pair elements from streams
// - Race: first stream to emit wins

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Merge: Interleave values as they arrive
	fmt.Println("=== Merge ===")
	s1 := flow.FromSlice([]string{"a", "b", "c"})
	s2 := flow.FromSlice([]string{"1", "2", "3"})

	merged := combine.Merge(s1, s2)
	mergedResults, _ := flow.Slice(ctx, merged)
	fmt.Println("Merged:", mergedResults)

	// Concat: Sequential combination
	fmt.Println("\n=== Concat ===")
	s3 := flow.FromSlice([]string{"first", "second"})
	s4 := flow.FromSlice([]string{"third", "fourth"})

	concatenated := combine.Concat(s3, s4)
	concatResults, _ := flow.Slice(ctx, concatenated)
	fmt.Println("Concatenated:", concatResults)

	// Zip: Pair elements
	fmt.Println("\n=== Zip ===")
	names := flow.FromSlice([]string{"Alice", "Bob", "Charlie"})
	ages := flow.FromSlice([]int{30, 25, 35})

	type Person struct {
		Name string
		Age  int
	}

	zipped := combine.ZipWith(names, ages, func(name string, age int) Person {
		return Person{Name: name, Age: age}
	})
	people, _ := flow.Slice(ctx, zipped)
	fmt.Println("Zipped people:")
	for _, p := range people {
		fmt.Printf("  %s is %d years old\n", p.Name, p.Age)
	}

	// Race: First stream to emit wins
	fmt.Println("\n=== Race ===")
	slow := flow.Defer(func() flow.Stream[string] {
		time.Sleep(100 * time.Millisecond)
		return flow.FromSlice([]string{"slow1", "slow2"})
	})
	fast := flow.FromSlice([]string{"fast1", "fast2"})

	winner := combine.Race(slow, fast)
	raceResults, _ := flow.Slice(ctx, winner)
	fmt.Println("Race winner:", raceResults)

	// CombineLatest: Emit latest from each when any emits
	fmt.Println("\n=== CombineLatest ===")
	ch1 := make(chan int, 3)
	ch2 := make(chan int, 3)

	go func() {
		ch1 <- 1
		time.Sleep(10 * time.Millisecond)
		ch1 <- 2
		close(ch1)
	}()

	go func() {
		time.Sleep(5 * time.Millisecond)
		ch2 <- 100
		time.Sleep(10 * time.Millisecond)
		ch2 <- 200
		close(ch2)
	}()

	stream1 := flow.FromChannel(ch1)
	stream2 := flow.FromChannel(ch2)

	combinedStream := combine.CombineLatest(stream1, stream2)
	combineResults, _ := flow.Slice(ctx, combinedStream)
	fmt.Println("Combined latest:")
	for _, pair := range combineResults {
		if len(pair) >= 2 {
			fmt.Printf("  [%d, %d]\n", pair[0], pair[1])
		}
	}
}
