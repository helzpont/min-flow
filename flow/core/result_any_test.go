package core

import (
	"testing"
)

// =============================================================================
// AnyResult vs Result[T] Benchmarks
// =============================================================================

// BenchmarkResultCreation compares allocation overhead of Result[T] vs AnyResult
func BenchmarkResultCreation(b *testing.B) {
	b.Run("Result[int]", func(b *testing.B) {
		b.ReportAllocs()
		var sink Result[int]
		for i := 0; i < b.N; i++ {
			sink = Ok(i)
		}
		_ = sink
	})

	b.Run("AnyResult_int", func(b *testing.B) {
		b.ReportAllocs()
		var sink AnyResult
		for i := 0; i < b.N; i++ {
			sink = AnyOk(i)
		}
		_ = sink
	})

	b.Run("Result[string]", func(b *testing.B) {
		b.ReportAllocs()
		var sink Result[string]
		s := "hello"
		for i := 0; i < b.N; i++ {
			sink = Ok(s)
		}
		_ = sink
	})

	b.Run("AnyResult_string", func(b *testing.B) {
		b.ReportAllocs()
		var sink AnyResult
		s := "hello"
		for i := 0; i < b.N; i++ {
			sink = AnyOk(s)
		}
		_ = sink
	})

	// Larger struct to see if any-based helps more with larger types
	type LargeStruct struct {
		A, B, C, D, E int64
		Name          string
		Data          []byte
	}

	b.Run("Result[LargeStruct]", func(b *testing.B) {
		b.ReportAllocs()
		var sink Result[LargeStruct]
		ls := LargeStruct{A: 1, B: 2, C: 3, D: 4, E: 5, Name: "test", Data: []byte("data")}
		for i := 0; i < b.N; i++ {
			sink = Ok(ls)
		}
		_ = sink
	})

	b.Run("AnyResult_LargeStruct", func(b *testing.B) {
		b.ReportAllocs()
		var sink AnyResult
		ls := LargeStruct{A: 1, B: 2, C: 3, D: 4, E: 5, Name: "test", Data: []byte("data")}
		for i := 0; i < b.N; i++ {
			sink = AnyOk(ls)
		}
		_ = sink
	})
}

// BenchmarkResultValueAccess compares the cost of retrieving values
func BenchmarkResultValueAccess(b *testing.B) {
	b.Run("Result[int].Value", func(b *testing.B) {
		b.ReportAllocs()
		r := Ok(42)
		var sink int
		for i := 0; i < b.N; i++ {
			sink = r.Value()
		}
		_ = sink
	})

	b.Run("AnyResult.ValueAs[int]", func(b *testing.B) {
		b.ReportAllocs()
		r := AnyOk(42)
		var sink int
		for i := 0; i < b.N; i++ {
			sink = ValueAs[int](r)
		}
		_ = sink
	})

	b.Run("AnyResult.Value+assertion", func(b *testing.B) {
		b.ReportAllocs()
		r := AnyOk(42)
		var sink int
		for i := 0; i < b.N; i++ {
			sink = r.Value().(int)
		}
		_ = sink
	})
}

// BenchmarkResultStateCheck compares the cost of state checks
func BenchmarkResultStateCheck(b *testing.B) {
	b.Run("Result[int].IsValue", func(b *testing.B) {
		b.ReportAllocs()
		r := Ok(42)
		var sink bool
		for i := 0; i < b.N; i++ {
			sink = r.IsValue()
		}
		_ = sink
	})

	b.Run("AnyResult.IsValue", func(b *testing.B) {
		b.ReportAllocs()
		r := AnyOk(42)
		var sink bool
		for i := 0; i < b.N; i++ {
			sink = r.IsValue()
		}
		_ = sink
	})
}

// BenchmarkResultConversion compares the cost of conversions
func BenchmarkResultConversion(b *testing.B) {
	b.Run("FromResult", func(b *testing.B) {
		b.ReportAllocs()
		r := Ok(42)
		var sink AnyResult
		for i := 0; i < b.N; i++ {
			sink = FromResult(r)
		}
		_ = sink
	})

	b.Run("ToResult", func(b *testing.B) {
		b.ReportAllocs()
		r := AnyOk(42)
		var sink Result[int]
		for i := 0; i < b.N; i++ {
			sink = ToResult[int](r)
		}
		_ = sink
	})
}

// BenchmarkChannelThroughput measures channel ops with both result types
func BenchmarkChannelThroughput(b *testing.B) {
	const bufSize = 64
	const itemCount = 10000

	b.Run("chan_Result[int]", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := make(chan Result[int], bufSize)
			go func() {
				for j := 0; j < itemCount; j++ {
					ch <- Ok(j)
				}
				close(ch)
			}()
			for r := range ch {
				_ = r.Value()
			}
		}
	})

	b.Run("chan_AnyResult", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := make(chan AnyResult, bufSize)
			go func() {
				for j := 0; j < itemCount; j++ {
					ch <- AnyOk(j)
				}
				close(ch)
			}()
			for r := range ch {
				_ = ValueAs[int](r)
			}
		}
	})
}

// BenchmarkTransformationPipeline simulates a real transformation pipeline
func BenchmarkTransformationPipeline(b *testing.B) {
	const itemCount = 10000

	// Simulate: input -> double -> add 1 -> filter even -> output
	b.Run("Result[int]_pipeline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results := make([]Result[int], 0, itemCount/2)
			for j := 0; j < itemCount; j++ {
				r := Ok(j)
				// Transform 1: double
				v := r.Value() * 2
				r = Ok(v)
				// Transform 2: add 1
				v = r.Value() + 1
				r = Ok(v)
				// Filter: keep even
				if r.Value()%2 == 0 {
					results = append(results, r)
				}
			}
			_ = results
		}
	})

	b.Run("AnyResult_pipeline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results := make([]AnyResult, 0, itemCount/2)
			for j := 0; j < itemCount; j++ {
				r := AnyOk(j)
				// Transform 1: double
				v := ValueAs[int](r) * 2
				r = AnyOk(v)
				// Transform 2: add 1
				v = ValueAs[int](r) + 1
				r = AnyOk(v)
				// Filter: keep even
				if ValueAs[int](r)%2 == 0 {
					results = append(results, r)
				}
			}
			_ = results
		}
	})

	b.Run("direct_int_pipeline", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results := make([]int, 0, itemCount/2)
			for j := 0; j < itemCount; j++ {
				v := j
				// Transform 1: double
				v = v * 2
				// Transform 2: add 1
				v = v + 1
				// Filter: keep even
				if v%2 == 0 {
					results = append(results, v)
				}
			}
			_ = results
		}
	})
}

// BenchmarkSliceCollection measures collecting results into a slice
func BenchmarkSliceCollection(b *testing.B) {
	const itemCount = 10000

	b.Run("Result[int]_collect", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results := make([]Result[int], 0, itemCount)
			for j := 0; j < itemCount; j++ {
				results = append(results, Ok(j))
			}
			_ = results
		}
	})

	b.Run("AnyResult_collect", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			results := make([]AnyResult, 0, itemCount)
			for j := 0; j < itemCount; j++ {
				results = append(results, AnyOk(j))
			}
			_ = results
		}
	})
}
