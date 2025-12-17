package observe

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Benchmarks comparing the new hooks system with the old interceptor system

// Helper to create a stream from a slice for testing
func fromSlice[T any](items []T) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], len(items))
		go func() {
			defer close(out)
			for _, item := range items {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(item):
				}
			}
		}()
		return out
	})
}

// Helper to generate benchmark data
func benchmarkData(n int) []int {
	data := make([]int, n)
	for i := range data {
		data[i] = i
	}
	return data
}

func BenchmarkMapperNoObservation(b *testing.B) {
	ctx := context.Background()
	mapper := core.Map(func(v int) (int, error) {
		return v * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(1000))
		result := mapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}

func BenchmarkMapperWithHooks(b *testing.B) {
	var count atomic.Int64
	ctx := core.WithHooks(context.Background(), core.Hooks[int]{
		OnValue: func(v int) {
			count.Add(1)
		},
	})

	mapper := core.Map(func(v int) (int, error) {
		return v * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(1000))
		result := mapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}

func BenchmarkMapperWithMultipleHooks(b *testing.B) {
	var count1, count2, count3 atomic.Int64
	ctx := context.Background()

	ctx = core.WithHooks(ctx, core.Hooks[int]{
		OnValue: func(v int) {
			count1.Add(1)
		},
	})
	ctx = core.WithHooks(ctx, core.Hooks[int]{
		OnValue: func(v int) {
			count2.Add(1)
		},
	})
	ctx = core.WithHooks(ctx, core.Hooks[int]{
		OnValue: func(v int) {
			count3.Add(1)
		},
	})

	mapper := core.Map(func(v int) (int, error) {
		return v * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(1000))
		result := mapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}

func BenchmarkFlatMapperWithHooks(b *testing.B) {
	var count atomic.Int64
	ctx := core.WithHooks(context.Background(), core.Hooks[int]{
		OnValue: func(v int) {
			count.Add(1)
		},
	})

	flatMapper := core.FlatMap(func(v int) ([]int, error) {
		return []int{v, v * 2}, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(500))
		result := flatMapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}

// Benchmark with all hook types
func BenchmarkMapperWithAllHookTypes(b *testing.B) {
	var starts, values, completes atomic.Int64
	ctx := core.WithHooks(context.Background(), core.Hooks[int]{
		OnStart: func() {
			starts.Add(1)
		},
		OnValue: func(v int) {
			values.Add(1)
		},
		OnComplete: func() {
			completes.Add(1)
		},
	})

	mapper := core.Map(func(v int) (int, error) {
		return v * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(1000))
		result := mapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}

// Benchmark with SafeHooks
func BenchmarkMapperWithSafeHooks(b *testing.B) {
	var count atomic.Int64
	hooks := core.NewSafeHooks(core.Hooks[int]{
		OnValue: func(v int) {
			count.Add(1)
		},
	}, nil)

	ctx := core.WithHooks(context.Background(), hooks.Hooks)
	mapper := core.Map(func(v int) (int, error) {
		return v * 2, nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream := fromSlice(benchmarkData(1000))
		result := mapper.Apply(ctx, stream)
		_, _ = core.Slice(ctx, result)
	}
}
