package benchmarks

import (
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/destel/rill"
	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/samber/lo"
)

// =============================================================================
// Pipeline Benchmarks (Map -> Filter -> Reduce)
// =============================================================================

func BenchmarkPipeline_MinFlow_Small(b *testing.B) {
	benchmarkPipelineMinFlow(b, SmallSize)
}

func BenchmarkPipeline_MinFlow_Medium(b *testing.B) {
	benchmarkPipelineMinFlow(b, MediumSize)
}

func BenchmarkPipeline_MinFlow_Large(b *testing.B) {
	benchmarkPipelineMinFlow(b, LargeSize)
}

func benchmarkPipelineMinFlow(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		filtered := filter.Where(isEven).Apply(ctx, mapped)
		reduced := aggregate.Reduce(add).Apply(ctx, filtered)
		_, _ = core.Slice(ctx, reduced)
	}
}

func BenchmarkPipeline_Rill_Small(b *testing.B) {
	benchmarkPipelineRill(b, SmallSize)
}

func BenchmarkPipeline_Rill_Medium(b *testing.B) {
	benchmarkPipelineRill(b, MediumSize)
}

func BenchmarkPipeline_Rill_Large(b *testing.B) {
	benchmarkPipelineRill(b, LargeSize)
}

func benchmarkPipelineRill(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		mapped := rill.Map(stream, 1, func(x int) (int, error) {
			return square(x), nil
		})
		filtered := rill.Filter(mapped, 1, func(x int) (bool, error) {
			return isEven(x), nil
		})
		_, _, _ = rill.Reduce(filtered, 1, func(a, b int) (int, error) {
			return add(a, b), nil
		})
	}
}

// Rill with explicit buffering for fair comparison with min-flow's internal buffering
func BenchmarkPipeline_RillBuffered_Large(b *testing.B) {
	benchmarkPipelineRillBuffered(b, LargeSize)
}

func benchmarkPipelineRillBuffered(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := rill.FromSlice(data, nil)
		stream = rill.Buffer(stream, 64) // match min-flow's DefaultBufferSize
		mapped := rill.Map(stream, 1, func(x int) (int, error) {
			return square(x), nil
		})
		mapped = rill.Buffer(mapped, 64)
		filtered := rill.Filter(mapped, 1, func(x int) (bool, error) {
			return isEven(x), nil
		})
		filtered = rill.Buffer(filtered, 64)
		_, _, _ = rill.Reduce(filtered, 1, func(a, b int) (int, error) {
			return add(a, b), nil
		})
	}
}

func BenchmarkPipeline_Lo_Small(b *testing.B) {
	benchmarkPipelineLo(b, SmallSize)
}

func BenchmarkPipeline_Lo_Medium(b *testing.B) {
	benchmarkPipelineLo(b, MediumSize)
}

func BenchmarkPipeline_Lo_Large(b *testing.B) {
	benchmarkPipelineLo(b, LargeSize)
}

func benchmarkPipelineLo(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mapped := lo.Map(data, func(x int, _ int) int {
			return square(x)
		})
		filtered := lo.Filter(mapped, func(x int, _ int) bool {
			return isEven(x)
		})
		_ = lo.Reduce(filtered, func(acc int, x int, _ int) int {
			return add(acc, x)
		}, 0)
	}
}

func BenchmarkPipeline_GoLinq_Small(b *testing.B) {
	benchmarkPipelineGoLinq(b, SmallSize)
}

func BenchmarkPipeline_GoLinq_Medium(b *testing.B) {
	benchmarkPipelineGoLinq(b, MediumSize)
}

func BenchmarkPipeline_GoLinq_Large(b *testing.B) {
	benchmarkPipelineGoLinq(b, LargeSize)
}

func benchmarkPipelineGoLinq(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = linq.From(data).
			SelectT(func(x int) int { return square(x) }).
			WhereT(func(x int) bool { return isEven(x) }).
			AggregateT(func(acc, x int) int { return add(acc, x) })
	}
}

func BenchmarkPipeline_RawLoop_Small(b *testing.B) {
	benchmarkPipelineRawLoop(b, SmallSize)
}

func BenchmarkPipeline_RawLoop_Medium(b *testing.B) {
	benchmarkPipelineRawLoop(b, MediumSize)
}

func BenchmarkPipeline_RawLoop_Large(b *testing.B) {
	benchmarkPipelineRawLoop(b, LargeSize)
}

func benchmarkPipelineRawLoop(b *testing.B, size int) {
	data := generateInts(size)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sum := 0
		for _, x := range data {
			squared := square(x)
			if isEven(squared) {
				sum = add(sum, squared)
			}
		}
		_ = sum
	}
}
