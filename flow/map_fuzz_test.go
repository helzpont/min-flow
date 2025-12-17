package flow_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
)

func FuzzMapperApply(f *testing.F) {
	f.Add(0)
	f.Add(1)
	f.Add(-1)
	f.Add(5)
	f.Add(11)

	f.Fuzz(func(t *testing.T, n int) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		mapper := flow.Map(func(x int) (int, error) {
			switch {
			case x%11 == 0:
				return 0, fmt.Errorf("err-%d", x)
			case x%5 == 0:
				panic("panic-path")
			case x < 0:
				return -x, nil
			default:
				return x * 2, nil
			}
		})

		stream := flow.FromSlice([]int{n})
		results := flow.Collect(ctx, mapper.Apply(stream))

		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		res := results[0]
		switch {
		case res.IsValue():
			expected := n * 2
			if n < 0 {
				expected = -n
			}
			if res.Value() != expected {
				t.Fatalf("value mismatch: got %d want %d", res.Value(), expected)
			}
		case res.IsError():
			if n%11 != 0 && n%5 != 0 {
				t.Fatalf("unexpected error for input %d: %v", n, res.Error())
			}
		case res.IsSentinel():
			t.Fatalf("unexpected sentinel for input %d", n)
		}
	})
}
