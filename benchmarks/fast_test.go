package benchmarks

import (
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/fast"
	"github.com/lguimbarda/min-flow/flow/filter"
)

// =============================================================================
// Fast vs Core Comparison Benchmarks
// =============================================================================

// BenchmarkFastVsCore_Map compares fast.Map vs core.Map
func BenchmarkFastVsCore_Map(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("core.Map", func(b *testing.B) {
		mapper := core.Map(squareWithErr)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			mapped := mapper.Apply(ctx, stream)
			_, _ = core.Slice(ctx, mapped)
		}
	})

	b.Run("fast.Map", func(b *testing.B) {
		mapper := fast.Map(square)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			mapped := mapper.Apply(ctx, stream)
			_ = fast.Slice(ctx, mapped)
		}
	})
}

// BenchmarkFastVsCore_MapFilterReduce compares a full pipeline
func BenchmarkFastVsCore_MapFilterReduce(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("core.Pipeline", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			mapped := core.Map(squareWithErr).Apply(ctx, stream)
			filtered := filter.Where(isEven).Apply(ctx, mapped)
			reduced := aggregate.Reduce(add).Apply(ctx, filtered)
			_, _ = core.Slice(ctx, reduced)
		}
	})

	b.Run("fast.Pipeline", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			mapped := fast.Map(square).Apply(ctx, stream)
			filtered := fast.Filter(isEven).Apply(ctx, mapped)
			reduced := fast.Reduce(add).Apply(ctx, filtered)
			_ = fast.Slice(ctx, reduced)
		}
	})
}

// BenchmarkFastVsCore_Fused compares fused mappers
func BenchmarkFastVsCore_Fused(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("core.Fused", func(b *testing.B) {
		addOne := core.Map(func(x int) (int, error) { return x + 1, nil })
		double := core.Map(func(x int) (int, error) { return x * 2, nil })
		addTen := core.Map(func(x int) (int, error) { return x + 10, nil })
		fused := core.Fuse(core.Fuse(addOne, double), addTen)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			mapped := fused.Apply(ctx, stream)
			_, _ = core.Slice(ctx, mapped)
		}
	})

	b.Run("fast.Fused", func(b *testing.B) {
		addOne := fast.Map(func(x int) int { return x + 1 })
		double := fast.Map(func(x int) int { return x * 2 })
		addTen := fast.Map(func(x int) int { return x + 10 })
		fused := fast.Fuse(fast.Fuse(addOne, double), addTen)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			mapped := fused.Apply(ctx, stream)
			_ = fast.Slice(ctx, mapped)
		}
	})
}

// BenchmarkFastVsCore_LongChain compares a longer chain of operations
func BenchmarkFastVsCore_LongChain(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("core.LongChain", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			s1 := core.Map(func(x int) (int, error) { return x + 1, nil }).Apply(ctx, stream)
			s2 := filter.Where(isEven).Apply(ctx, s1)
			s3 := core.Map(func(x int) (int, error) { return x * 2, nil }).Apply(ctx, s2)
			s4 := filter.Take[int](1000).Apply(ctx, s3)
			s5 := core.Map(func(x int) (int, error) { return x - 1, nil }).Apply(ctx, s4)
			_, _ = core.Slice(ctx, s5)
		}
	})

	b.Run("fast.LongChain", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			s1 := fast.Map(func(x int) int { return x + 1 }).Apply(ctx, stream)
			s2 := fast.Filter(isEven).Apply(ctx, s1)
			s3 := fast.Map(func(x int) int { return x * 2 }).Apply(ctx, s2)
			s4 := fast.Take[int](1000).Apply(ctx, s3)
			s5 := fast.Map(func(x int) int { return x - 1 }).Apply(ctx, s4)
			_ = fast.Slice(ctx, s5)
		}
	})
}

// =============================================================================
// Feature Overhead Isolation Benchmarks
// =============================================================================

// BenchmarkOverhead_ResultWrapping measures the cost of Result[T] vs direct values
func BenchmarkOverhead_ResultWrapping(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("direct_values", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			_ = fast.Slice(ctx, stream)
		}
	})

	b.Run("result_wrapped", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			_, _ = core.Slice(ctx, stream)
		}
	})
}

// BenchmarkOverhead_PanicRecovery measures the cost of defer/recover
func BenchmarkOverhead_PanicRecovery(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("no_recovery", func(b *testing.B) {
		mapper := fast.Map(square)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			mapped := mapper.Apply(ctx, stream)
			_ = fast.Slice(ctx, mapped)
		}
	})

	b.Run("with_recovery", func(b *testing.B) {
		mapper := core.Map(squareWithErr)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			mapped := mapper.Apply(ctx, stream)
			_, _ = core.Slice(ctx, mapped)
		}
	})
}

// BenchmarkOverhead_ErrorSignature measures the cost of error return values
func BenchmarkOverhead_ErrorSignature(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("no_error_return", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			mapped := fast.Map(square).Apply(ctx, stream)
			_ = fast.Slice(ctx, mapped)
		}
	})

	b.Run("with_error_return", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			mapped := core.Map(squareWithErr).Apply(ctx, stream)
			_, _ = core.Slice(ctx, mapped)
		}
	})
}

// BenchmarkOverhead_ContextCheck measures the cost of per-item context checks
// Note: Both implementations check context at channel operations,
// but core also checks more frequently in some paths.
func BenchmarkOverhead_ContextCheck(b *testing.B) {
	data := generateInts(LargeSize)

	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := fast.FromSlice(data)
			_ = fast.Slice(ctx, stream)
		}
	})

	b.Run("core", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			stream := flow.FromSlice(data)
			_, _ = core.Slice(ctx, stream)
		}
	})
}
