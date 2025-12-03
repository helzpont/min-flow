package benchmarks

import (
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/destel/rill"
	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/samber/lo"
)

// =============================================================================
// Reduce Benchmarks
// =============================================================================

func BenchmarkReduce_MinFlow_Small(b *testing.B) {
	benchmarkReduceMinFlow(b, SmallSize)
}

func BenchmarkReduce_MinFlow_Medium(b *testing.B) {
	benchmarkReduceMinFlow(b, MediumSize)
}

func BenchmarkReduce_MinFlow_Large(b *testing.B) {
	benchmarkReduceMinFlow(b, LargeSize)
}

func benchmarkReduceMinFlow(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		reduced := aggregate.Reduce(add).Apply(ctx, stream)
		_, _ = core.Slice(ctx, reduced)
	}
}

func BenchmarkReduce_Rill_Small(b *testing.B) {
	benchmarkReduceRill(b, SmallSize)
}

func BenchmarkReduce_Rill_Medium(b *testing.B) {
	benchmarkReduceRill(b, MediumSize)
}

func BenchmarkReduce_Rill_Large(b *testing.B) {
	benchmarkReduceRill(b, LargeSize)
}

func benchmarkReduceRill(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		_, _, _ = rill.Reduce(stream, 1, func(a, b int) (int, error) {
			return add(a, b), nil
		})
	}
}

func BenchmarkReduce_Lo_Small(b *testing.B) {
	benchmarkReduceLo(b, SmallSize)
}

func BenchmarkReduce_Lo_Medium(b *testing.B) {
	benchmarkReduceLo(b, MediumSize)
}

func BenchmarkReduce_Lo_Large(b *testing.B) {
	benchmarkReduceLo(b, LargeSize)
}

func benchmarkReduceLo(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = lo.Reduce(data, func(acc int, x int, _ int) int {
			return add(acc, x)
		}, 0)
	}
}

func BenchmarkReduce_GoLinq_Small(b *testing.B) {
	benchmarkReduceGoLinq(b, SmallSize)
}

func BenchmarkReduce_GoLinq_Medium(b *testing.B) {
	benchmarkReduceGoLinq(b, MediumSize)
}

func BenchmarkReduce_GoLinq_Large(b *testing.B) {
	benchmarkReduceGoLinq(b, LargeSize)
}

func benchmarkReduceGoLinq(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = linq.From(data).AggregateT(func(acc, x int) int {
			return add(acc, x)
		})
	}
}

func BenchmarkReduce_RawLoop_Small(b *testing.B) {
	benchmarkReduceRawLoop(b, SmallSize)
}

func BenchmarkReduce_RawLoop_Medium(b *testing.B) {
	benchmarkReduceRawLoop(b, MediumSize)
}

func BenchmarkReduce_RawLoop_Large(b *testing.B) {
	benchmarkReduceRawLoop(b, LargeSize)
}

func benchmarkReduceRawLoop(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sum := 0
		for _, x := range data {
			sum = add(sum, x)
		}
		_ = sum
	}
}
