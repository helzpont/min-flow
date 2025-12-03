# min-flow Benchmarks

Comparative benchmarks of min-flow against popular Go stream processing libraries.

## Competitors

| Library                                             | Description                                   |
| --------------------------------------------------- | --------------------------------------------- |
| [destel/rill](https://github.com/destel/rill)       | Channel-based composable concurrency toolkit  |
| [samber/lo](https://github.com/samber/lo)           | Lodash-style Go utility library with generics |
| [ahmetb/go-linq](https://github.com/ahmetb/go-linq) | .NET LINQ-style query library for Go          |

## Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific benchmark category
go test -bench=BenchmarkMap -benchmem ./...
go test -bench=BenchmarkFilter -benchmem ./...
go test -bench=BenchmarkReduce -benchmem ./...
go test -bench=BenchmarkParallel -benchmem ./...

# Run with multiple iterations for stability
go test -bench=. -benchmem -count=5 ./...

# Output to file for analysis
go test -bench=. -benchmem -count=5 ./... | tee results.txt
```

## Benchmark Categories

### 1. Throughput Benchmarks

Compare raw throughput for Map, Filter, and Reduce operations.

### 2. Parallel Scaling Benchmarks

Test parallel performance at 1, 2, 4, 8, 16 workers.

### 3. Memory Allocation Benchmarks

Compare allocations per operation (via `-benchmem`).

### 4. Pipeline Benchmarks

Multi-stage pipelines combining Map, Filter, and Reduce.

## Notes

- All benchmarks use the same input data for fair comparison
- Each library is used idiomatically (not forced into another's patterns)
- Benchmarks focus on common operations available across all libraries
- min-flow's streaming model may show different characteristics than slice-based libraries
