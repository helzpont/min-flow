package core

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"
)

// =============================================================================
// Interceptor Overhead Benchmarks
// =============================================================================
//
// These benchmarks measure the overhead of the interceptor system:
// - Registry lookup cost
// - Event matching overhead
// - Per-item interceptor invocation
// - Multiple interceptors

// NoOpInterceptor is a minimal interceptor for benchmarking pure overhead
type NoOpInterceptor struct {
	events []Event
	calls  atomic.Int64
}

func NewNoOpInterceptor(events ...Event) *NoOpInterceptor {
	return &NoOpInterceptor{events: events}
}

func (n *NoOpInterceptor) Init() error                                  { return nil }
func (n *NoOpInterceptor) Close() error                                 { return nil }
func (n *NoOpInterceptor) Events() []Event                              { return n.events }
func (n *NoOpInterceptor) Do(_ context.Context, _ Event, _ ...any) error { n.calls.Add(1); return nil }
func (n *NoOpInterceptor) Calls() int64                                 { return n.calls.Load() }

// CountingInterceptor counts invocations per event type
type CountingInterceptor struct {
	events      []Event
	streamStart atomic.Int64
	streamEnd   atomic.Int64
	itemRecv    atomic.Int64
	valueRecv   atomic.Int64
}

func NewCountingInterceptor(events ...Event) *CountingInterceptor {
	return &CountingInterceptor{events: events}
}

func (c *CountingInterceptor) Init() error     { return nil }
func (c *CountingInterceptor) Close() error    { return nil }
func (c *CountingInterceptor) Events() []Event { return c.events }
func (c *CountingInterceptor) Do(_ context.Context, event Event, _ ...any) error {
	switch event {
	case StreamStart:
		c.streamStart.Add(1)
	case StreamEnd:
		c.streamEnd.Add(1)
	case ItemReceived:
		c.itemRecv.Add(1)
	case ValueReceived:
		c.valueRecv.Add(1)
	}
	return nil
}

// BenchmarkInterceptor_Baseline measures stream processing without interceptors
func BenchmarkInterceptor_Baseline(b *testing.B) {
	itemCounts := []int{100, 1000, 10000}

	for _, count := range itemCounts {
		b.Run("items_"+strconv.Itoa(count), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) { return x * 2, nil })

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx := context.Background()
				stream := makeIntStream(count)
				mapped := mapper.Apply(ctx, stream)
				_, _ = Slice(ctx, mapped)
			}
		})
	}
}

// BenchmarkInterceptor_WithRegistry measures overhead of having a registry (no interceptors)
func BenchmarkInterceptor_WithRegistry(b *testing.B) {
	itemCounts := []int{100, 1000, 10000}

	for _, count := range itemCounts {
		b.Run("items_"+strconv.Itoa(count), func(b *testing.B) {
			mapper := Map(func(x int) (int, error) { return x * 2, nil })

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, _ := WithRegistry(context.Background())
				stream := makeIntStream(count)
				mapped := mapper.Apply(ctx, stream)
				// Add intercept stage
				intercepted := Intercept[int]().Apply(ctx, mapped)
				_, _ = Slice(ctx, intercepted)
			}
		})
	}
}

// BenchmarkInterceptor_SingleInterceptor measures overhead with one interceptor
func BenchmarkInterceptor_SingleInterceptor(b *testing.B) {
	itemCounts := []int{100, 1000, 10000}

	for _, count := range itemCounts {
		b.Run("items_"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, registry := WithRegistry(context.Background())
				interceptor := NewNoOpInterceptor(StreamStart, StreamEnd, ItemReceived)
				_ = registry.Register(interceptor)

				mapper := Map(func(x int) (int, error) { return x * 2, nil })
				stream := makeIntStream(count)
				mapped := mapper.Apply(ctx, stream)
				intercepted := Intercept[int]().Apply(ctx, mapped)
				_, _ = Slice(ctx, intercepted)
			}
		})
	}
}

// BenchmarkInterceptor_MultipleInterceptors measures overhead with multiple interceptors
func BenchmarkInterceptor_MultipleInterceptors(b *testing.B) {
	interceptorCounts := []int{1, 2, 4, 8}
	itemCount := 1000

	for _, numInterceptors := range interceptorCounts {
		b.Run("interceptors_"+strconv.Itoa(numInterceptors), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, registry := WithRegistry(context.Background())
				for j := 0; j < numInterceptors; j++ {
					interceptor := NewNoOpInterceptor(ItemReceived)
					_ = registry.Register(interceptor)
				}

				mapper := Map(func(x int) (int, error) { return x * 2, nil })
				stream := makeIntStream(itemCount)
				mapped := mapper.Apply(ctx, stream)
				intercepted := Intercept[int]().Apply(ctx, mapped)
				_, _ = Slice(ctx, intercepted)
			}
		})
	}
}

// BenchmarkInterceptor_EventMatching measures overhead of event pattern matching
func BenchmarkInterceptor_EventMatching(b *testing.B) {
	patterns := []struct {
		name   string
		events []Event
	}{
		{"exact_match", []Event{ItemReceived}},
		{"multiple_exact", []Event{StreamStart, StreamEnd, ItemReceived, ValueReceived}},
		{"wildcard_all", []Event{Event(AllEvents)}},
		{"wildcard_prefix", []Event{Event(AllItemEvents)}},
	}

	itemCount := 1000

	for _, p := range patterns {
		b.Run(p.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ctx, registry := WithRegistry(context.Background())
				interceptor := NewNoOpInterceptor(p.events...)
				_ = registry.Register(interceptor)

				mapper := Map(func(x int) (int, error) { return x * 2, nil })
				stream := makeIntStream(itemCount)
				mapped := mapper.Apply(ctx, stream)
				intercepted := Intercept[int]().Apply(ctx, mapped)
				_, _ = Slice(ctx, intercepted)
			}
		})
	}
}

// BenchmarkInterceptor_RegistryLookup measures registry lookup overhead
func BenchmarkInterceptor_RegistryLookup(b *testing.B) {
	b.Run("empty_registry", func(b *testing.B) {
		ctx, _ := WithRegistry(context.Background())
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry, ok := GetRegistry(ctx)
			if ok {
				_ = registry.Interceptors()
			}
		}
	})

	b.Run("one_interceptor", func(b *testing.B) {
		ctx, registry := WithRegistry(context.Background())
		_ = registry.Register(NewNoOpInterceptor(ItemReceived))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry, ok := GetRegistry(ctx)
			if ok {
				_ = registry.Interceptors()
			}
		}
	})

	b.Run("ten_interceptors", func(b *testing.B) {
		ctx, registry := WithRegistry(context.Background())
		for i := 0; i < 10; i++ {
			_ = registry.Register(NewNoOpInterceptor(ItemReceived))
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry, ok := GetRegistry(ctx)
			if ok {
				_ = registry.Interceptors()
			}
		}
	})
}

// BenchmarkInterceptor_EventMatchingMicro measures Event.Matches performance
func BenchmarkInterceptor_EventMatchingMicro(b *testing.B) {
	event := ItemReceived

	b.Run("exact_match", func(b *testing.B) {
		pattern := string(ItemReceived)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = event.Matches(pattern)
		}
	})

	b.Run("wildcard_all", func(b *testing.B) {
		pattern := AllEvents
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = event.Matches(pattern)
		}
	})

	b.Run("wildcard_prefix", func(b *testing.B) {
		pattern := AllItemEvents
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = event.Matches(pattern)
		}
	})

	b.Run("wildcard_suffix", func(b *testing.B) {
		pattern := AllStartEvents
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = event.Matches(pattern)
		}
	})

	b.Run("no_match", func(b *testing.B) {
		pattern := string(StreamStart)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = event.Matches(pattern)
		}
	})
}

// BenchmarkInterceptor_PerItemOverhead measures per-item interceptor overhead precisely
func BenchmarkInterceptor_PerItemOverhead(b *testing.B) {
	itemCount := 10000

	b.Run("no_intercept", func(b *testing.B) {
		mapper := Map(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			stream := makeIntStream(itemCount)
			mapped := mapper.Apply(ctx, stream)
			_, _ = Slice(ctx, mapped)
		}
	})

	b.Run("with_intercept_no_interceptors", func(b *testing.B) {
		mapper := Map(func(x int) (int, error) { return x * 2, nil })
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, _ := WithRegistry(context.Background())
			stream := makeIntStream(itemCount)
			mapped := mapper.Apply(ctx, stream)
			intercepted := Intercept[int]().Apply(ctx, mapped)
			_, _ = Slice(ctx, intercepted)
		}
	})

	b.Run("with_intercept_one_noop", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, registry := WithRegistry(context.Background())
			_ = registry.Register(NewNoOpInterceptor(ItemReceived))

			mapper := Map(func(x int) (int, error) { return x * 2, nil })
			stream := makeIntStream(itemCount)
			mapped := mapper.Apply(ctx, stream)
			intercepted := Intercept[int]().Apply(ctx, mapped)
			_, _ = Slice(ctx, intercepted)
		}
	})

	b.Run("with_intercept_counting", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, registry := WithRegistry(context.Background())
			_ = registry.Register(NewCountingInterceptor(StreamStart, StreamEnd, ItemReceived, ValueReceived))

			mapper := Map(func(x int) (int, error) { return x * 2, nil })
			stream := makeIntStream(itemCount)
			mapped := mapper.Apply(ctx, stream)
			intercepted := Intercept[int]().Apply(ctx, mapped)
			_, _ = Slice(ctx, intercepted)
		}
	})
}

// =============================================================================
// Helper functions
// =============================================================================

func makeIntStream(count int) Stream[int] {
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
