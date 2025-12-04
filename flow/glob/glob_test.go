package glob

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
)

func setupTestDir(t *testing.T) string {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("content2"), 0644)
	os.WriteFile(filepath.Join(dir, "file3.go"), []byte("content3"), 0644)
	os.WriteFile(filepath.Join(dir, "subdir", "file4.txt"), []byte("content4"), 0644)
	return dir
}

func TestMatch(t *testing.T) {
	dir := setupTestDir(t)
	pattern := filepath.Join(dir, "*.txt")
	ctx := context.Background()
	stream := Match(pattern)
	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 2 {
		t.Errorf("expected 2 matches, got %d", len(results))
	}
}

func TestWalk(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	stream := Walk(dir)
	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	// dir + subdir + 4 files = 6
	if len(results) != 6 {
		t.Errorf("expected 6 paths, got %d: %v", len(results), results)
	}
}

func TestWalkFiles(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	stream := WalkFiles(dir)
	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 4 {
		t.Errorf("expected 4 files, got %d", len(results))
	}
}

func TestWalkDirs(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	stream := WalkDirs(dir)
	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 2 {
		t.Errorf("expected 2 dirs, got %d", len(results))
	}
}

func TestFilter(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	stream := WalkFiles(dir)
	filtered := Filter("*.txt").Apply(ctx, stream)
	var results []string
	for res := range filtered.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 3 {
		t.Errorf("expected 3 .txt files, got %d", len(results))
	}
}

func TestListDir(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	stream := ListDir(dir)
	var results []string
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	// 3 files + 1 subdir = 4
	if len(results) != 4 {
		t.Errorf("expected 4 entries, got %d", len(results))
	}
}

func TestStat(t *testing.T) {
	dir := setupTestDir(t)
	ctx := context.Background()
	input := core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], 2)
		out <- core.Ok(filepath.Join(dir, "file1.txt"))
		out <- core.Ok(filepath.Join(dir, "subdir"))
		close(out)
		return out
	})
	output := Stat().Apply(ctx, input)
	var results []FileInfo
	for res := range output.Emit(ctx) {
		if res.IsError() {
			t.Fatalf("unexpected error: %v", res.Error())
		}
		results = append(results, res.Value())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Name != "file1.txt" {
		t.Errorf("expected file1.txt, got %s", results[0].Name)
	}
	if results[0].IsDir {
		t.Error("file1.txt should not be a directory")
	}
	if results[1].Name != "subdir" {
		t.Errorf("expected subdir, got %s", results[1].Name)
	}
	if !results[1].IsDir {
		t.Error("subdir should be a directory")
	}
}

func TestMatch_InvalidPattern(t *testing.T) {
	ctx := context.Background()
	stream := Match("[invalid")
	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (error), got %d", len(results))
	}
	if !results[0].IsError() {
		t.Error("expected error for invalid pattern")
	}
}

func TestWalk_NonexistentDir(t *testing.T) {
	ctx := context.Background()
	stream := Walk("/nonexistent/path")
	results := stream.Collect(ctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (error), got %d", len(results))
	}
	if !results[0].IsError() {
		t.Error("expected error for nonexistent directory")
	}
}

func TestWalk_ContextCancellation(t *testing.T) {
	dir := setupTestDir(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := Walk(dir)
	count := 0
	for res := range stream.Emit(ctx) {
		if res.IsError() {
			break
		}
		count++
		if count >= 2 {
			cancel()
		}
	}
	if count < 2 {
		t.Error("expected at least 2 paths before cancel")
	}
}
