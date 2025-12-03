package core

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// =============================================================================
// Channel Buffer Size Tuning Benchmarks
// =============================================================================
//
// This file contains benchmarks to determine optimal buffer sizes for different
// workload characteristics:
// - Fast producers with slow consumers
// - Slow producers with fast consumers
// - Balanced producer/consumer speeds
// - CPU-bound vs I/O-bound operations
// - Different item counts

// BenchmarkBufferSize_FastProducerSlowConsumer tests scenarios where production
// is faster than consumption (e.g., generating data faster than processing it)
func BenchmarkBufferSize_FastProducerSlowConsumer(b *testing.B) {
	bufferSizes := []int{0, 1, 8, 16, 32, 64, 128, 256, 512, 1024}
	itemCount := 1000

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			// Fast producer: just creates items
			// Slow consumer: simulates processing delay
			mapper := Map(func(x int) (int, error) {
				// Simulate slow processing (100ns per item)
				start := time.Now()
				for time.Since(start) < 100*time.Nanosecond {
				}
				return x * 2, nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := fastProducer(itemCount)
				mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkBufferSize_SlowProducerFastConsumer tests scenarios where consumption
// is faster than production (e.g., waiting for I/O)
func BenchmarkBufferSize_SlowProducerFastConsumer(b *testing.B) {
	bufferSizes := []int{0, 1, 8, 16, 32, 64, 128, 256}
	itemCount := 500

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			// Consumer is fast, producer has simulated delay
			mapper := Map(func(x int) (int, error) {
				return x * 2, nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := slowProducer(itemCount, 50*time.Nanosecond)
				mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkBufferSize_CPUBound tests CPU-intensive transformations
func BenchmarkBufferSize_CPUBound(b *testing.B) {
	bufferSizes := []int{0, 1, 8, 16, 32, 64, 128, 256, 512}
	itemCount := 1000

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			// CPU-bound work: compute-heavy transformation
			mapper := Map(func(x int) (int, error) {
				// Simulate CPU work
				result := x
				for j := 0; j < 100; j++ {
					result = (result * 31) ^ (result >> 3)
				}
				return result, nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := fastProducer(itemCount)
				mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkBufferSize_Trivial tests minimal transformation (best case for buffer)
func BenchmarkBufferSize_Trivial(b *testing.B) {
	bufferSizes := []int{0, 1, 8, 16, 32, 64, 128, 256, 512, 1024}
	itemCount := 10000

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) {
				return x * 2, nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := fastProducer(itemCount)
				mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkBufferSize_FlatMap tests FlatMap with different buffer sizes
func BenchmarkBufferSize_FlatMap(b *testing.B) {
	bufferSizes := []int{0, 1, 8, 16, 32, 64, 128, 256}
	itemCount := 1000
	outputsPerItem := 5

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
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
				stream := fastProducer(itemCount)
				mapped := fm.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkBufferSize_ChainedMappers tests buffer size with multiple chained transforms
func BenchmarkBufferSize_ChainedMappers(b *testing.B) {
	bufferSizes := []int{0, 1, 16, 32, 64, 128, 256}
	itemCount := 5000

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			m1 := Map(func(x int) (int, error) { return x * 2, nil })
			m2 := Map(func(x int) (int, error) { return x + 1, nil })
			m3 := Map(func(x int) (int, error) { return x * 3, nil })

			opts := []TransformOption{WithBufferSize(bufSize)}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := fastProducer(itemCount)
				s1 := m1.ApplyWith(ctx, stream, opts...)
				s2 := m2.ApplyWith(ctx, s1, opts...)
				s3 := m3.ApplyWith(ctx, s2, opts...)
				_, _ = Slice(ctx, s3)
			}
		})
	}
}

// BenchmarkBufferSize_VaryingItemCount tests how buffer size scales with item count
func BenchmarkBufferSize_VaryingItemCount(b *testing.B) {
	itemCounts := []int{100, 1000, 10000, 100000}
	bufferSizes := []int{16, 64, 256}

	for _, itemCount := range itemCounts {
		for _, bufSize := range bufferSizes {
			b.Run(itemsBufName(itemCount, bufSize), func(b *testing.B) {
				mapper := Map(func(x int) (int, error) { return x * 2, nil })

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					ctx := context.Background()
					stream := fastProducer(itemCount)
					mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
					_, _ = Slice(ctx, mapped)
				}
			})
		}
	}
}

// BenchmarkBufferSize_MemoryPressure tests buffer size under memory pressure
func BenchmarkBufferSize_LargeItems(b *testing.B) {
	bufferSizes := []int{1, 8, 16, 32, 64, 128}
	itemCount := 1000

	// Large item: 1KB struct
	type largeItem struct {
		data [1024]byte
		id   int
	}

	for _, bufSize := range bufferSizes {
		b.Run(bufName(bufSize), func(b *testing.B) {
			mapper := Map(func(x largeItem) (largeItem, error) {
				x.id = x.id * 2
				return x, nil
			})

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := Emit(func(ctx context.Context) <-chan Result[largeItem] {
					ch := make(chan Result[largeItem], 64)
					go func() {
						defer close(ch)
						for j := 0; j < itemCount; j++ {
							ch <- Ok(largeItem{id: j})
						}
					}()
					return ch
				})
				mapped := mapper.ApplyWith(ctx, stream, WithBufferSize(bufSize))
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// =============================================================================
// Helper functions
// =============================================================================

func bufName(size int) string {
	return "buf_" + strconv.Itoa(size)
}

func itemsBufName(items, buf int) string {
	return "items_" + strconv.Itoa(items) + "_buf_" + strconv.Itoa(buf)
}

// fastProducer creates a stream that produces items as fast as possible
func fastProducer(count int) Stream[int] {
	return Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], 64)
		go func() {
			defer close(ch)
			for i := 0; i < count; i++ {
				ch <- Ok(i)
			}
		}()
		return ch
	})
}

// slowProducer creates a stream that produces items with a delay
func slowProducer(count int, delay time.Duration) Stream[int] {
	return Emit(func(ctx context.Context) <-chan Result[int] {
		ch := make(chan Result[int], 64)
		go func() {
			defer close(ch)
			for i := 0; i < count; i++ {
				start := time.Now()
				for time.Since(start) < delay {
				}
				ch <- Ok(i)
			}
		}()
		return ch
	})
}
