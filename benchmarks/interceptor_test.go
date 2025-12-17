package benchmarks

import (
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/flowerrors"
	"github.com/lguimbarda/min-flow/flow/observe"
)

// =============================================================================
// Hooks Overhead Benchmarks
// These benchmarks measure the cost of the typed hooks system.
// Run with: go test -bench=BenchmarkHooks -benchmem
// =============================================================================

// -----------------------------------------------------------------------------
// Baseline: Stream processing without hooks
// -----------------------------------------------------------------------------

func BenchmarkHooks_Baseline_NoHooks(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 1 simple hook (Counter - atomic increment)
// -----------------------------------------------------------------------------

func BenchmarkHooks_1Counter(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)

		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 3 hooks (Counter, ErrorCounter, ValueHook)
// -----------------------------------------------------------------------------

func BenchmarkHooks_3Hooks(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		testCtx, _ = flowerrors.WithErrorCounter[int](testCtx, nil)
		testCtx = observe.WithValueHook(testCtx, func(int) {})

		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With full Hooks struct (all callbacks)
// -----------------------------------------------------------------------------

func BenchmarkHooks_FullHooksStruct(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx := core.WithHooks(ctx, core.Hooks[int]{
			OnStart:    func() {},
			OnValue:    func(int) {},
			OnError:    func(error) {},
			OnSentinel: func(error) {},
			OnComplete: func() {},
		})

		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// -----------------------------------------------------------------------------
// With 5 hooks (realistic production scenario)
// -----------------------------------------------------------------------------

func BenchmarkHooks_5Hooks(b *testing.B) {
	data := generateInts(MediumSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		testCtx, _ = observe.WithValueCounter[int](testCtx)
		testCtx, _ = flowerrors.WithErrorCounter[int](testCtx, nil)
		testCtx, _ = flowerrors.WithErrorCollector[int](testCtx)
		testCtx = observe.WithValueHook(testCtx, func(int) {})

		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// =============================================================================
// Per-item overhead comparison
// Using a large dataset to measure per-item hook cost
// =============================================================================

func BenchmarkHooks_PerItem_Baseline(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(ctx, stream)
		_, _ = core.Slice(ctx, mapped)
	}
}

func BenchmarkHooks_PerItem_WithCounter(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

func BenchmarkHooks_PerItem_WithLogging(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx := observe.WithLogging[int](ctx, func(string, ...any) {})
		stream := flow.FromSlice(data)
		mapped := core.Map(squareWithErr).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, mapped)
	}
}

// =============================================================================
// InterceptBuffered with hooks
// =============================================================================

func BenchmarkHooks_InterceptBuffered16(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](16).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

func BenchmarkHooks_InterceptBuffered64(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](64).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

func BenchmarkHooks_InterceptBuffered256(b *testing.B) {
	data := generateInts(LargeSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx, _ := observe.WithCounter[int](ctx)
		stream := flow.FromSlice(data)
		intercepted := core.InterceptBuffered[int](256).Apply(testCtx, stream)
		_, _ = core.Slice(testCtx, intercepted)
	}
}

// =============================================================================
// Hook registration overhead
// =============================================================================

func BenchmarkHooks_WithHooks_Single(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = core.WithHooks(ctx, core.Hooks[int]{
			OnValue: func(int) {},
		})
	}
}

func BenchmarkHooks_WithHooks_Full(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = core.WithHooks(ctx, core.Hooks[int]{
			OnStart:    func() {},
			OnValue:    func(int) {},
			OnError:    func(error) {},
			OnSentinel: func(error) {},
			OnComplete: func() {},
		})
	}
}

func BenchmarkHooks_WithCounter(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = observe.WithCounter[int](ctx)
	}
}

func BenchmarkHooks_Compose_3Hooks(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testCtx := core.WithHooks(ctx, core.Hooks[int]{OnValue: func(int) {}})
		testCtx = core.WithHooks(testCtx, core.Hooks[int]{OnError: func(error) {}})
		_ = core.WithHooks(testCtx, core.Hooks[int]{OnComplete: func() {}})
	}
}

// =============================================================================
// Event matching overhead (kept for reference, still used by Registry)
// =============================================================================

func BenchmarkEvent_Matching_Exact(b *testing.B) {
	event := core.ItemReceived
	pattern := "item:received"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkEvent_Matching_WildcardSuffix(b *testing.B) {
	event := core.StreamStart
	pattern := "stream:*"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkEvent_Matching_WildcardPrefix(b *testing.B) {
	event := core.StreamStart
	pattern := "*:start"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

func BenchmarkEvent_Matching_All(b *testing.B) {
	event := core.ItemReceived
	pattern := "*"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = event.Matches(pattern)
	}
}

// =============================================================================
// Registry operations (still available for other delegate types)
// =============================================================================

func BenchmarkRegistry_WithRegistry(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = core.WithRegistry(ctx)
	}
}

func BenchmarkRegistry_GetRegistry(b *testing.B) {
	testCtx, _ := core.WithRegistry(ctx)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = core.GetRegistry(testCtx)
	}
}
