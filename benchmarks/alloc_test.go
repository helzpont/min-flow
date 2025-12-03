package benchmarks

import (
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/destel/rill"
	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/samber/lo"
)

// =============================================================================
// Memory Allocation Benchmarks
// These benchmarks are designed to highlight allocation differences.
// Run with: go test -bench=BenchmarkAlloc -benchmem
// =============================================================================

// Large dataset to amplify allocation differences
const AllocSize = 10_000

func BenchmarkAlloc_Map_MinFlow(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

func BenchmarkAlloc_Map_Rill(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		mapped := rill.Map(stream, 1, func(x int) (int, error) {
			return square(x), nil
		})
		_, _ = rill.ToSlice(mapped)
	}
}

func BenchmarkAlloc_Map_Lo(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = lo.Map(data, func(x int, _ int) int {
			return square(x)
		})
	}
}

func BenchmarkAlloc_Map_GoLinq(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result []int
		linq.From(data).SelectT(func(x int) int {
			return square(x)
		}).ToSlice(&result)
	}
}

func BenchmarkAlloc_Map_RawLoop(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := make([]int, len(data))
		for j, x := range data {
			result[j] = square(x)
		}
		_ = result
	}
}

// =============================================================================
// Chained Operations - Multiple allocations per item
// =============================================================================

func BenchmarkAlloc_Chain_Rill(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		step1 := rill.Map(stream, 1, func(x int) (int, error) {
			return square(x), nil
		})
		step2 := rill.Map(step1, 1, func(x int) (int, error) {
			return square(x), nil
		})
		step3 := rill.Map(step2, 1, func(x int) (int, error) {
			return square(x), nil
		})
		_, _ = rill.ToSlice(step3)
	}
}

func BenchmarkAlloc_Chain_Lo(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		step1 := lo.Map(data, func(x int, _ int) int {
			return square(x)
		})
		step2 := lo.Map(step1, func(x int, _ int) int {
			return square(x)
		})
		_ = lo.Map(step2, func(x int, _ int) int {
			return square(x)
		})
	}
}

func BenchmarkAlloc_Chain_GoLinq(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var result []int
		linq.From(data).
			SelectT(func(x int) int { return square(x) }).
			SelectT(func(x int) int { return square(x) }).
			SelectT(func(x int) int { return square(x) }).
			ToSlice(&result)
	}
}

func BenchmarkAlloc_Chain_RawLoop(b *testing.B) {
	data := generateInts(AllocSize)
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		step1 := make([]int, len(data))
		for j, x := range data {
			step1[j] = square(x)
		}
		step2 := make([]int, len(step1))
		for j, x := range step1 {
			step2[j] = square(x)
		}
		step3 := make([]int, len(step2))
		for j, x := range step2 {
			step3[j] = square(x)
		}
		_ = step3
	}
}
