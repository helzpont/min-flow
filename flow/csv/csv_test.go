package csv

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

func TestReadRecords(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected [][]string
	}{
		{
			name:     "simple CSV",
			content:  "a,b,c\n1,2,3\n4,5,6\n",
			expected: [][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5", "6"}},
		},
		{
			name:     "empty file",
			content:  "",
			expected: [][]string{},
		},
		{
			name:     "single row",
			content:  "a,b,c\n",
			expected: [][]string{{"a", "b", "c"}},
		},
		{
			name:     "quoted fields",
			content:  "\"hello, world\",\"test\"\n",
			expected: [][]string{{"hello, world", "test"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "test.csv")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			ctx := context.Background()
			stream := ReadRecords(tmpFile)
			var results [][]string
			for res := range stream.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				results = append(results, res.Value())
			}
			if len(results) != len(tt.expected) {
				t.Errorf("got %d records, want %d", len(results), len(tt.expected))
				return
			}
			for i, record := range results {
				if len(record) != len(tt.expected[i]) {
					t.Errorf("record %d: got %d fields, want %d", i, len(record), len(tt.expected[i]))
					continue
				}
				for j, field := range record {
					if field != tt.expected[i][j] {
						t.Errorf("record %d, field %d: got %q, want %q", i, j, field, tt.expected[i][j])
					}
				}
			}
		})
	}
}

func TestReadRecordsFrom(t *testing.T) {
	content := "name,age\nAlice,30\nBob,25\n"
	reader := strings.NewReader(content)
	ctx := context.Background()
	stream := ReadRecordsFrom(reader)
	var results [][]string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	expected := [][]string{{"name", "age"}, {"Alice", "30"}, {"Bob", "25"}}
	if len(results) != len(expected) {
		t.Errorf("got %d records, want %d", len(results), len(expected))
	}
}

func TestReadRecordsWithOptions(t *testing.T) {
	content := "a;b;c\n1;2;3\n"
	tmpFile := filepath.Join(t.TempDir(), "test.csv")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	ctx := context.Background()
	stream := ReadRecordsWithOptions(tmpFile, WithComma(';'))
	var results [][]string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	expected := [][]string{{"a", "b", "c"}, {"1", "2", "3"}}
	if len(results) != len(expected) {
		t.Errorf("got %d records, want %d", len(results), len(expected))
	}
}

func TestWriteRecords(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.csv")
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], 3)
		out <- core.Ok([]string{"name", "age"})
		out <- core.Ok([]string{"Alice", "30"})
		out <- core.Ok([]string{"Bob", "25"})
		close(out)
		return out
	})
	output := WriteRecords(tmpFile).Apply(ctx, input)
	var count int
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		count++
	}
	if count != 3 {
		t.Errorf("expected 3 records, got %d", count)
	}
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	expected := "name,age\nAlice,30\nBob,25\n"
	if string(content) != expected {
		t.Errorf("file content: got %q, want %q", string(content), expected)
	}
}

func TestWriteRecordsTo(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], 2)
		out <- core.Ok([]string{"a", "b"})
		out <- core.Ok([]string{"1", "2"})
		close(out)
		return out
	})
	output := WriteRecordsTo(&buf).Apply(ctx, input)
	for range output.Emit(ctx) {
	}
	expected := "a,b\n1,2\n"
	if buf.String() != expected {
		t.Errorf("buffer content: got %q, want %q", buf.String(), expected)
	}
}

func TestSkipHeader(t *testing.T) {
	content := "name,age\nAlice,30\nBob,25\n"
	reader := strings.NewReader(content)
	ctx := context.Background()
	stream := ReadRecordsFrom(reader)
	output := SkipHeader().Apply(ctx, stream)
	var results [][]string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	expected := [][]string{{"Alice", "30"}, {"Bob", "25"}}
	if len(results) != len(expected) {
		t.Errorf("got %d records, want %d", len(results), len(expected))
	}
}

func TestReadRecords_FileNotFound(t *testing.T) {
	ctx := context.Background()
	stream := ReadRecords("/nonexistent/path/file.csv")
	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (error), got %d", len(results))
	}
	if !results[0].IsError() {
		t.Error("expected error result for nonexistent file")
	}
}

func TestDecodeCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected [][]string
	}{
		{
			name:     "simple CSV",
			input:    []byte("a,b,c\n1,2,3\n"),
			expected: [][]string{{"a", "b", "c"}, {"1", "2", "3"}},
		},
		{
			name:     "single row",
			input:    []byte("hello,world\n"),
			expected: [][]string{{"hello", "world"}},
		},
		{
			name:     "quoted fields",
			input:    []byte("\"hello, world\",test\n"),
			expected: [][]string{{"hello, world", "test"}},
		},
		{
			name:     "multiline quoted field",
			input:    []byte("\"line1\nline2\",value\n"),
			expected: [][]string{{"line1\nline2", "value"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			input := core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
				out := make(chan core.Result[[]byte], 1)
				out <- core.Ok(tt.input)
				close(out)
				return out
			})

			output := DecodeCSV().Apply(ctx, input)

			var results [][]string
			for res := range output.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				results = append(results, res.Value())
			}

			if len(results) != len(tt.expected) {
				t.Errorf("got %d records, want %d", len(results), len(tt.expected))
				return
			}

			for i, record := range results {
				if len(record) != len(tt.expected[i]) {
					t.Errorf("record %d: got %d fields, want %d", i, len(record), len(tt.expected[i]))
					continue
				}
				for j, field := range record {
					if field != tt.expected[i][j] {
						t.Errorf("record %d, field %d: got %q, want %q", i, j, field, tt.expected[i][j])
					}
				}
			}
		})
	}
}

func TestDecodeCSV_ChunkedInput(t *testing.T) {
	// Test that DecodeCSV handles data split across chunks
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], 3)
		// Split "a,b\n1,2\n" across chunks
		out <- core.Ok([]byte("a,b\n"))
		out <- core.Ok([]byte("1,2"))
		out <- core.Ok([]byte("\n"))
		close(out)
		return out
	})

	output := DecodeCSV().Apply(ctx, input)

	var results [][]string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}

	expected := [][]string{{"a", "b"}, {"1", "2"}}
	if len(results) != len(expected) {
		t.Errorf("got %d records, want %d", len(results), len(expected))
	}
}

func TestDecodeCSV_WithOptions(t *testing.T) {
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], 1)
		out <- core.Ok([]byte("a;b;c\n1;2;3\n"))
		close(out)
		return out
	})

	output := DecodeCSVBuffered(DefaultBufferSize, WithComma(';')).Apply(ctx, input)

	var results [][]string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}

	expected := [][]string{{"a", "b", "c"}, {"1", "2", "3"}}
	if len(results) != len(expected) {
		t.Errorf("got %d records, want %d", len(results), len(expected))
	}
}

func TestEncodeCSV(t *testing.T) {
	tests := []struct {
		name     string
		records  [][]string
		expected string
	}{
		{
			name:     "simple records",
			records:  [][]string{{"a", "b", "c"}, {"1", "2", "3"}},
			expected: "a,b,c\n1,2,3\n",
		},
		{
			name:     "single record",
			records:  [][]string{{"hello", "world"}},
			expected: "hello,world\n",
		},
		{
			name:     "fields with commas",
			records:  [][]string{{"hello, world", "test"}},
			expected: "\"hello, world\",test\n",
		},
		{
			name:     "fields with quotes",
			records:  [][]string{{"say \"hello\"", "ok"}},
			expected: "\"say \"\"hello\"\"\",ok\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			input := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
				out := make(chan core.Result[[]string], len(tt.records))
				for _, record := range tt.records {
					out <- core.Ok(record)
				}
				close(out)
				return out
			})

			output := EncodeCSV().Apply(ctx, input)

			var result bytes.Buffer
			for res := range output.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				result.Write(res.Value())
			}

			if result.String() != tt.expected {
				t.Errorf("got %q, want %q", result.String(), tt.expected)
			}
		})
	}
}

func TestEncodeCSV_WithOptions(t *testing.T) {
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], 2)
		out <- core.Ok([]string{"a", "b", "c"})
		out <- core.Ok([]string{"1", "2", "3"})
		close(out)
		return out
	})

	output := EncodeCSVBuffered(DefaultBufferSize, WithWriterComma(';')).Apply(ctx, input)

	var result bytes.Buffer
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		result.Write(res.Value())
	}

	expected := "a;b;c\n1;2;3\n"
	if result.String() != expected {
		t.Errorf("got %q, want %q", result.String(), expected)
	}
}

func TestEncodeCSV_ErrorPassthrough(t *testing.T) {
	ctx := context.Background()
	testErr := core.Err[[]string](os.ErrNotExist)

	input := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], 3)
		out <- core.Ok([]string{"a", "b"})
		out <- testErr
		out <- core.Ok([]string{"1", "2"})
		close(out)
		return out
	})

	output := EncodeCSV().Apply(ctx, input)

	var values int
	var errors int
	for res := range output.Emit(ctx) {
		if res.IsError() {
			errors++
		} else {
			values++
		}
	}

	if values != 2 {
		t.Errorf("expected 2 values, got %d", values)
	}
	if errors != 1 {
		t.Errorf("expected 1 error, got %d", errors)
	}
}

func TestDecodeCSV_EncodeCSV_Roundtrip(t *testing.T) {
	// Test that encoding then decoding produces the same records
	ctx := context.Background()
	original := [][]string{
		{"name", "age", "city"},
		{"Alice", "30", "New York"},
		{"Bob", "25", "Los Angeles"},
	}

	// Encode records to bytes
	encodeInput := core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], len(original))
		for _, record := range original {
			out <- core.Ok(record)
		}
		close(out)
		return out
	})

	var encoded bytes.Buffer
	encodeOutput := EncodeCSV().Apply(ctx, encodeInput)
	for res := range encodeOutput.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("encode error: %v", res.Error())
		}
		encoded.Write(res.Value())
	}

	// Decode bytes back to records
	decodeInput := core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], 1)
		out <- core.Ok(encoded.Bytes())
		close(out)
		return out
	})

	decodeOutput := DecodeCSV().Apply(ctx, decodeInput)

	var decoded [][]string
	for res := range decodeOutput.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("decode error: %v", res.Error())
		}
		decoded = append(decoded, res.Value())
	}

	if len(decoded) != len(original) {
		t.Errorf("got %d records, want %d", len(decoded), len(original))
		return
	}

	for i, record := range decoded {
		for j, field := range record {
			if field != original[i][j] {
				t.Errorf("record %d, field %d: got %q, want %q", i, j, field, original[i][j])
			}
		}
	}
}
