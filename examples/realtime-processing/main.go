// Package main demonstrates real-time data processing with windowing,
// batching, and time-based operations.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/combine"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/lguimbarda/min-flow/flow/timing"
)

// SensorReading represents a simulated IoT sensor reading
type SensorReading struct {
	SensorID  string
	Value     float64
	Timestamp time.Time
}

// WindowStats represents statistics for a time window
type WindowStats struct {
	Count     int
	Sum       float64
	Min       float64
	Max       float64
	Avg       float64
	StartTime time.Time
	EndTime   time.Time
}

func main() {
	fmt.Println("=== Real-Time Stream Processing ===")
	fmt.Println()

	batchingExample()
	windowingExample()
	multiStreamExample()
	throttlingExample()
}

// batchingExample demonstrates batching items for efficient bulk processing
func batchingExample() {
	fmt.Println("--- Batching Example ---")
	ctx := context.Background()

	// Generate a stream of readings
	readings := flow.Range(1, 20)

	// Batch into groups of 5
	batched := aggregate.Batch[int](5).Apply(ctx, readings)

	// Process each batch
	fmt.Println("Processing in batches of 5:")
	for res := range batched.All(ctx) {
		if res.IsValue() {
			batch := res.Value()
			sum := 0
			for _, v := range batch {
				sum += v
			}
			fmt.Printf("  Batch %v -> sum = %d\n", batch, sum)
		}
	}
	fmt.Println()
}

// windowingExample shows tumbling window aggregation
func windowingExample() {
	fmt.Println("--- Time Window Example ---")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Create a stream that emits values at intervals
	values := flow.Emit(func(ctx context.Context) <-chan flow.Result[SensorReading] {
		out := make(chan flow.Result[SensorReading])
		go func() {
			defer close(out)
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			sensorID := "sensor-1"
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-ticker.C:
					reading := SensorReading{
						SensorID:  sensorID,
						Value:     20.0 + rand.Float64()*10, // 20-30 range
						Timestamp: t,
					}
					select {
					case <-ctx.Done():
						return
					case out <- flow.Ok(reading):
					}
				}
			}
		}()
		return out
	})

	// Aggregate using tumbling windows of 500ms
	// We use BatchTimeout to collect readings within a time window
	windowed := aggregate.BatchTimeout[SensorReading](100, 500*time.Millisecond).Apply(ctx, values)

	// Compute stats for each window
	stats := flow.Map(func(batch []SensorReading) (WindowStats, error) {
		if len(batch) == 0 {
			return WindowStats{}, nil
		}
		stats := WindowStats{
			Count:     len(batch),
			Min:       batch[0].Value,
			Max:       batch[0].Value,
			StartTime: batch[0].Timestamp,
			EndTime:   batch[len(batch)-1].Timestamp,
		}
		for _, r := range batch {
			stats.Sum += r.Value
			if r.Value < stats.Min {
				stats.Min = r.Value
			}
			if r.Value > stats.Max {
				stats.Max = r.Value
			}
		}
		stats.Avg = stats.Sum / float64(stats.Count)
		return stats, nil
	}).Apply(ctx, windowed)

	fmt.Println("Time-windowed statistics (500ms windows):")
	for res := range stats.All(ctx) {
		if res.IsValue() {
			s := res.Value()
			fmt.Printf("  Window: count=%d, avg=%.2f, min=%.2f, max=%.2f\n",
				s.Count, s.Avg, s.Min, s.Max)
		}
	}
	fmt.Println()
}

// multiStreamExample demonstrates merging and zipping streams
func multiStreamExample() {
	fmt.Println("--- Multi-Stream Processing ---")
	ctx := context.Background()

	// Create two sensor streams
	sensor1 := flow.FromSlice([]SensorReading{
		{SensorID: "temp-1", Value: 22.5},
		{SensorID: "temp-1", Value: 23.0},
		{SensorID: "temp-1", Value: 22.8},
	})

	sensor2 := flow.FromSlice([]SensorReading{
		{SensorID: "temp-2", Value: 21.0},
		{SensorID: "temp-2", Value: 21.5},
		{SensorID: "temp-2", Value: 21.2},
	})

	// Merge streams (interleaved processing)
	merged := combine.Merge(sensor1, sensor2)

	fmt.Println("Merged sensor readings:")
	for res := range merged.All(ctx) {
		if res.IsValue() {
			r := res.Value()
			fmt.Printf("  %s: %.1f°C\n", r.SensorID, r.Value)
		}
	}

	// Zip streams (pairwise processing)
	fmt.Println("\nPaired readings (Zip):")
	s1 := flow.FromSlice([]float64{22.5, 23.0, 22.8})
	s2 := flow.FromSlice([]float64{21.0, 21.5, 21.2})

	zipped := combine.ZipWith(s1, s2, func(a, b float64) float64 {
		return (a + b) / 2 // Average of paired readings
	})

	for res := range zipped.All(ctx) {
		if res.IsValue() {
			fmt.Printf("  Average: %.2f°C\n", res.Value())
		}
	}
	fmt.Println()
}

// throttlingExample shows rate limiting and debouncing
func throttlingExample() {
	fmt.Println("--- Throttling Example ---")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Fast producer: emits every 50ms
	fast := flow.Emit(func(ctx context.Context) <-chan flow.Result[int] {
		out := make(chan flow.Result[int])
		go func() {
			defer close(out)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			i := 0
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					i++
					select {
					case <-ctx.Done():
						return
					case out <- flow.Ok(i):
					}
				}
			}
		}()
		return out
	})

	// Throttle: only emit one item per 200ms
	throttled := timing.Throttle[int](200*time.Millisecond).Apply(ctx, fast)

	fmt.Println("Throttled output (max 1 per 200ms):")
	var values []int
	for res := range throttled.All(ctx) {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}
	fmt.Printf("  Received %d values: %v\n", len(values), values)

	// Sampling example: take every 3rd item
	fmt.Println("\nSampled output (every 3rd item):")
	numbers := flow.Range(1, 20)
	sampled := filter.TakeEvery[int](3).Apply(context.Background(), numbers)

	sampledVals, _ := flow.Slice(context.Background(), sampled)
	fmt.Printf("  Sampled values: %v\n", sampledVals)
	fmt.Println()
}
