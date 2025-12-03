package benchmarks

import (
	"testing"
	"time"

	"github.com/destel/rill"
	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/parallel"
	lop "github.com/samber/lo/parallel"
)

// =============================================================================
// Parallel Map Benchmarks - comparing parallel scaling across libraries
// =============================================================================

// Simulate some work to make parallelization worthwhile
func expensiveSquare(x int) int {
	// Small amount of work to justify parallelization
	time.Sleep(time.Microsecond)
	return x * x
}

// -----------------------------------------------------------------------------
// min-flow parallel benchmarks
// -----------------------------------------------------------------------------

func BenchmarkParallelMap_MinFlow_1Worker(b *testing.B) {
	benchmarkParallelMapMinFlow(b, 1)
}

func BenchmarkParallelMap_MinFlow_2Workers(b *testing.B) {
	benchmarkParallelMapMinFlow(b, 2)
}

func BenchmarkParallelMap_MinFlow_4Workers(b *testing.B) {
	benchmarkParallelMapMinFlow(b, 4)
}

func BenchmarkParallelMap_MinFlow_8Workers(b *testing.B) {
	benchmarkParallelMapMinFlow(b, 8)
}

func BenchmarkParallelMap_MinFlow_16Workers(b *testing.B) {
	benchmarkParallelMapMinFlow(b, 16)
}

func benchmarkParallelMapMinFlow(b *testing.B, workers int) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := parallel.Map(workers, expensiveSquare).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

// -----------------------------------------------------------------------------
// rill parallel benchmarks
// -----------------------------------------------------------------------------

func BenchmarkParallelMap_Rill_1Worker(b *testing.B) {
	benchmarkParallelMapRill(b, 1)
}

func BenchmarkParallelMap_Rill_2Workers(b *testing.B) {
	benchmarkParallelMapRill(b, 2)
}

func BenchmarkParallelMap_Rill_4Workers(b *testing.B) {
	benchmarkParallelMapRill(b, 4)
}

func BenchmarkParallelMap_Rill_8Workers(b *testing.B) {
	benchmarkParallelMapRill(b, 8)
}

func BenchmarkParallelMap_Rill_16Workers(b *testing.B) {
	benchmarkParallelMapRill(b, 16)
}

func benchmarkParallelMapRill(b *testing.B, workers int) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		mapped := rill.Map(stream, workers, func(x int) (int, error) {
			return expensiveSquare(x), nil
		})
		_, _ = rill.ToSlice(mapped)
	}
}

// -----------------------------------------------------------------------------
// samber/lo parallel benchmarks (uses lop package)
// Note: lo parallel doesn't support explicit worker count, uses runtime.NumCPU
// -----------------------------------------------------------------------------

func BenchmarkParallelMap_Lo(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Note: lo's parallel Map uses all available CPUs
		_ = lop.Map(data, func(x int, _ int) int {
			return expensiveSquare(x)
		})
	}
}

// -----------------------------------------------------------------------------
// Sequential baseline for comparison
// -----------------------------------------------------------------------------

func BenchmarkParallelMap_Sequential(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := make([]int, len(data))
		for j, x := range data {
			result[j] = expensiveSquare(x)
		}
		_ = result
	}
}
