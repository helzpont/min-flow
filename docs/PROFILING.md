# Profiling Min-Flow: A Practical Guide

This guide covers how to profile min-flow streams for both **development optimization** and **production monitoring**. We distinguish between two complementary approaches:

1. **pprof** — For absolute performance measurements and optimization work
2. **Interceptors** — For understanding stream behavior and relative comparisons

---

## When to Use Each Approach

| Goal                        | Tool         | Why                                      |
| --------------------------- | ------------ | ---------------------------------------- |
| Find CPU hotspots           | pprof        | Zero-overhead sampling, flame graphs     |
| Find memory allocations     | pprof heap   | Tracks actual allocator calls            |
| Compare two implementations | pprof        | Absolute numbers without observer effect |
| Understand item flow        | Interceptors | Counts values, errors, sentinels         |
| Monitor throughput patterns | Interceptors | Items/second, latency distribution       |
| Debug stream behavior       | Interceptors | See what events fire and when            |
| Production monitoring       | Interceptors | Integrates with metrics systems          |

**Key Principle**: Use pprof when you need absolute performance numbers. Use interceptors when you need to understand stream behavior or for production monitoring where a small overhead is acceptable.

---

## Part 1: Profiling with pprof

### Basic CPU Profile

Create a benchmark file and run with profiling:

```go
// benchmarks/profile_test.go
package benchmarks

import (
    "context"
    "testing"

    "github.com/lguimbarda/min-flow/flow"
    "github.com/lguimbarda/min-flow/flow/core"
    "github.com/lguimbarda/min-flow/flow/filter"
)

func BenchmarkPipelineProfile(b *testing.B) {
    b.ReportAllocs()
    ctx := context.Background()
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }

    double := core.Map(func(x int) (int, error) { return x * 2, nil })
    divisibleBy4 := filter.Where(func(x int) (bool, error) { return x%4 == 0, nil })
    addOne := core.Map(func(x int) (int, error) { return x + 1, nil })

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream := flow.FromSlice(data)
        stream = double.Apply(ctx, stream)
        stream = divisibleBy4.Apply(ctx, stream)
        stream = addOne.Apply(ctx, stream)

        _, _ = core.Slice(ctx, stream)
    }
}
```

Run with CPU profiling:

```bash
# Generate CPU profile
go test -bench=BenchmarkPipelineProfile -cpuprofile=cpu.prof ./benchmarks/

# Analyze interactively
go tool pprof cpu.prof

# Or generate flame graph (requires graphviz)
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```bash
# Generate memory profile
go test -bench=BenchmarkPipelineProfile -memprofile=mem.prof ./benchmarks/

# Analyze allocations
go tool pprof -alloc_space mem.prof

# Focus on specific package
(pprof) top 20 -cum
(pprof) list core.Map
```

### Useful pprof Commands

```bash
# In interactive mode:
(pprof) top 20          # Top 20 functions by CPU/memory
(pprof) top -cum        # Sort by cumulative time
(pprof) list funcName   # Show source code with annotations
(pprof) web             # Open flame graph in browser
(pprof) peek funcName   # Show callers and callees
```

### Comparing Profiles

Use `-base` to compare before/after optimization:

```bash
# Before optimization
go test -bench=. -cpuprofile=before.prof ./benchmarks/

# After optimization
go test -bench=. -cpuprofile=after.prof ./benchmarks/

# Compare
go tool pprof -base=before.prof after.prof
```

### Trace Profiling

For understanding concurrency and goroutine scheduling:

```bash
go test -bench=BenchmarkPipelineProfile -trace=trace.out ./benchmarks/
go tool trace trace.out
```

This shows:

- Goroutine creation/destruction
- Channel operations
- GC pauses
- Scheduler behavior

---

## Part 2: Profiling with Interceptors

Interceptors add overhead but provide semantic understanding of stream behavior. Use them for:

- Understanding what your stream is doing
- Relative comparisons (both runs have the same overhead)
- Production monitoring

### Setting Up Metrics Collection

```go
package main

import (
    "context"
    "fmt"

    "github.com/lguimbarda/min-flow/flow"
    "github.com/lguimbarda/min-flow/flow/core"
    "github.com/lguimbarda/min-flow/flow/observe"
)

func main() {
    ctx := context.Background()

    // Create metrics interceptor with callback
    var finalMetrics observe.StreamMetrics
    metricsInterceptor := observe.NewMetricsInterceptor(func(m observe.StreamMetrics) {
        finalMetrics = m
    })

    // Register interceptor
    registry := core.NewRegistry()
    registry.Register(metricsInterceptor)
    ctx = core.WithRegistry(ctx, registry)

    // Run stream
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }
    stream := flow.FromSlice(data)
    double := core.Map(func(x int) (int, error) { return x * 2, nil })
    stream = double.Apply(ctx, stream)

    _, _ = core.Slice(ctx, stream)

    // Report results
    fmt.Printf("Duration: %v\n", finalMetrics.Duration)
    fmt.Printf("Total Items: %d\n", finalMetrics.TotalItems)
    fmt.Printf("Values: %d, Errors: %d, Sentinels: %d\n",
        finalMetrics.ValueCount, finalMetrics.ErrorCount, finalMetrics.SentinelCount)
    fmt.Printf("Items/sec: %.2f\n", finalMetrics.ItemsPerSecond)
    fmt.Printf("Avg Latency: %v\n", finalMetrics.AverageLatency)
}
```

### Live Metrics for Long-Running Streams

```go
// Create live metrics for concurrent access
liveMetrics := observe.NewLiveMetrics()
interceptor := observe.NewLiveMetricsInterceptor(liveMetrics)

registry := core.NewRegistry()
registry.Register(interceptor)
ctx = core.WithRegistry(ctx, registry)

// Query metrics while stream is running (from another goroutine)
go func() {
    for {
        time.Sleep(time.Second)
        fmt.Printf("Items processed: %d, Rate: %.2f/s\n",
            liveMetrics.TotalItems(),
            liveMetrics.ItemsPerSecond())
    }
}()
```

### Custom Interceptors for Specific Measurements

```go
// Interceptor that measures time between specific events
type LatencyInterceptor struct {
    startTimes sync.Map  // streamID -> start time
    latencies  []time.Duration
    mu         sync.Mutex
}

func (l *LatencyInterceptor) Init() error  { return nil }
func (l *LatencyInterceptor) Close() error { return nil }

func (l *LatencyInterceptor) Events() []core.Event {
    return []core.Event{core.StreamStart, core.StreamEnd}
}

func (l *LatencyInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
    streamID := getStreamID(args)

    switch event {
    case core.StreamStart:
        l.startTimes.Store(streamID, time.Now())
    case core.StreamEnd:
        if start, ok := l.startTimes.Load(streamID); ok {
            latency := time.Since(start.(time.Time))
            l.mu.Lock()
            l.latencies = append(l.latencies, latency)
            l.mu.Unlock()
        }
    }
    return nil
}
```

---

## Part 3: Comparing Core vs Fast Package

The `flow/fast` package provides a baseline without min-flow features. Use it to measure feature overhead:

```go
// benchmarks/overhead_test.go
package benchmarks

import (
    "context"
    "testing"

    "github.com/lguimbarda/min-flow/flow"
    "github.com/lguimbarda/min-flow/flow/core"
    "github.com/lguimbarda/min-flow/flow/fast"
)

func BenchmarkCore(b *testing.B) {
    ctx := context.Background()
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }
    double := core.Map(func(x int) (int, error) { return x * 2, nil })

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream := flow.FromSlice(data)
        stream = double.Apply(ctx, stream)
        _, _ = core.Slice(ctx, stream)
    }
}

func BenchmarkFast(b *testing.B) {
    ctx := context.Background()
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }
    double := fast.Map(func(x int) int { return x * 2 })

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream := fast.FromSlice(data)
        stream = double.Apply(ctx, stream)
        _ = fast.Slice(ctx, stream)
    }
}
```

### Isolating Specific Overhead

```go
// Measure just Result wrapping overhead
func BenchmarkResultOverhead(b *testing.B) {
    ctx := context.Background()
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream := flow.FromSlice(data)
        _, _ = core.Slice(ctx, stream)
    }
}

func BenchmarkNoResultOverhead(b *testing.B) {
    ctx := context.Background()
    data := make([]int, 10000)
    for i := range data {
        data[i] = i
    }

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        stream := fast.FromSlice(data)
        _ = fast.Slice(ctx, stream)
    }
}
```

---

## Part 4: Optimization Investigation Workflow

When investigating performance issues:

### Step 1: Establish Baseline

```bash
# Run benchmarks with memory info
go test -bench=. -benchmem ./benchmarks/ | tee baseline.txt
```

### Step 2: Generate Profile

```bash
# CPU profile for the specific benchmark
go test -bench=BenchmarkProblem -cpuprofile=cpu.prof -memprofile=mem.prof ./benchmarks/
```

### Step 3: Identify Hotspots

```bash
go tool pprof -http=:8080 cpu.prof
# Look at flame graph, find wide bars
```

### Step 4: Focus Investigation

```bash
# In pprof interactive mode
(pprof) list core.Map
(pprof) peek core.Result
```

### Step 5: Make Change and Compare

```bash
# After modification
go test -bench=. -benchmem ./benchmarks/ | tee after.txt

# Compare
diff baseline.txt after.txt

# Or use benchstat for statistical comparison
go install golang.org/x/perf/cmd/benchstat@latest
benchstat baseline.txt after.txt
```

---

## Part 5: Common Performance Patterns

### Pattern 1: Allocation Hot Spots

Look for in pprof:

- `runtime.newobject` — heap allocations
- `runtime.makeslice` — slice allocations
- `runtime.makechan` — channel creation

Solutions:

- Pool objects with `sync.Pool`
- Pre-allocate slices with capacity
- Reduce channel buffer churn

### Pattern 2: Lock Contention

Look for in trace:

- Goroutines blocked on mutex
- High "Sync block" time

Solutions:

- Use atomic operations
- Reduce critical section size
- Consider lock-free algorithms

### Pattern 3: GC Pressure

Look for:

- High `runtime.gcBgMarkWorker` in CPU profile
- Frequent GC cycles in trace

Solutions:

- Reduce allocations
- Use value types instead of pointers
- Pool long-lived objects

### Pattern 4: Channel Overhead

Look for:

- `runtime.chansend` / `runtime.chanrecv`
- Goroutines blocked on channel ops

Solutions:

- Use fusion to eliminate intermediate channels
- Batch operations
- Consider larger buffer sizes

---

## Quick Reference

### Run All Benchmarks with Memory Info

```bash
go test -bench=. -benchmem ./benchmarks/
```

### CPU Profile Specific Benchmark

```bash
go test -bench=BenchmarkName -cpuprofile=cpu.prof ./benchmarks/
go tool pprof -http=:8080 cpu.prof
```

### Memory Profile

```bash
go test -bench=BenchmarkName -memprofile=mem.prof ./benchmarks/
go tool pprof -alloc_space mem.prof
```

### Execution Trace

```bash
go test -bench=BenchmarkName -trace=trace.out ./benchmarks/
go tool trace trace.out
```

### Compare Before/After

```bash
go install golang.org/x/perf/cmd/benchstat@latest
go test -bench=. -benchmem -count=10 ./benchmarks/ > before.txt
# make changes
go test -bench=. -benchmem -count=10 ./benchmarks/ > after.txt
benchstat before.txt after.txt
```

---

## Important Caveats

1. **Interceptor Overhead**: Interceptors add measurable overhead. When comparing absolute performance, use pprof or the fast package as baseline. Interceptors are best for relative comparisons where both measurements include the same overhead.

2. **Microbenchmarks vs Real Workloads**: Microbenchmarks may not reflect real-world performance. Always validate with realistic data sizes and patterns.

3. **Warmup**: Go's runtime may behave differently during warmup. Use `b.ResetTimer()` after setup and consider using `-count=N` for statistical validity.

4. **GC Variability**: Run benchmarks multiple times and use `benchstat` for statistical analysis. Single runs can be misleading.

5. **Hardware Variability**: Disable turbo boost, close other applications, and use isolated cores for reproducible results.
