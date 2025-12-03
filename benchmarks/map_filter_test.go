package benchmarks

import (
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/destel/rill"
	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/samber/lo"
)

// =============================================================================
// Map Benchmarks
// =============================================================================

func BenchmarkMap_MinFlow_Small(b *testing.B) {
	benchmarkMapMinFlow(b, SmallSize)
}

func BenchmarkMap_MinFlow_Medium(b *testing.B) {
	benchmarkMapMinFlow(b, MediumSize)
}

func BenchmarkMap_MinFlow_Large(b *testing.B) {
	benchmarkMapMinFlow(b, LargeSize)
}

func benchmarkMapMinFlow(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

func BenchmarkMap_Rill_Small(b *testing.B) {
	benchmarkMapRill(b, SmallSize)
}

func BenchmarkMap_Rill_Medium(b *testing.B) {
	benchmarkMapRill(b, MediumSize)
}

func BenchmarkMap_Rill_Large(b *testing.B) {
	benchmarkMapRill(b, LargeSize)
}

func benchmarkMapRill(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		mapped := rill.Map(stream, 1, func(x int) (int, error) {
			return square(x), nil
		})
		_, _ = rill.ToSlice(mapped)
	}
}

func BenchmarkMap_Lo_Small(b *testing.B) {
	benchmarkMapLo(b, SmallSize)
}

func BenchmarkMap_Lo_Medium(b *testing.B) {
	benchmarkMapLo(b, MediumSize)
}

func BenchmarkMap_Lo_Large(b *testing.B) {
	benchmarkMapLo(b, LargeSize)
}

func benchmarkMapLo(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = lo.Map(data, func(x int, _ int) int {
			return square(x)
		})
	}
}

func BenchmarkMap_GoLinq_Small(b *testing.B) {
	benchmarkMapGoLinq(b, SmallSize)
}

func BenchmarkMap_GoLinq_Medium(b *testing.B) {
	benchmarkMapGoLinq(b, MediumSize)
}

func BenchmarkMap_GoLinq_Large(b *testing.B) {
	benchmarkMapGoLinq(b, LargeSize)
}

func benchmarkMapGoLinq(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var result []int
		linq.From(data).SelectT(func(x int) int {
			return square(x)
		}).ToSlice(&result)
	}
}

// Baseline: raw for loop
func BenchmarkMap_RawLoop_Small(b *testing.B) {
	benchmarkMapRawLoop(b, SmallSize)
}

func BenchmarkMap_RawLoop_Medium(b *testing.B) {
	benchmarkMapRawLoop(b, MediumSize)
}

func BenchmarkMap_RawLoop_Large(b *testing.B) {
	benchmarkMapRawLoop(b, LargeSize)
}

func benchmarkMapRawLoop(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := make([]int, len(data))
		for j, x := range data {
			result[j] = square(x)
		}
		_ = result
	}
}

// =============================================================================
// Filter Benchmarks
// =============================================================================

func BenchmarkFilter_MinFlow_Small(b *testing.B) {
	benchmarkFilterMinFlow(b, SmallSize)
}

func BenchmarkFilter_MinFlow_Medium(b *testing.B) {
	benchmarkFilterMinFlow(b, MediumSize)
}

func BenchmarkFilter_MinFlow_Large(b *testing.B) {
	benchmarkFilterMinFlow(b, LargeSize)
}

func benchmarkFilterMinFlow(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		filtered := filter.Where(isEven).Apply(ctx, stream)
		_, _ = core.Slice(ctx, filtered)
	}
}

func BenchmarkFilter_Rill_Small(b *testing.B) {
	benchmarkFilterRill(b, SmallSize)
}

func BenchmarkFilter_Rill_Medium(b *testing.B) {
	benchmarkFilterRill(b, MediumSize)
}

func BenchmarkFilter_Rill_Large(b *testing.B) {
	benchmarkFilterRill(b, LargeSize)
}

func benchmarkFilterRill(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		filtered := rill.Filter(stream, 1, func(x int) (bool, error) {
			return isEven(x), nil
		})
		_, _ = rill.ToSlice(filtered)
	}
}

func BenchmarkFilter_Lo_Small(b *testing.B) {
	benchmarkFilterLo(b, SmallSize)
}

func BenchmarkFilter_Lo_Medium(b *testing.B) {
	benchmarkFilterLo(b, MediumSize)
}

func BenchmarkFilter_Lo_Large(b *testing.B) {
	benchmarkFilterLo(b, LargeSize)
}

func benchmarkFilterLo(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = lo.Filter(data, func(x int, _ int) bool {
			return isEven(x)
		})
	}
}

func BenchmarkFilter_GoLinq_Small(b *testing.B) {
	benchmarkFilterGoLinq(b, SmallSize)
}

func BenchmarkFilter_GoLinq_Medium(b *testing.B) {
	benchmarkFilterGoLinq(b, MediumSize)
}

func BenchmarkFilter_GoLinq_Large(b *testing.B) {
	benchmarkFilterGoLinq(b, LargeSize)
}

func benchmarkFilterGoLinq(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var result []int
		linq.From(data).WhereT(func(x int) bool {
			return isEven(x)
		}).ToSlice(&result)
	}
}

func BenchmarkFilter_RawLoop_Small(b *testing.B) {
	benchmarkFilterRawLoop(b, SmallSize)
}

func BenchmarkFilter_RawLoop_Medium(b *testing.B) {
	benchmarkFilterRawLoop(b, MediumSize)
}

func BenchmarkFilter_RawLoop_Large(b *testing.B) {
	benchmarkFilterRawLoop(b, LargeSize)
}

func benchmarkFilterRawLoop(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := make([]int, 0, len(data)/2)
		for _, x := range data {
			if isEven(x) {
				result = append(result, x)
			}
		}
		_ = result
	}
}
