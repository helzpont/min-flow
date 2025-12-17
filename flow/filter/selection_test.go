package filter_test

import (
	"context"
	"testing"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/filter"
)

func TestFind(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		wantValue int
		wantFound bool
	}{
		{
			name:      "finds first match",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v > 2 },
			wantValue: 3,
			wantFound: true,
		},
		{
			name:      "no match returns not found",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			wantValue: 0,
			wantFound: false,
		},
		{
			name:      "empty stream returns not found",
			input:     []int{},
			predicate: func(v int) bool { return true },
			wantValue: 0,
			wantFound: false,
		},
		{
			name:      "finds exact value",
			input:     []int{10, 20, 30, 40},
			predicate: func(v int) bool { return v == 30 },
			wantValue: 30,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.Find(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0].Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", got[0].Found, tt.wantFound)
			}
			if got[0].Found && got[0].Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", got[0].Value, tt.wantValue)
			}
		})
	}
}

func TestFindPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.Find[int](nil)
}

func TestFindIndex(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      int
	}{
		{
			name:      "finds first match index",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v > 2 },
			want:      2,
		},
		{
			name:      "no match returns -1",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			want:      -1,
		},
		{
			name:      "empty stream returns -1",
			input:     []int{},
			predicate: func(v int) bool { return true },
			want:      -1,
		},
		{
			name:      "finds first element",
			input:     []int{10, 20, 30},
			predicate: func(v int) bool { return v == 10 },
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.FindIndex(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestFindIndexPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.FindIndex[int](nil)
}

func TestFindLast(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		wantValue int
		wantFound bool
	}{
		{
			name:      "finds last match",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v < 4 },
			wantValue: 3,
			wantFound: true,
		},
		{
			name:      "no match returns not found",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			wantValue: 0,
			wantFound: false,
		},
		{
			name:      "finds last of duplicates",
			input:     []int{1, 2, 2, 2, 5},
			predicate: func(v int) bool { return v == 2 },
			wantValue: 2,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.FindLast(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 result, got %d", len(got))
			}
			if got[0].Found != tt.wantFound {
				t.Errorf("Found = %v, want %v", got[0].Found, tt.wantFound)
			}
			if got[0].Found && got[0].Value != tt.wantValue {
				t.Errorf("Value = %v, want %v", got[0].Value, tt.wantValue)
			}
		})
	}
}

func TestFindLastPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.FindLast[int](nil)
}

func TestFindLastIndex(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      int
	}{
		{
			name:      "finds last match index",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v < 4 },
			want:      2,
		},
		{
			name:      "no match returns -1",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			want:      -1,
		},
		{
			name:      "finds last of duplicates",
			input:     []int{1, 2, 2, 2, 5},
			predicate: func(v int) bool { return v == 2 },
			want:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.FindLastIndex(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestFindLastIndexPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.FindLastIndex[int](nil)
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		value int
		want  bool
	}{
		{
			name:  "value exists",
			input: []int{1, 2, 3, 4, 5},
			value: 3,
			want:  true,
		},
		{
			name:  "value does not exist",
			input: []int{1, 2, 3, 4, 5},
			value: 10,
			want:  false,
		},
		{
			name:  "empty stream",
			input: []int{},
			value: 1,
			want:  false,
		},
		{
			name:  "first element",
			input: []int{1, 2, 3},
			value: 1,
			want:  true,
		},
		{
			name:  "last element",
			input: []int{1, 2, 3},
			value: 3,
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.Contains(tt.value)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestContainsBy(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      bool
	}{
		{
			name:      "matching element exists",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v > 3 },
			want:      true,
		},
		{
			name:      "no matching element",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			want:      false,
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(v int) bool { return true },
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.ContainsBy(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestContainsByPanicOnNilPredicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil predicate")
		}
	}()
	filter.ContainsBy[int](nil)
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  bool
	}{
		{
			name:  "empty stream",
			input: []int{},
			want:  true,
		},
		{
			name:  "single element",
			input: []int{1},
			want:  false,
		},
		{
			name:  "multiple elements",
			input: []int{1, 2, 3},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.IsEmpty[int]()
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestIsNotEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  bool
	}{
		{
			name:  "empty stream",
			input: []int{},
			want:  false,
		},
		{
			name:  "single element",
			input: []int{1},
			want:  true,
		},
		{
			name:  "multiple elements",
			input: []int{1, 2, 3},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.IsNotEmpty[int]()
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestSequenceEqual(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		other []int
		want  bool
	}{
		{
			name:  "equal sequences",
			input: []int{1, 2, 3},
			other: []int{1, 2, 3},
			want:  true,
		},
		{
			name:  "different values",
			input: []int{1, 2, 3},
			other: []int{1, 2, 4},
			want:  false,
		},
		{
			name:  "different lengths - other shorter",
			input: []int{1, 2, 3},
			other: []int{1, 2},
			want:  false,
		},
		{
			name:  "different lengths - other longer",
			input: []int{1, 2},
			other: []int{1, 2, 3},
			want:  false,
		},
		{
			name:  "both empty",
			input: []int{},
			other: []int{},
			want:  true,
		},
		{
			name:  "source empty other not",
			input: []int{},
			other: []int{1},
			want:  false,
		},
		{
			name:  "other empty source not",
			input: []int{1},
			other: []int{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			other := flow.FromSlice(tt.other)
			transformer := filter.SequenceEqual(other)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestSequenceEqualBy(t *testing.T) {
	type item struct {
		id   int
		name string
	}

	tests := []struct {
		name  string
		input []item
		other []item
		want  bool
	}{
		{
			name:  "equal by id",
			input: []item{{1, "a"}, {2, "b"}},
			other: []item{{1, "x"}, {2, "y"}},
			want:  true,
		},
		{
			name:  "not equal by id",
			input: []item{{1, "a"}, {2, "b"}},
			other: []item{{1, "x"}, {3, "y"}},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			other := flow.FromSlice(tt.other)
			equals := func(a, b item) bool { return a.id == b.id }
			transformer := filter.SequenceEqualBy(other, equals)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestSequenceEqualByPanicOnNilEquals(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil equals function")
		}
	}()
	filter.SequenceEqualBy[int](flow.FromSlice([]int{}), nil)
}

func TestCountIf(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		want      int
	}{
		{
			name:      "count matching elements",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(v int) bool { return v > 2 },
			want:      3,
		},
		{
			name:      "no matches",
			input:     []int{1, 2, 3},
			predicate: func(v int) bool { return v > 10 },
			want:      0,
		},
		{
			name:      "nil predicate counts all",
			input:     []int{1, 2, 3, 4, 5},
			predicate: nil,
			want:      5,
		},
		{
			name:      "empty stream",
			input:     []int{},
			predicate: func(v int) bool { return true },
			want:      0,
		},
		{
			name:      "count evens",
			input:     []int{1, 2, 3, 4, 5, 6},
			predicate: func(v int) bool { return v%2 == 0 },
			want:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.CountIf(tt.predicate)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		value int
		want  int
	}{
		{
			name:  "finds value",
			input: []int{10, 20, 30, 40},
			value: 30,
			want:  2,
		},
		{
			name:  "value not found",
			input: []int{10, 20, 30},
			value: 99,
			want:  -1,
		},
		{
			name:  "first element",
			input: []int{10, 20, 30},
			value: 10,
			want:  0,
		},
		{
			name:  "finds first occurrence",
			input: []int{1, 2, 2, 2, 3},
			value: 2,
			want:  1,
		},
		{
			name:  "empty stream",
			input: []int{},
			value: 1,
			want:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.IndexOf(tt.value)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestLastIndexOf(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		value int
		want  int
	}{
		{
			name:  "finds last occurrence",
			input: []int{1, 2, 2, 2, 3},
			value: 2,
			want:  3,
		},
		{
			name:  "value not found",
			input: []int{10, 20, 30},
			value: 99,
			want:  -1,
		},
		{
			name:  "single occurrence",
			input: []int{10, 20, 30},
			value: 20,
			want:  1,
		},
		{
			name:  "last element",
			input: []int{10, 20, 30},
			value: 30,
			want:  2,
		},
		{
			name:  "empty stream",
			input: []int{},
			value: 1,
			want:  -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stream := flow.FromSlice(tt.input)
			transformer := filter.LastIndexOf(tt.value)
			result := transformer.Apply(stream)
			got, err := flow.Slice(ctx, result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 || got[0] != tt.want {
				t.Errorf("got %v, want [%v]", got, tt.want)
			}
		})
	}
}

func TestSelectionContextCancellation(t *testing.T) {
	t.Run("Find respects cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		transformer := filter.Find(func(int) bool { return false })
		result := transformer.Apply(stream)
		_, _ = flow.Slice(ctx, result)
		// Just ensure it doesn't hang
	})

	t.Run("Contains respects cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		stream := flow.FromSlice([]int{1, 2, 3, 4, 5})
		transformer := filter.Contains(10)
		result := transformer.Apply(stream)
		_, _ = flow.Slice(ctx, result)
		// Just ensure it doesn't hang
	})
}

func TestSelectionWithStrings(t *testing.T) {
	t.Run("Find string", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]string{"apple", "banana", "cherry"})
		transformer := filter.Find(func(s string) bool { return len(s) > 5 })
		result := transformer.Apply(stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || !got[0].Found || got[0].Value != "banana" {
			t.Errorf("got %v, want banana found", got)
		}
	})

	t.Run("Contains string", func(t *testing.T) {
		ctx := context.Background()
		stream := flow.FromSlice([]string{"apple", "banana", "cherry"})
		transformer := filter.Contains("banana")
		result := transformer.Apply(stream)
		got, err := flow.Slice(ctx, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || !got[0] {
			t.Errorf("got %v, want [true]", got)
		}
	})
}
