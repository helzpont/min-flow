package benchmarks

import (
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
)

// =============================================================================
// Stream Creation Benchmarks
// These benchmarks measure the overhead of creating streams from various sources.
// Run with: go test -bench=BenchmarkStream -benchmem
// =============================================================================

// -----------------------------------------------------------------------------
// FromSlice - Most common stream source
// -----------------------------------------------------------------------------

func BenchmarkStream_FromSlice_Small(b *testing.B) {
	data := generateInts(SmallSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_FromSlice_Medium(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_FromSlice_Large(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

// -----------------------------------------------------------------------------
// FromChannel - Channel-based stream source
// -----------------------------------------------------------------------------

func BenchmarkStream_FromChannel_Small(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch := make(chan int, SmallSize)
		go func() {
			for j := 0; j < SmallSize; j++ {
				ch <- j
			}
			close(ch)
		}()
		stream := flow.FromChannel(ch)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_FromChannel_Medium(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch := make(chan int, MediumSize)
		go func() {
			for j := 0; j < MediumSize; j++ {
				ch <- j
			}
			close(ch)
		}()
		stream := flow.FromChannel(ch)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_FromChannel_Large(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch := make(chan int, LargeSize)
		go func() {
			for j := 0; j < LargeSize; j++ {
				ch <- j
			}
			close(ch)
		}()
		stream := flow.FromChannel(ch)
		_, _ = core.Slice(ctx, stream)
	}
}

// -----------------------------------------------------------------------------
// Range - Numeric range generation
// -----------------------------------------------------------------------------

func BenchmarkStream_Range_Small(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.Range(0, SmallSize)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_Range_Medium(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.Range(0, MediumSize)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkStream_Range_Large(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.Range(0, LargeSize)
		_, _ = core.Slice(ctx, stream)
	}
}

// -----------------------------------------------------------------------------
// Just - Single value stream
// -----------------------------------------------------------------------------

func BenchmarkStream_Once(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.Once(42)
		_, _ = core.Slice(ctx, stream)
	}
}

// -----------------------------------------------------------------------------
// Empty - Empty stream
// -----------------------------------------------------------------------------

func BenchmarkStream_Empty(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.Empty[int]()
		_, _ = core.Slice(ctx, stream)
	}
}

// =============================================================================
// Result Type Benchmarks
// Measure the overhead of Result wrapping/unwrapping
// =============================================================================

func BenchmarkResult_Ok(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := core.Ok(42)
		_ = r.Value()
	}
}

func BenchmarkResult_Err(b *testing.B) {
	err := errTest
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := core.Err[int](err)
		_ = r.Error()
	}
}

func BenchmarkResult_IsValue(b *testing.B) {
	r := core.Ok(42)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = r.IsValue()
	}
}

func BenchmarkResult_IsError(b *testing.B) {
	r := core.Err[int](errTest)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = r.IsError()
	}
}

// =============================================================================
// Terminal Operations
// =============================================================================

func BenchmarkTerminal_Slice_Small(b *testing.B) {
	data := generateInts(SmallSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkTerminal_Slice_Large(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkTerminal_First(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.First(ctx, stream)
	}
}

func BenchmarkTerminal_Run(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_ = core.Run(ctx, stream)
	}
}

// =============================================================================
// Transformer Chain Benchmarks
// Measure the overhead of chaining multiple transformers
// =============================================================================

func BenchmarkChain_1Transformer(b *testing.B) {
	data := generateInts(MediumSize)
	t1 := core.Map(squareWithErr)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := t1.Apply(stream)
		_, _ = core.Slice(ctx, s1)
	}
}

func BenchmarkChain_2Transformers(b *testing.B) {
	data := generateInts(MediumSize)
	t1 := core.Map(squareWithErr)
	t2 := core.Map(func(x int) (int, error) { return x + 1, nil })
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := t1.Apply(stream)
		s2 := t2.Apply(s1)
		_, _ = core.Slice(ctx, s2)
	}
}

func BenchmarkChain_3Transformers(b *testing.B) {
	data := generateInts(MediumSize)
	t1 := core.Map(squareWithErr)
	t2 := core.Map(func(x int) (int, error) { return x + 1, nil })
	t3 := core.Map(func(x int) (int, error) { return x * 2, nil })
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := t1.Apply(stream)
		s2 := t2.Apply(s1)
		s3 := t3.Apply(s2)
		_, _ = core.Slice(ctx, s3)
	}
}

func BenchmarkChain_5Transformers(b *testing.B) {
	data := generateInts(MediumSize)
	t1 := core.Map(squareWithErr)
	t2 := core.Map(func(x int) (int, error) { return x + 1, nil })
	t3 := core.Map(func(x int) (int, error) { return x * 2, nil })
	t4 := core.Map(func(x int) (int, error) { return x - 1, nil })
	t5 := core.Map(func(x int) (int, error) { return x / 2, nil })
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := t1.Apply(stream)
		s2 := t2.Apply(s1)
		s3 := t3.Apply(s2)
		s4 := t4.Apply(s3)
		s5 := t5.Apply(s4)
		_, _ = core.Slice(ctx, s5)
	}
}

// =============================================================================
// Fusion Benchmarks
// Compare fused vs unfused transformer chains
// =============================================================================

func BenchmarkFusion_3Unfused(b *testing.B) {
	data := generateInts(LargeSize)
	addOne := core.Map(func(x int) (int, error) { return x + 1, nil })
	double := core.Map(func(x int) (int, error) { return x * 2, nil })
	addTen := core.Map(func(x int) (int, error) { return x + 10, nil })
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := addOne.Apply(stream)
		s2 := double.Apply(s1)
		s3 := addTen.Apply(s2)
		_, _ = core.Slice(ctx, s3)
	}
}

func BenchmarkFusion_3Fused(b *testing.B) {
	data := generateInts(LargeSize)
	addOne := core.Map(func(x int) (int, error) { return x + 1, nil })
	double := core.Map(func(x int) (int, error) { return x * 2, nil })
	addTen := core.Map(func(x int) (int, error) { return x + 10, nil })
	fused := core.Fuse(core.Fuse(addOne, double), addTen)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s := fused.Apply(stream)
		_, _ = core.Slice(ctx, s)
	}
}

func BenchmarkFusion_5Unfused(b *testing.B) {
	data := generateInts(LargeSize)
	t1 := core.Map(func(x int) (int, error) { return x + 1, nil })
	t2 := core.Map(func(x int) (int, error) { return x * 2, nil })
	t3 := core.Map(func(x int) (int, error) { return x + 10, nil })
	t4 := core.Map(func(x int) (int, error) { return x - 5, nil })
	t5 := core.Map(func(x int) (int, error) { return x * 3, nil })
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s1 := t1.Apply(stream)
		s2 := t2.Apply(s1)
		s3 := t3.Apply(s2)
		s4 := t4.Apply(s3)
		s5 := t5.Apply(s4)
		_, _ = core.Slice(ctx, s5)
	}
}

func BenchmarkFusion_5Fused(b *testing.B) {
	data := generateInts(LargeSize)
	t1 := core.Map(func(x int) (int, error) { return x + 1, nil })
	t2 := core.Map(func(x int) (int, error) { return x * 2, nil })
	t3 := core.Map(func(x int) (int, error) { return x + 10, nil })
	t4 := core.Map(func(x int) (int, error) { return x - 5, nil })
	t5 := core.Map(func(x int) (int, error) { return x * 3, nil })
	fused := core.Fuse(core.Fuse(core.Fuse(core.Fuse(t1, t2), t3), t4), t5)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		s := fused.Apply(stream)
		_, _ = core.Slice(ctx, s)
	}
}
