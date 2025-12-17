package core

import (
	"context"
	"errors"
	"testing"
)

func TestEmit(t *testing.T) {
	emitter := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 2)
		out <- Ok(1)
		out <- Ok(2)
		close(out)
		return out
	})

	ctx := context.Background()
	ch := emitter.Emit(ctx)

	var values []int
	for res := range ch {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	if len(values) != 2 || values[0] != 1 || values[1] != 2 {
		t.Errorf("Emit() = %v, want [1, 2]", values)
	}
}

func TestEmitter_Collect(t *testing.T) {
	emitter := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 3)
		out <- Ok(1)
		out <- Err[int](errors.New("test"))
		out <- Ok(3)
		close(out)
		return out
	})

	ctx := context.Background()
	results := emitter.Collect(ctx)

	if len(results) != 3 {
		t.Errorf("Collect() got %d results, want 3", len(results))
	}

	if !results[0].IsValue() || results[0].Value() != 1 {
		t.Errorf("results[0] = %v, want Ok(1)", results[0])
	}
	if !results[1].IsError() {
		t.Errorf("results[1] should be error")
	}
	if !results[2].IsValue() || results[2].Value() != 3 {
		t.Errorf("results[2] = %v, want Ok(3)", results[2])
	}
}

func TestEmitter_All(t *testing.T) {
	emitter := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 3)
		out <- Ok(1)
		out <- Ok(2)
		out <- Ok(3)
		close(out)
		return out
	})

	ctx := context.Background()
	var values []int
	for res := range emitter.All(ctx) {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	if len(values) != 3 {
		t.Errorf("All() yielded %d values, want 3", len(values))
	}
}

func TestEmitter_All_EarlyBreak(t *testing.T) {
	emitter := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 10)
		for i := 0; i < 10; i++ {
			out <- Ok(i)
		}
		close(out)
		return out
	})

	ctx := context.Background()
	var count int
	for res := range emitter.All(ctx) {
		if res.IsValue() {
			count++
			if count >= 3 {
				break
			}
		}
	}

	if count != 3 {
		t.Errorf("All() with early break yielded %d values, want 3", count)
	}
}

func TestTransmit(t *testing.T) {
	transmitter := Transmit(func(ctx context.Context, in <-chan Result[int]) <-chan Result[string] {
		out := make(chan Result[string])
		go func() {
			defer close(out)
			for res := range in {
				if res.IsValue() {
					out <- Ok("v")
				} else {
					out <- Err[string](res.Error())
				}
			}
		}()
		return out
	})

	input := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 2)
		out <- Ok(1)
		out <- Ok(2)
		close(out)
		return out
	})

	ctx := context.Background()
	result := transmitter.Apply(input)

	var values []string
	for res := range result.Emit(ctx) {
		if res.IsValue() {
			values = append(values, res.Value())
		}
	}

	if len(values) != 2 || values[0] != "v" || values[1] != "v" {
		t.Errorf("Transmit.Apply() = %v, want [v, v]", values)
	}
}

func TestTransmitter_Apply_ErrorPropagation(t *testing.T) {
	transmitter := Transmit(func(ctx context.Context, in <-chan Result[int]) <-chan Result[int] {
		out := make(chan Result[int])
		go func() {
			defer close(out)
			for res := range in {
				out <- res
			}
		}()
		return out
	})

	input := Emit(func(ctx context.Context) <-chan Result[int] {
		out := make(chan Result[int], 2)
		out <- Ok(1)
		out <- Err[int](errors.New("test error"))
		close(out)
		return out
	})

	ctx := context.Background()
	result := transmitter.Apply(input)

	var gotError bool
	for res := range result.Emit(ctx) {
		if res.IsError() {
			gotError = true
		}
	}

	if !gotError {
		t.Error("Transmitter should propagate errors")
	}
}
