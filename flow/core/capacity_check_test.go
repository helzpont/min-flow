package core

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// Capacity-Based Context Checking Benchmarks
// =============================================================================

// BenchmarkContextCheckStrategies compares different context checking approaches
func BenchmarkContextCheckStrategies(b *testing.B) {
	const itemCount = 10000

	data := make([]int, itemCount)
	for i := range data {
		data[i] = i
	}

	// Create a stream source
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

	// Strategy 1: Current approach - check on every send
	b.Run("every_item_check", func(b *testing.B) {
		mapper := Map(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream(ctx)
			mapped := mapper.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})

	// Strategy 2: Capacity-based checking
	b.Run("capacity_based_check", func(b *testing.B) {
		mapper := NewCapacityMapper(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeStream(ctx)
			mapped := mapper.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})

	// Strategy 3: No context check (baseline - fast package style)
	b.Run("no_context_check", func(b *testing.B) {
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

	_ = ctx // use ctx to avoid unused variable warning
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

	b.Run("3_stage_every_item", func(b *testing.B) {
		m1 := Map(func(x int) (int, error) { return x * 2, nil })
		m2 := Map(func(x int) (int, error) { return x + 1, nil })
		m3 := Map(func(x int) (int, error) { return x / 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			s := makeStream(ctx)
			s = m1.Apply(ctx, s)
			s = m2.Apply(ctx, s)
			s = m3.Apply(ctx, s)
			_, _ = Slice(ctx, s)
		}
	})

	b.Run("3_stage_capacity_based", func(b *testing.B) {
		m1 := NewCapacityMapper(func(x int) (int, error) { return x * 2, nil })
		m2 := NewCapacityMapper(func(x int) (int, error) { return x + 1, nil })
		m3 := NewCapacityMapper(func(x int) (int, error) { return x / 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			s := makeStream(ctx)
			s = m1.Apply(ctx, s)
			s = m2.Apply(ctx, s)
			s = m3.Apply(ctx, s)
			_, _ = Slice(ctx, s)
		}
	})
}

// BenchmarkBufferSizeImpact tests how buffer size affects the optimization
func BenchmarkBufferSizeImpact(b *testing.B) {
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

	for _, bufSize := range []int{1, 8, 32, 64, 128, 256} {
		b.Run("every_item_buf_"+itoa(bufSize), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) { return x * 2, nil })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				s := makeStream(ctx, bufSize)
				s = mapper.ApplyWith(ctx, s, WithBufferSize(bufSize))
				_, _ = Slice(ctx, s)
			}
		})

		b.Run("capacity_based_buf_"+itoa(bufSize), func(b *testing.B) {
			mapper := NewCapacityMapper(func(x int) (int, error) { return x * 2, nil })
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				s := makeStream(ctx, bufSize)
				s = mapper.ApplyWith(ctx, s, bufSize)
				_, _ = Slice(ctx, s)
			}
		})
	}
}

// simple int to string for benchmark names
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

// TestCapacityMapperCorrectness ensures the capacity-based mapper produces correct results
func TestCapacityMapperCorrectness(t *testing.T) {
	ctx := context.Background()

	// Create test data
	data := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// Create stream
	stream := Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], len(data))
		for _, v := range data {
			ch <- Ok(v)
		}
		close(ch)
		return ch
	})

	// Apply capacity-based mapper
	mapper := NewCapacityMapper(func(x int) (int, error) { return x * 2, nil })
	mapped := mapper.Apply(ctx, stream)

	// Collect results
	results, err := Slice(ctx, mapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify
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

// TestCapacityMapperCancellation ensures cancellation still works
func TestCapacityMapperCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a large stream
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

	// Apply capacity-based mapper
	mapper := NewCapacityMapper(func(x int) (int, error) {
		time.Sleep(time.Microsecond) // Slow it down
		return x * 2, nil
	})
	mapped := mapper.Apply(ctx, stream)

	// Start consuming
	resultCh := mapped.Emit(ctx)

	// Read a few items
	count := 0
	for range resultCh {
		count++
		if count >= 100 {
			cancel()
			break
		}
	}

	// Drain remaining (should stop soon due to cancellation)
	remaining := 0
	for range resultCh {
		remaining++
	}

	// With buffer size 64 and capacity-based checking, we might get up to
	// bufferSize more items after cancel (worst case)
	if remaining > DefaultBufferSize*2 {
		t.Errorf("expected at most %d remaining items, got %d", DefaultBufferSize*2, remaining)
	}

	t.Logf("Processed %d items before cancel, %d after (expected ≤ %d after)",
		count, remaining, DefaultBufferSize*2)
}
