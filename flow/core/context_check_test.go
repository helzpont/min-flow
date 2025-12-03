package core

import (
	"context"
	"iter"
	"testing"
	"time"
)

// =============================================================================
// Context Check Strategy Benchmarks
// =============================================================================

// BenchmarkContextCheckStrategies compares CheckEveryItem vs CheckOnCapacity
func BenchmarkContextCheckStrategies(b *testing.B) {
	const itemCount = 10000

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	makeStream := func(ctx context.Context) Stream[int] {
		return Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 64)
			go func() {
				defer close(ch)
				for _, v := range data {
					select {
					case <-ctx.Done():
						return
					case ch <- Ok(v):
					}
				}
			}()
			return ch
		})
	}

	b.Run("CheckEveryItem", func(b *testing.B) {
		mapper := Map(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream(ctx)
			mapped := mapper.ApplyWith(ctx, stream, WithFastCancellation())
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("CheckOnCapacity", func(b *testing.B) {
		mapper := Map(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream(ctx)
			mapped := mapper.ApplyWith(ctx, stream, WithHighThroughput())
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("NoContextCheck_baseline", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch := make(chan int, 64)
			go func() {
				defer close(ch)
				for _, v := range data {
					ch <- v * 2
				}
			}()
			result := make([]int, 0, itemCount)
			for v := range ch {
				result = append(result, v)
			}
			_ = result
		}
	})
}

// BenchmarkContextCheckOverhead isolates just the context check cost
func BenchmarkContextCheckOverhead(b *testing.B) {
	ctx := context.Background()

	b.Run("select_ctx_done_default", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			select {
			case <-ctx.Done():
			default:
			}
		}
	})

	b.Run("no_check", func(b *testing.B) {
		b.ReportAllocs()
		var sink int
		for i := 0; i < b.N; i++ {
			sink = i
		}
		_ = sink
	})

	_ = ctx
}

// BenchmarkPipelineContextStrategies compares strategies in a multi-stage pipeline
func BenchmarkPipelineContextStrategies(b *testing.B) {
	const itemCount = 10000

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	makeStream := func(ctx context.Context) Stream[int] {
		return Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 64)
			go func() {
				defer close(ch)
				for _, v := range data {
					ch <- Ok(v)
				}
			}()
			return ch
		})
	}

	b.Run("3_stage_CheckEveryItem", func(b *testing.B) {
		m1 := Map(func(x int) (int, error) { return x * 2, nil })
		m2 := Map(func(x int) (int, error) { return x + 1, nil })
		m3 := Map(func(x int) (int, error) { return x / 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			s := makeStream(ctx)
			s = m1.ApplyWith(ctx, s, WithFastCancellation())
			s = m2.ApplyWith(ctx, s, WithFastCancellation())
			s = m3.ApplyWith(ctx, s, WithFastCancellation())
			_, _ = Slice(ctx, s)
		}
	})

	b.Run("3_stage_CheckOnCapacity", func(b *testing.B) {
		m1 := Map(func(x int) (int, error) { return x * 2, nil })
		m2 := Map(func(x int) (int, error) { return x + 1, nil })
		m3 := Map(func(x int) (int, error) { return x / 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			s := makeStream(ctx)
			s = m1.ApplyWith(ctx, s, WithHighThroughput())
			s = m2.ApplyWith(ctx, s, WithHighThroughput())
			s = m3.ApplyWith(ctx, s, WithHighThroughput())
			_, _ = Slice(ctx, s)
		}
	})
}

// BenchmarkBufferSizeWithStrategy tests how buffer size affects each strategy
func BenchmarkBufferSizeWithStrategy(b *testing.B) {
	const itemCount = 10000

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	makeStream := func(ctx context.Context, bufSize int) Stream[int] {
		return Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], bufSize)
			go func() {
				defer close(ch)
				for _, v := range data {
					ch <- Ok(v)
				}
			}()
			return ch
		})
	}

	for _, bufSize := range []int{8, 64, 256} {
		b.Run("CheckEveryItem_buf_"+itoa(bufSize), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) { return x * 2, nil })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				s := makeStream(ctx, bufSize)
				s = mapper.ApplyWith(ctx, s, WithBufferSize(bufSize), WithFastCancellation())
				_, _ = Slice(ctx, s)
			}
		})

		b.Run("CheckOnCapacity_buf_"+itoa(bufSize), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) { return x * 2, nil })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				s := makeStream(ctx, bufSize)
				s = mapper.ApplyWith(ctx, s, WithBufferSize(bufSize), WithHighThroughput())
				_, _ = Slice(ctx, s)
			}
		})
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// =============================================================================
// Correctness Tests
// =============================================================================

// TestCheckOnCapacityCorrectness ensures capacity-based checking produces correct results
func TestCheckOnCapacityCorrectness(t *testing.T) {
	ctx := context.Background()
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], len(data))
		for _, v := range data {
			ch <- Ok(v)
		}
		close(ch)
		return ch
	})

	mapper := Map(func(x int) (int, error) { return x * 2, nil })
	mapped := mapper.ApplyWith(ctx, stream, WithHighThroughput())

	results, err := Slice(ctx, mapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %d, expected %d", i, v, expected[i])
		}
	}
}

// TestCheckOnCapacityCancellation ensures cancellation still works with capacity-based checking
func TestCheckOnCapacityCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], 64)
		go func() {
			defer close(ch)
			for i := 0; i < 1000000; i++ {
				select {
				case <-ctx.Done():
					return
				case ch <- Ok(i):
				}
			}
		}()
		return ch
	})

	mapper := Map(func(x int) (int, error) {
		time.Sleep(time.Microsecond)
		return x * 2, nil
	})
	mapped := mapper.ApplyWith(ctx, stream, WithHighThroughput())

	resultCh := mapped.Emit(ctx)

	count := 0
	for range resultCh {
		count++
		if count >= 100 {
			cancel()
			break
		}
	}

	remaining := 0
	for range resultCh {
		remaining++
	}

	// With capacity-based checking, we may get up to bufferSize*2 items after cancel
	if remaining > DefaultBufferSize*2 {
		t.Errorf("expected at most %d remaining items, got %d", DefaultBufferSize*2, remaining)
	}

	t.Logf("Processed %d items before cancel, %d after (expected ≤ %d after)",
		count, remaining, DefaultBufferSize*2)
}

// TestFlatMapperWithCheckOnCapacity tests FlatMapper with capacity-based checking
func TestFlatMapperWithCheckOnCapacity(t *testing.T) {
	ctx := context.Background()
	data := []int{1, 2, 3}

	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], len(data))
		for _, v := range data {
			ch <- Ok(v)
		}
		close(ch)
		return ch
	})

	// Each input produces multiple outputs
	flatMapper := FlatMap(func(x int) ([]int, error) {
		return []int{x, x * 10, x * 100}, nil
	})
	mapped := flatMapper.ApplyWith(ctx, stream, WithHighThroughput())

	results, err := Slice(ctx, mapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{1, 10, 100, 2, 20, 200, 3, 30, 300}
	if len(results) != len(expected) {
		t.Fatalf("expected %d results, got %d", len(expected), len(results))
	}
	for i, v := range results {
		if v != expected[i] {
			t.Errorf("results[%d] = %d, expected %d", i, v, expected[i])
		}
	}
}

// =============================================================================
// FlatMapper vs IterFlatMapper Benchmarks
// =============================================================================

// BenchmarkFlatMapperVsIterFlatMapper compares slice-based vs iterator-based FlatMapper
func BenchmarkFlatMapperVsIterFlatMapper(b *testing.B) {
	const itemCount = 10000
	const outputsPerItem = 5

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	makeStream := func() Stream[int] {
		return Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 64)
			go func() {
				defer close(ch)
				for _, v := range data {
					ch <- Ok(v)
				}
			}()
			return ch
		})
	}

	b.Run("FlatMapper_slice", func(b *testing.B) {
		fm := FlatMap(func(x int) ([]int, error) {
			out := make([]int, outputsPerItem)
			for i := range out {
				out[i] = x * (i + 1)
			}
			return out, nil
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream()
			mapped := fm.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("IterFlatMapper_iter", func(b *testing.B) {
		ifm := IterFlatMap(func(x int) iter.Seq[int] {
			return func(yield func(int) bool) {
				for i := 0; i < outputsPerItem; i++ {
					if !yield(x * (i + 1)) {
						return
					}
				}
			}
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream()
			mapped := ifm.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("IterFlatMapSlice_iter", func(b *testing.B) {
		ifm := IterFlatMapSlice(func(x int) ([]int, error) {
			out := make([]int, outputsPerItem)
			for i := range out {
				out[i] = x * (i + 1)
			}
			return out, nil
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream()
			mapped := ifm.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})
}

// BenchmarkFlatMapperVsIterFlatMapper_WithStrategies tests both types with different check strategies
func BenchmarkFlatMapperVsIterFlatMapper_WithStrategies(b *testing.B) {
	const itemCount = 10000
	const outputsPerItem = 3

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	makeStream := func() Stream[int] {
		return Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 64)
			go func() {
				defer close(ch)
				for _, v := range data {
					ch <- Ok(v)
				}
			}()
			return ch
		})
	}

	fm := FlatMap(func(x int) ([]int, error) {
		return []int{x, x * 2, x * 3}, nil
	})

	ifm := IterFlatMap(func(x int) iter.Seq[int] {
		return func(yield func(int) bool) {
			yield(x)
			yield(x * 2)
			yield(x * 3)
		}
	})

	b.Run("FlatMapper_CheckEveryItem", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			mapped := fm.ApplyWith(ctx, makeStream(), WithFastCancellation())
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("FlatMapper_CheckOnCapacity", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			mapped := fm.ApplyWith(ctx, makeStream(), WithHighThroughput())
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("IterFlatMapper_CheckEveryItem", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			mapped := ifm.ApplyWith(ctx, makeStream(), WithFastCancellation())
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("IterFlatMapper_CheckOnCapacity", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			mapped := ifm.ApplyWith(ctx, makeStream(), WithHighThroughput())
			_, _ = Slice(ctx, mapped)
		}
	})
}
