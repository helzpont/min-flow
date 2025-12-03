package benchmarks

import (
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
	"github.com/lguimbarda/min-flow/flow/observe"
)

// =============================================================================
// Interceptor Overhead Benchmarks
// These benchmarks measure the cost of the interceptor dispatch system.
// Run with: go test -bench=BenchmarkInterceptor -benchmem
// =============================================================================

// -----------------------------------------------------------------------------
// Baseline: Stream processing without interceptors
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_Baseline_NoInterceptors(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With Intercept[T] transmitter but no registered interceptors
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_InterceptNoRegistry(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(ctx, stream)
		mapped := core.Map(squareWithErr).Apply(ctx, intercepted)
		_, _ = core.Slice(ctx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With Intercept[T] and empty registry
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_EmptyRegistry(b *testing.B) {
	data := generateInts(MediumSize)
	testCtx, _ := core.WithRegistry(ctx)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		mapped := core.Map(squareWithErr).Apply(testCtx, intercepted)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 1 simple interceptor (CounterInterceptor - atomic increment)
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_1Counter(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, registry := core.WithRegistry(ctx)
		counter := observe.NewCounterInterceptor()
		_ = registry.Register(counter)

		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		mapped := core.Map(squareWithErr).Apply(testCtx, intercepted)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 3 interceptors (Counter, ErrorCounter, Callback)
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_3Interceptors(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, registry := core.WithRegistry(ctx)
		_ = registry.Register(observe.NewCounterInterceptor())
		_ = registry.Register(flowerrors.NewErrorCounterInterceptor(nil))
		_ = registry.Register(observe.NewCallbackInterceptor(
			observe.WithOnValue(func(any) {}),
		))

		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		mapped := core.Map(squareWithErr).Apply(testCtx, intercepted)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With MetricsInterceptor (mutex-locked, does time calculations)
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_Metrics(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, registry := core.WithRegistry(ctx)
		metrics := observe.NewMetricsInterceptor(nil)
		_ = registry.Register(metrics)

		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		mapped := core.Map(squareWithErr).Apply(testCtx, intercepted)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 5 interceptors (realistic production scenario)
// -----------------------------------------------------------------------------

func BenchmarkInterceptor_5Interceptors(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, registry := core.WithRegistry(ctx)
		_ = registry.Register(observe.NewMetricsInterceptor(nil))
		_ = registry.Register(observe.NewCounterInterceptor())
		_ = registry.Register(flowerrors.NewErrorCounterInterceptor(nil))
		_ = registry.Register(flowerrors.NewErrorCollectorInterceptor())
		_ = registry.Register(observe.NewCallbackInterceptor(
			observe.WithOnValue(func(any) {}),
		))

		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		mapped := core.Map(squareWithErr).Apply(testCtx, intercepted)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// =============================================================================
// Per-item overhead comparison
// Using a large dataset to measure per-item interceptor cost
// =============================================================================

func BenchmarkInterceptor_PerItem_Baseline(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		_, _ = core.Slice(ctx, stream)
	}
}

func BenchmarkInterceptor_PerItem_WithIntercept(b *testing.B) {
	data := generateInts(LargeSize)
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.Intercept[int]().Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

func BenchmarkInterceptor_PerItem_Buffered16(b *testing.B) {
	data := generateInts(LargeSize)
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](16).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

func BenchmarkInterceptor_PerItem_Buffered64(b *testing.B) {
	data := generateInts(LargeSize)
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](64).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

func BenchmarkInterceptor_PerItem_Buffered256(b *testing.B) {
	data := generateInts(LargeSize)
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](256).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

// =============================================================================
// Event matching overhead
// Measure the cost of event pattern matching
// =============================================================================

func BenchmarkInterceptor_EventMatching_Exact(b *testing.B) {
	event := core.ItemReceived
	pattern := "item:received"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkInterceptor_EventMatching_WildcardSuffix(b *testing.B) {
	event := core.StreamStart
	pattern := "stream:*"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkInterceptor_EventMatching_WildcardPrefix(b *testing.B) {
	event := core.StreamStart
	pattern := "*:start"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkInterceptor_EventMatching_All(b *testing.B) {
	event := core.ItemReceived
	pattern := "*"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

// =============================================================================
// Registry operations
// =============================================================================

func BenchmarkRegistry_WithRegistry(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = core.WithRegistry(ctx)
	}
}

func BenchmarkRegistry_Register(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, registry := core.WithRegistry(ctx)
		_ = registry.Register(observe.NewCounterInterceptor())
	}
}

func BenchmarkRegistry_GetInterceptors_1(b *testing.B) {
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	_ = testCtx
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = registry.Interceptors()
	}
}

func BenchmarkRegistry_GetInterceptors_5(b *testing.B) {
	testCtx, registry := core.WithRegistry(ctx)
	_ = registry.Register(observe.NewCounterInterceptor())
	_ = registry.Register(observe.NewMetricsInterceptor(nil))
	_ = registry.Register(flowerrors.NewErrorCounterInterceptor(nil))
	_ = registry.Register(flowerrors.NewErrorCollectorInterceptor())
	_ = registry.Register(observe.NewCallbackInterceptor())
	_ = testCtx
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = registry.Interceptors()
	}
}

func BenchmarkRegistry_GetRegistry(b *testing.B) {
	testCtx, _ := core.WithRegistry(ctx)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = core.GetRegistry(testCtx)
	}
}
