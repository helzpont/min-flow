package io

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

func TestReadLines(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "empty file",
			content:  "",
			expected: []string{},
		},
		{
			name:     "single line no newline",
			content:  "hello",
			expected: []string{"hello"},
		},
		{
			name:     "single line with newline",
			content:  "hello\n",
			expected: []string{"hello"},
		},
		{
			name:     "multiple lines",
			content:  "line1\nline2\nline3\n",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			name:     "lines with spaces",
			content:  "  hello  \n  world  \n",
			expected: []string{"  hello  ", "  world  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "test.txt")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			ctx := context.Background()
			stream := ReadLines(tmpFile)

			var results []string
			for res := range stream.Emit(ctx) {
				if res.IsError() {
					t.Fatalf("unexpected error: %v", res.Error())
				}
				results = append(results, res.Value())
			}

			if len(results) != len(tt.expected) {
				t.Errorf("got %d lines, want %d", len(results), len(tt.expected))
				return
			}

			for i, line := range results {
				if line != tt.expected[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestReadLines_FileNotFound(t *testing.T) {
	ctx := context.Background()
	stream := ReadLines("/nonexistent/path/file.txt")

	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (error), got %d", len(results))
	}

	if !results[0].IsError() {
		t.Error("expected error result for nonexistent file")
	}
}

func TestReadLines_ContextCancellation(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "large.txt")
	var content strings.Builder
	for i := 0; i < 10000; i++ {
		content.WriteString("line\n")
	}
	if err := os.WriteFile(tmpFile, []byte(content.String()), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	stream := ReadLines(tmpFile)

	count := 0
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		count++
		if count >= 100 {
			cancel()
			break
		}
	}

	if count < 100 {
		t.Errorf("expected at least 100 lines before cancel, got %d", count)
	}
}

func TestReadLinesFrom(t *testing.T) {
	content := "line1\nline2\nline3\n"
	reader := strings.NewReader(content)

	ctx := context.Background()
	stream := ReadLinesFrom(reader)

	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}

	expected := []string{"line1", "line2", "line3"}
	if len(results) != len(expected) {
		t.Errorf("got %d lines, want %d", len(results), len(expected))
		return
	}

	for i, line := range results {
		if line != expected[i] {
			t.Errorf("line %d: got %q, want %q", i, line, expected[i])
		}
	}
}

func TestReadBytes(t *testing.T) {
	content := []byte("hello world this is a test")
	tmpFile := filepath.Join(t.TempDir(), "test.bin")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	ctx := context.Background()
	stream := ReadBytes(tmpFile, 5)

	var result []byte
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		result = append(result, res.Value()...)
	}

	if !bytes.Equal(result, content) {
		t.Errorf("got %q, want %q", result, content)
	}
}

func TestWriteLines(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 3)
		out <- core.Ok("line1")
		out <- core.Ok("line2")
		out <- core.Ok("line3")
		close(out)
		return out
	})

	output := WriteLines(tmpFile).Apply(ctx, input)

	var results []string
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	expected := "line1\nline2\nline3\n"
	if string(content) != expected {
		t.Errorf("file content: got %q, want %q", string(content), expected)
	}
}

func TestAppendLines(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	if err := os.WriteFile(tmpFile, []byte("existing\n"), 0644); err != nil {
		t.Fatalf("failed to create initial file: %v", err)
	}

	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 2)
		out <- core.Ok("new1")
		out <- core.Ok("new2")
		close(out)
		return out
	})

	output := AppendLines(tmpFile).Apply(ctx, input)

	for range output.Emit(ctx) {
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	expected := "existing\nnew1\nnew2\n"
	if string(content) != expected {
		t.Errorf("file content: got %q, want %q", string(content), expected)
	}
}

func TestWriteTo(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()

	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 2)
		out <- core.Ok("hello")
		out <- core.Ok("world")
		close(out)
		return out
	})

	output := WriteTo(&buf).Apply(ctx, input)

	for range output.Emit(ctx) {
	}

	expected := "hello\nworld\n"
	if buf.String() != expected {
		t.Errorf("buffer content: got %q, want %q", buf.String(), expected)
	}
}

func TestWriteLines_ErrorPassthrough(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	ctx := context.Background()
	testErr := core.Err[string](os.ErrNotExist)

	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 3)
		out <- core.Ok("line1")
		out <- testErr
		out <- core.Ok("line3")
		close(out)
		return out
	})

	output := WriteLines(tmpFile).Apply(ctx, input)

	var values []string
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
