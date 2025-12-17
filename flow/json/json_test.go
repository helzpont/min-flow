package json

import (
	"context"
	"strings"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []Person
		errors   int
	}{
		{
			name:     "valid JSON objects",
			input:    []string{`{"name":"Alice","age":30}`, `{"name":"Bob","age":25}`},
			expected: []Person{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 25}},
			errors:   0,
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []Person{},
			errors:   0,
		},
		{
			name:     "invalid JSON",
			input:    []string{`{"name":"Alice"`, `{"name":"Bob","age":25}`},
			expected: []Person{{Name: "Bob", Age: 25}},
			errors:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
				out := make(chan core.Result[string], len(tt.input))
				for _, s := range tt.input {
					out <- core.Ok(s)
				}
				close(out)
				return out
			})

			output := Decode[Person]().Apply(input)

			var values []Person
			var errors int
			for res := range output.Emit(ctx) {
				if res.IsError() {
					errors++
				} else {
					values = append(values, res.Value())
				}
			}

			if len(values) != len(tt.expected) {
				t.Errorf("got %d values, want %d", len(values), len(tt.expected))
				return
			}

			for i, v := range values {
				if v != tt.expected[i] {
					t.Errorf("value %d: got %+v, want %+v", i, v, tt.expected[i])
				}
			}

			if errors != tt.errors {
				t.Errorf("got %d errors, want %d", errors, tt.errors)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[Person] {
		out := make(chan core.Result[Person], 2)
		out <- core.Ok(Person{Name: "Alice", Age: 30})
		out <- core.Ok(Person{Name: "Bob", Age: 25})
		close(out)
		return out
	})

	output := Encode[Person]().Apply(input)

	var values []string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		values = append(values, res.Value())
	}

	expected := []string{`{"name":"Alice","age":30}`, `{"name":"Bob","age":25}`}
	if len(values) != len(expected) {
		t.Errorf("got %d values, want %d", len(values), len(expected))
		return
	}

	for i, v := range values {
		if v != expected[i] {
			t.Errorf("value %d: got %q, want %q", i, v, expected[i])
		}
	}
}

func TestDecodeStream(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Person
	}{
		{
			name:     "NDJSON stream",
			input:    "{\"name\":\"Alice\",\"age\":30}\n{\"name\":\"Bob\",\"age\":25}\n",
			expected: []Person{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 25}},
		},
		{
			name:     "compact JSON objects",
			input:    "{\"name\":\"Alice\",\"age\":30}{\"name\":\"Bob\",\"age\":25}",
			expected: []Person{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 25}},
		},
		{
			name:     "empty input",
			input:    "",
			expected: []Person{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := strings.NewReader(tt.input)
			stream := DecodeStream[Person](reader)

			var values []Person
			for res := range stream.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				values = append(values, res.Value())
			}

			if len(values) != len(tt.expected) {
				t.Errorf("got %d values, want %d", len(values), len(tt.expected))
				return
			}

			for i, v := range values {
				if v != tt.expected[i] {
					t.Errorf("value %d: got %+v, want %+v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDecodeArray(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Person
	}{
		{
			name:     "JSON array",
			input:    `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`,
			expected: []Person{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 25}},
		},
		{
			name:     "empty array",
			input:    `[]`,
			expected: []Person{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader := strings.NewReader(tt.input)
			stream := DecodeArray[Person](reader)

			var values []Person
			for res := range stream.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				values = append(values, res.Value())
			}

			if len(values) != len(tt.expected) {
				t.Errorf("got %d values, want %d", len(values), len(tt.expected))
				return
			}

			for i, v := range values {
				if v != tt.expected[i] {
					t.Errorf("value %d: got %+v, want %+v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestDecode_ErrorPassthrough(t *testing.T) {
	ctx := context.Background()
	testErr := core.Err[string](context.Canceled)

	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 3)
		out <- core.Ok(`{"name":"Alice","age":30}`)
		out <- testErr
		out <- core.Ok(`{"name":"Bob","age":25}`)
		close(out)
		return out
	})

	output := Decode[Person]().Apply(input)

	var values []Person
	var errors int
	for res := range output.Emit(ctx) {
		if res.IsError() {
			errors++
		} else {
			values = append(values, res.Value())
		}
	}

	if len(values) != 2 {
		t.Errorf("expected 2 values, got %d", len(values))
	}
	if errors != 1 {
		t.Errorf("expected 1 error, got %d", errors)
	}
}

func TestDecodeBytes(t *testing.T) {
	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], 2)
		out <- core.Ok([]byte(`{"name":"Alice","age":30}`))
		out <- core.Ok([]byte(`{"name":"Bob","age":25}`))
		close(out)
		return out
	})

	output := DecodeBytes[Person]().Apply(input)

	var values []Person
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		values = append(values, res.Value())
	}

	expected := []Person{{Name: "Alice", Age: 30}, {Name: "Bob", Age: 25}}
	if len(values) != len(expected) {
		t.Errorf("got %d values, want %d", len(values), len(expected))
	}
}

func TestEncodeBytes(t *testing.T) {
	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[Person] {
		out := make(chan core.Result[Person], 2)
		out <- core.Ok(Person{Name: "Alice", Age: 30})
		out <- core.Ok(Person{Name: "Bob", Age: 25})
		close(out)
		return out
	})

	output := EncodeBytes[Person]().Apply(input)

	var values []string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		values = append(values, string(res.Value()))
	}

	expected := []string{`{"name":"Alice","age":30}`, `{"name":"Bob","age":25}`}
	if len(values) != len(expected) {
		t.Errorf("got %d values, want %d", len(values), len(expected))
	}
}
