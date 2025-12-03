package core

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestTransformConfig(t *testing.T) {
	tests := []struct {
		name           string
		opts           []TransformOption
		wantBufferSize int
	}{
		{
			name:           "default config",
			opts:           nil,
			wantBufferSize: DefaultBufferSize,
		},
		{
			name:           "custom buffer size",
			opts:           []TransformOption{WithBufferSize(128)},
			wantBufferSize: 128,
		},
		{
			name:           "zero buffer size (unbuffered)",
			opts:           []TransformOption{WithBufferSize(0)},
			wantBufferSize: 0,
		},
		{
			name:           "multiple options last wins",
			opts:           []TransformOption{WithBufferSize(32), WithBufferSize(256)},
			wantBufferSize: 256,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := applyOptions(context.Background(), tt.opts...)
			if cfg.BufferSize != tt.wantBufferSize {
				t.Errorf("BufferSize = %d, want %d", cfg.BufferSize, tt.wantBufferSize)
			}
		})
	}
}

func TestTransformConfig_Delegate(t *testing.T) {
	cfg := &TransformConfig{BufferSize: 64}

	// Test Init
	if err := cfg.Init(); err != nil {
		t.Errorf("Init() error = %v, want nil", err)
	}

	// Test Close
	if err := cfg.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestTransformConfig_Validate(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
		wantErr    bool
	}{
		{
			name:       "valid zero",
			bufferSize: 0,
			wantErr:    false,
		},
		{
			name:       "valid positive",
			bufferSize: 64,
			wantErr:    false,
		},
		{
			name:       "invalid negative",
			bufferSize: -1,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &TransformConfig{BufferSize: tt.bufferSize}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTransformConfig_FromContext(t *testing.T) {
	tests := []struct {
		name           string
		ctxBufferSize  int
		opts           []TransformOption
		wantBufferSize int
	}{
		{
			name:           "context config only",
			ctxBufferSize:  128,
			opts:           nil,
			wantBufferSize: 128,
		},
		{
			name:           "option overrides context",
			ctxBufferSize:  128,
			opts:           []TransformOption{WithBufferSize(256)},
			wantBufferSize: 256,
		},
		{
			name:           "no context config uses default",
			ctxBufferSize:  -1, // sentinel for no context config
			opts:           nil,
			wantBufferSize: DefaultBufferSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctxBufferSize >= 0 {
				var registry *Registry
				ctx, registry = WithRegistry(ctx)
				cfg := &TransformConfig{BufferSize: tt.ctxBufferSize}
				_ = registry.Register(cfg)
			}

			result := applyOptions(ctx, tt.opts...)
			if result.BufferSize != tt.wantBufferSize {
				t.Errorf("BufferSize = %d, want %d", result.BufferSize, tt.wantBufferSize)
			}
		})
	}
}

func TestMapper_ApplyWith(t *testing.T) {
	ctx := context.Background()
	double := Map(func(x int) (int, error) { return x * 2, nil })

	tests := []struct {
		name       string
		input      []int
		opts       []TransformOption
		wantValues []int
	}{
		{
			name:       "default buffer",
			input:      []int{1, 2, 3},
			opts:       nil,
			wantValues: []int{2, 4, 6},
		},
		{
			name:       "custom buffer size",
			input:      []int{1, 2, 3, 4, 5},
			opts:       []TransformOption{WithBufferSize(2)},
			wantValues: []int{2, 4, 6, 8, 10},
		},
		{
			name:       "unbuffered",
			input:      []int{1, 2},
			opts:       []TransformOption{WithBufferSize(0)},
			wantValues: []int{2, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := Emit(func(ctx context.Context) <-chan Result[int] {
				ch := make(chan Result[int], len(tt.input))
				for _, v := range tt.input {
					ch <- Ok(v)
				}
				close(ch)
				return ch
			})

			result := double.ApplyWith(ctx, stream, tt.opts...)
			collected := result.Collect(ctx)

			if len(collected) != len(tt.wantValues) {
				t.Fatalf("got %d results, want %d", len(collected), len(tt.wantValues))
			}

			for i, res := range collected {
				if res.IsError() {
					t.Errorf("result[%d] is error: %v", i, res.Error())
					continue
				}
				if res.Value() != tt.wantValues[i] {
					t.Errorf("result[%d] = %d, want %d", i, res.Value(), tt.wantValues[i])
				}
			}
		})
	}
}

func TestFuse(t *testing.T) {
	ctx := context.Background()

	// Define simple mappers for testing
	double := Map(func(x int) (int, error) { return x * 2, nil })
	addTen := Map(func(x int) (int, error) { return x + 10, nil })
	toString := Map(func(x int) (string, error) { return strconv.Itoa(x), nil })

	t.Run("fuse two same-type mappers", func(t *testing.T) {
		// Fuse: double then addTen = (x * 2) + 10
		fused := Fuse(double, addTen)

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 3)
			ch <- Ok(1)
			ch <- Ok(5)
			ch <- Ok(10)
			close(ch)
			return ch
		})

		collected := fused.Apply(ctx, stream).Collect(ctx)

		want := []int{12, 20, 30} // (1*2)+10=12, (5*2)+10=20, (10*2)+10=30
		if len(collected) != len(want) {
			t.Fatalf("got %d results, want %d", len(collected), len(want))
		}
		for i, res := range collected {
			if res.IsError() {
				t.Errorf("result[%d] is error: %v", i, res.Error())
				continue
			}
			if res.Value() != want[i] {
				t.Errorf("result[%d] = %d, want %d", i, res.Value(), want[i])
			}
		}
	})

	t.Run("fuse mappers with different types", func(t *testing.T) {
		// Fuse: double then toString = strconv.Itoa(x * 2)
		fused := Fuse(double, toString)

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 2)
			ch <- Ok(7)
			ch <- Ok(21)
			close(ch)
			return ch
		})

		collected := fused.Apply(ctx, stream).Collect(ctx)

		want := []string{"14", "42"}
		if len(collected) != len(want) {
			t.Fatalf("got %d results, want %d", len(collected), len(want))
		}
		for i, res := range collected {
			if res.IsError() {
				t.Errorf("result[%d] is error: %v", i, res.Error())
				continue
			}
			if res.Value() != want[i] {
				t.Errorf("result[%d] = %q, want %q", i, res.Value(), want[i])
			}
		}
	})

	t.Run("fuse propagates errors from first mapper", func(t *testing.T) {
		errMapper := Map(func(x int) (int, error) {
			if x < 0 {
				return 0, errors.New("negative input")
			}
			return x, nil
		})
		fused := Fuse(errMapper, double)

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 2)
			ch <- Ok(-5)
			ch <- Ok(3)
			close(ch)
			return ch
		})

		collected := fused.Apply(ctx, stream).Collect(ctx)

		if len(collected) != 2 {
			t.Fatalf("got %d results, want 2", len(collected))
		}
		if !collected[0].IsError() {
			t.Errorf("result[0] should be error, got value %v", collected[0].Value())
		}
		if collected[1].IsError() {
			t.Errorf("result[1] should be value, got error %v", collected[1].Error())
		}
		if collected[1].Value() != 6 {
			t.Errorf("result[1] = %d, want 6", collected[1].Value())
		}
	})

	t.Run("fuse propagates errors from second mapper", func(t *testing.T) {
		errMapper := Map(func(x int) (int, error) {
			if x > 10 {
				return 0, errors.New("too large")
			}
			return x, nil
		})
		fused := Fuse(double, errMapper) // double first, then error check

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 2)
			ch <- Ok(3)  // 3*2=6, passes
			ch <- Ok(10) // 10*2=20, fails
			close(ch)
			return ch
		})

		collected := fused.Apply(ctx, stream).Collect(ctx)

		if len(collected) != 2 {
			t.Fatalf("got %d results, want 2", len(collected))
		}
		if collected[0].IsError() {
			t.Errorf("result[0] should be value, got error %v", collected[0].Error())
		}
		if collected[0].Value() != 6 {
			t.Errorf("result[0] = %d, want 6", collected[0].Value())
		}
		if !collected[1].IsError() {
			t.Errorf("result[1] should be error, got value %v", collected[1].Value())
		}
	})

	t.Run("fuse chain of three mappers", func(t *testing.T) {
		// Fuse three: double -> addTen -> toString
		fused := Fuse(Fuse(double, addTen), toString)

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 1)
			ch <- Ok(5)
			close(ch)
			return ch
		})

		collected := fused.Apply(ctx, stream).Collect(ctx)

		if len(collected) != 1 {
			t.Fatalf("got %d results, want 1", len(collected))
		}
		// (5*2)+10 = 20 -> "20"
		if collected[0].Value() != "20" {
			t.Errorf("result = %q, want %q", collected[0].Value(), "20")
		}
	})
}
func TestFlatMapper_ApplyWith(t *testing.T) {
	ctx := context.Background()
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })

	tests := []struct {
		name       string
		input      []int
		opts       []TransformOption
		wantValues []int
	}{
		{
			name:       "default buffer",
			input:      []int{1, 2},
			opts:       nil,
			wantValues: []int{1, 1, 2, 2},
		},
		{
			name:       "custom buffer size",
			input:      []int{1, 2, 3},
			opts:       []TransformOption{WithBufferSize(4)},
			wantValues: []int{1, 1, 2, 2, 3, 3},
		},
		{
			name:       "unbuffered",
			input:      []int{5},
			opts:       []TransformOption{WithBufferSize(0)},
			wantValues: []int{5, 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := Emit(func(ctx context.Context) <-chan Result[int] {
				ch := make(chan Result[int], len(tt.input))
				for _, v := range tt.input {
					ch <- Ok(v)
				}
				close(ch)
				return ch
			})

			result := duplicate.ApplyWith(ctx, stream, tt.opts...)
			collected := result.Collect(ctx)

			if len(collected) != len(tt.wantValues) {
				t.Fatalf("got %d results, want %d", len(collected), len(tt.wantValues))
			}

			for i, res := range collected {
				if res.IsError() {
					t.Errorf("result[%d] is error: %v", i, res.Error())
					continue
				}
				if res.Value() != tt.wantValues[i] {
					t.Errorf("result[%d] = %d, want %d", i, res.Value(), tt.wantValues[i])
				}
			}
		})
	}
}

// Helper to create a stream from a slice
func streamFromSlice[T any](data []T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		ch := make(chan Result[T], len(data))
		for _, v := range data {
			ch <- Ok(v)
		}
		close(ch)
		return ch
	})
}

// Helper to collect values from results
func collectValues[T any](results []Result[T]) []T {
	values := make([]T, 0, len(results))
	for _, r := range results {
		if r.IsValue() {
			values = append(values, r.Value())
		}
	}
	return values
}

func TestFuseMapFlat(t *testing.T) {
	ctx := context.Background()
	double := Map(func(x int) (int, error) { return x * 2, nil })
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })

	// Fuse: double then duplicate (using ToFlatMapper conversion)
	fused := FuseFlat(double.ToFlatMapper(), duplicate)

	stream := streamFromSlice([]int{1, 2, 3})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// (1*2, 1*2), (2*2, 2*2), (3*2, 3*2) = 2,2,4,4,6,6
	want := []int{2, 2, 4, 4, 6, 6}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
	for i, v := range values {
		if v != want[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, want[i])
		}
	}
}

func TestFuseFlatMap(t *testing.T) {
	ctx := context.Background()
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })
	double := Map(func(x int) (int, error) { return x * 2, nil })

	// Fuse: duplicate then double (using ToFlatMapper conversion)
	fused := FuseFlat(duplicate, double.ToFlatMapper())

	stream := streamFromSlice([]int{1, 2})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// (1,1) -> (2,2), (2,2) -> (4,4) = 2,2,4,4
	want := []int{2, 2, 4, 4}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
	for i, v := range values {
		if v != want[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, want[i])
		}
	}
}

func TestFuseFlat(t *testing.T) {
	ctx := context.Background()
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })
	triple := FlatMap(func(x int) ([]int, error) { return []int{x, x, x}, nil })

	// Fuse: duplicate then triple = 6 outputs per input
	fused := FuseFlat(duplicate, triple)

	stream := streamFromSlice([]int{1})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// 1 -> (1,1) -> (1,1,1,1,1,1)
	want := []int{1, 1, 1, 1, 1, 1}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
}

func TestFuseMapFilter(t *testing.T) {
	ctx := context.Background()
	double := Map(func(x int) (int, error) { return x * 2, nil })
	isEven := Predicate[int](func(x int) bool { return x%2 == 0 })

	// Fuse: double then filter even (using ToFlatMapper conversions)
	fused := FuseFlat(double.ToFlatMapper(), isEven.ToFlatMapper())

	stream := streamFromSlice([]int{1, 2, 3})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	want := []int{2, 4, 6}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}

	// Now test with a filter that actually filters
	greaterThan3 := Predicate[int](func(x int) bool { return x > 3 })
	fused2 := FuseFlat(double.ToFlatMapper(), greaterThan3.ToFlatMapper())
	stream2 := streamFromSlice([]int{1, 2, 3})
	collected2 := fused2.Apply(ctx, stream2).Collect(ctx)
	values2 := collectValues(collected2)

	// 1*2=2 (filtered), 2*2=4 (pass), 3*2=6 (pass)
	want2 := []int{4, 6}
	if len(values2) != len(want2) {
		t.Fatalf("got %d values, want %d", len(values2), len(want2))
	}
}

func TestFuseFilterMap(t *testing.T) {
	ctx := context.Background()
	isPositive := Predicate[int](func(x int) bool { return x > 0 })
	double := Map(func(x int) (int, error) { return x * 2, nil })

	// Fuse: filter positive then double (using ToFlatMapper conversions)
	fused := FuseFlat(isPositive.ToFlatMapper(), double.ToFlatMapper())

	stream := streamFromSlice([]int{-1, 0, 1, 2})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// -1 (filtered), 0 (filtered), 1*2=2, 2*2=4
	want := []int{2, 4}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
	for i, v := range values {
		if v != want[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, want[i])
		}
	}
}

func TestFuseFlatFilter(t *testing.T) {
	ctx := context.Background()
	expand := FlatMap(func(x int) ([]int, error) { return []int{x - 1, x, x + 1}, nil })
	isPositive := Predicate[int](func(x int) bool { return x > 0 })

	// Fuse: expand to (x-1, x, x+1) then filter positive
	fused := FuseFlat(expand, isPositive.ToFlatMapper())

	stream := streamFromSlice([]int{1})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// 1 -> (0, 1, 2) -> filter -> (1, 2)
	want := []int{1, 2}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
}

func TestFuseFilterFlat(t *testing.T) {
	ctx := context.Background()
	isPositive := Predicate[int](func(x int) bool { return x > 0 })
	duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })

	// Fuse: filter positive then duplicate
	fused := FuseFlat(isPositive.ToFlatMapper(), duplicate)

	stream := streamFromSlice([]int{-1, 1, 2})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// -1 (filtered), 1 -> (1,1), 2 -> (2,2)
	want := []int{1, 1, 2, 2}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
}

func TestFuseFilters(t *testing.T) {
	ctx := context.Background()
	isPositive := Predicate[int](func(x int) bool { return x > 0 })
	isEven := Predicate[int](func(x int) bool { return x%2 == 0 })

	// Fuse: filter positive AND even (using ToFlatMapper)
	fused := FuseFlat(isPositive.ToFlatMapper(), isEven.ToFlatMapper())

	stream := streamFromSlice([]int{-2, -1, 0, 1, 2, 3, 4})
	collected := fused.Apply(ctx, stream).Collect(ctx)
	values := collectValues(collected)

	// Only positive AND even: 2, 4
	want := []int{2, 4}
	if len(values) != len(want) {
		t.Fatalf("got %d values, want %d", len(values), len(want))
	}
	for i, v := range values {
		if v != want[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, want[i])
		}
	}
}

func TestFuseErrorPropagation(t *testing.T) {
	ctx := context.Background()

	t.Run("FuseFlat with mapper propagates error", func(t *testing.T) {
		errMapper := Map(func(x int) (int, error) {
			if x < 0 {
				return 0, errors.New("negative")
			}
			return x, nil
		})
		duplicate := FlatMap(func(x int) ([]int, error) { return []int{x, x}, nil })
		fused := FuseFlat(errMapper.ToFlatMapper(), duplicate)

		stream := streamFromSlice([]int{-1, 1})
		collected := fused.Apply(ctx, stream).Collect(ctx)

		if len(collected) != 3 { // 1 error + 2 values
			t.Fatalf("got %d results, want 3", len(collected))
		}
		if !collected[0].IsError() {
			t.Error("first result should be error")
		}
	})

	t.Run("FuseFlat propagates flatmapper error", func(t *testing.T) {
		errFlat := FlatMap(func(x int) ([]int, error) {
			if x < 0 {
				return nil, errors.New("negative")
			}
			return []int{x}, nil
		})
		double := Map(func(x int) (int, error) { return x * 2, nil })
		fused := FuseFlat(errFlat, double.ToFlatMapper())

		stream := streamFromSlice([]int{-1, 1})
		collected := fused.Apply(ctx, stream).Collect(ctx)

		if !collected[0].IsError() {
			t.Error("first result should be error")
		}
		if collected[1].Value() != 2 {
			t.Errorf("second value = %d, want 2", collected[1].Value())
		}
	})
}

func TestToFlatMapper(t *testing.T) {
	ctx := context.Background()

	t.Run("Mapper.ToFlatMapper produces single output", func(t *testing.T) {
		double := Map(func(x int) (int, error) { return x * 2, nil })
		flat := double.ToFlatMapper()

		stream := streamFromSlice([]int{1, 2, 3})
		collected := flat.Apply(ctx, stream).Collect(ctx)
		values := collectValues(collected)

		want := []int{2, 4, 6}
		if len(values) != len(want) {
			t.Fatalf("got %d values, want %d", len(values), len(want))
		}
		for i, v := range values {
			if v != want[i] {
				t.Errorf("values[%d] = %d, want %d", i, v, want[i])
			}
		}
	})

	t.Run("Predicate.ToFlatMapper filters correctly", func(t *testing.T) {
		isPositive := Predicate[int](func(x int) bool { return x > 0 })
		flat := isPositive.ToFlatMapper()

		stream := streamFromSlice([]int{-1, 0, 1, 2})
		collected := flat.Apply(ctx, stream).Collect(ctx)
		values := collectValues(collected)

		want := []int{1, 2}
		if len(values) != len(want) {
			t.Fatalf("got %d values, want %d", len(values), len(want))
		}
		for i, v := range values {
			if v != want[i] {
				t.Errorf("values[%d] = %d, want %d", i, v, want[i])
			}
		}
	})

	t.Run("Predicate.ToFlatMapper passes through errors", func(t *testing.T) {
		isPositive := Predicate[int](func(x int) bool { return x > 0 })
		flat := isPositive.ToFlatMapper()

		stream := Emit(func(ctx context.Context) <-chan Result[int] {
			ch := make(chan Result[int], 2)
			ch <- Err[int](errors.New("test error"))
			ch <- Ok(5)
			close(ch)
			return ch
		})

		collected := flat.Apply(ctx, stream).Collect(ctx)

		if len(collected) != 2 {
			t.Fatalf("got %d results, want 2", len(collected))
		}
		if !collected[0].IsError() {
			t.Error("first result should be error")
		}
		if collected[1].Value() != 5 {
			t.Errorf("second value = %d, want 5", collected[1].Value())
		}
	})
}
