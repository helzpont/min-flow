// Package main demonstrates file system operations with glob patterns,
// directory traversal, and batch file processing.
package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/aggregate"
	"github.com/lguimbarda/min-flow/flow/filter"
	"github.com/lguimbarda/min-flow/flow/glob"
	"github.com/lguimbarda/min-flow/flow/io"
	"github.com/lguimbarda/min-flow/flow/parallel"
)

// FileStats represents statistics about a file
type FileStats struct {
	Path     string
	Size     int64
	Checksum string
}

// DirectoryStats represents aggregated directory statistics
type DirectoryStats struct {
	TotalFiles int
	TotalSize  int64
	Extensions map[string]int
}

func main() {
	fmt.Println("=== File System Pipeline Examples ===")
	fmt.Println()

	// Create test directory structure
	testDir := setupTestDirectory()
	defer os.RemoveAll(testDir)

	directoryAnalysis(testDir)
	fileFiltering(testDir)
	parallelFileProcessing(testDir)
	fileAggregation(testDir)
}

// directoryAnalysis walks a directory and collects file information
func directoryAnalysis(dir string) {
	fmt.Println("--- Directory Analysis ---")
	ctx := context.Background()

	// Walk all files in the directory
	files := glob.WalkFiles(dir)

	// Get file info for each
	stats := glob.Stat().Apply(ctx, files)

	// Collect results
	fmt.Println("Files found:")
	var totalSize int64
	for res := range stats.All(ctx) {
		if res.IsValue() {
			info := res.Value()
			fmt.Printf("  %s (%d bytes)\n", info.Name, info.Size)
			totalSize += info.Size
		}
	}
	fmt.Printf("Total size: %d bytes\n\n", totalSize)
}

// fileFiltering demonstrates filtering files by pattern and properties
func fileFiltering(dir string) {
	fmt.Println("--- File Filtering ---")
	ctx := context.Background()

	// Walk all files
	files := glob.WalkFiles(dir)

	// Filter by extension using glob pattern
	txtFiles := glob.Filter("*.txt").Apply(ctx, files)

	fmt.Println("Text files only:")
	for res := range txtFiles.All(ctx) {
		if res.IsValue() {
			fmt.Printf("  %s\n", filepath.Base(res.Value()))
		}
	}

	// Filter by size (files larger than 20 bytes)
	allFiles := glob.WalkFiles(dir)
	withInfo := glob.Stat().Apply(ctx, allFiles)
	largeFiles := filter.Where(func(info glob.FileInfo) bool {
		return info.Size > 20
	}).Apply(ctx, withInfo)

	fmt.Println("\nFiles larger than 20 bytes:")
	for res := range largeFiles.All(ctx) {
		if res.IsValue() {
			info := res.Value()
			fmt.Printf("  %s (%d bytes)\n", info.Name, info.Size)
		}
	}
	fmt.Println()
}

// parallelFileProcessing shows parallel file content processing
func parallelFileProcessing(dir string) {
	fmt.Println("--- Parallel File Processing ---")
	ctx := context.Background()

	// Get all text files
	files := glob.WalkFiles(dir)
	txtFiles := glob.Filter("*.txt").Apply(ctx, files)

	// Compute MD5 checksums in parallel
	checksums := parallel.Map(4, func(path string) FileStats {
		content, err := os.ReadFile(path)
		if err != nil {
			return FileStats{Path: path}
		}
		hash := md5.Sum(content)
		info, _ := os.Stat(path)
		return FileStats{
			Path:     path,
			Size:     info.Size(),
			Checksum: fmt.Sprintf("%x", hash),
		}
	}).Apply(ctx, txtFiles)

	fmt.Println("File checksums (computed in parallel):")
	for res := range checksums.All(ctx) {
		if res.IsValue() {
			stats := res.Value()
			fmt.Printf("  %s: %s\n", filepath.Base(stats.Path), stats.Checksum[:8]+"...")
		}
	}
	fmt.Println()
}

// fileAggregation demonstrates aggregating file information
func fileAggregation(dir string) {
	fmt.Println("--- File Aggregation ---")
	ctx := context.Background()

	// Walk all files
	files := glob.WalkFiles(dir)
	withInfo := glob.Stat().Apply(ctx, files)

	// Aggregate statistics using Fold
	stats := aggregate.Fold(
		DirectoryStats{Extensions: make(map[string]int)},
		func(acc DirectoryStats, info glob.FileInfo) DirectoryStats {
			acc.TotalFiles++
			acc.TotalSize += info.Size
			ext := strings.ToLower(filepath.Ext(info.Name))
			if ext == "" {
				ext = "(no ext)"
			}
			acc.Extensions[ext]++
			return acc
		},
	).Apply(ctx, withInfo)

	// Print statistics
	for res := range stats.All(ctx) {
		if res.IsValue() {
			s := res.Value()
			fmt.Printf("Directory Statistics:\n")
			fmt.Printf("  Total files: %d\n", s.TotalFiles)
			fmt.Printf("  Total size: %d bytes\n", s.TotalSize)
			fmt.Printf("  Extensions:\n")
			for ext, count := range s.Extensions {
				fmt.Printf("    %s: %d files\n", ext, count)
			}
		}
	}

	// Group files by extension using batch
	fmt.Println("\nFiles grouped by extension:")
	allFiles := glob.WalkFiles(dir)

	// First, transform to include extension
	type FileWithExt struct {
		Path string
		Ext  string
	}
	withExt := flow.Map(func(path string) (FileWithExt, error) {
		ext := filepath.Ext(path)
		if ext == "" {
			ext = "(none)"
		}
		return FileWithExt{Path: path, Ext: ext}, nil
	}).Apply(ctx, allFiles)

	// Collect and group
	groups := make(map[string][]string)
	for res := range withExt.All(ctx) {
		if res.IsValue() {
			f := res.Value()
			groups[f.Ext] = append(groups[f.Ext], filepath.Base(f.Path))
		}
	}

	for ext, fileList := range groups {
		fmt.Printf("  %s: %v\n", ext, fileList)
	}
	fmt.Println()

	// Process files in batches
	batchProcessing(dir)
}

// batchProcessing shows batch file operations
func batchProcessing(dir string) {
	fmt.Println("--- Batch File Processing ---")
	ctx := context.Background()

	// Get all files
	files := glob.WalkFiles(dir)

	// Read file contents
	contents := flow.FlatMap(func(path string) ([]string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// Return each line as a separate item
		return strings.Split(string(data), "\n"), nil
	}).Apply(ctx, files)

	// Filter non-empty lines
	nonEmpty := filter.Where(func(line string) bool {
		return strings.TrimSpace(line) != ""
	}).Apply(ctx, contents)

	// Batch lines for processing (e.g., for bulk insert)
	batched := aggregate.Batch[string](5).Apply(ctx, nonEmpty)

	fmt.Println("Processing lines in batches of 5:")
	batchNum := 1
	for res := range batched.All(ctx) {
		if res.IsValue() {
			batch := res.Value()
			fmt.Printf("  Batch %d: %d lines\n", batchNum, len(batch))
			batchNum++
		}
	}
	fmt.Println()

	// Write processed output
	outputFile := filepath.Join(dir, "output.txt")
	allFilePaths := glob.WalkFiles(dir)
	paths := filter.Where(func(path string) bool {
		return strings.HasSuffix(path, ".txt") && !strings.Contains(path, "output")
	}).Apply(ctx, allFilePaths)

	// Collect paths and write summary
	pathList := flow.Map(func(path string) (string, error) {
		rel, _ := filepath.Rel(dir, path)
		return "- " + rel, nil
	}).Apply(ctx, paths)

	written := io.WriteLines(outputFile).Apply(ctx, pathList)

	var count int
	for range written.All(ctx) {
		count++
	}
	fmt.Printf("Wrote %d file paths to %s\n", count, filepath.Base(outputFile))
}

// setupTestDirectory creates a test directory with sample files
func setupTestDirectory() string {
	dir, err := os.MkdirTemp("", "fileops-example")
	if err != nil {
		panic(err)
	}

	// Create subdirectories
	os.MkdirAll(filepath.Join(dir, "subdir1"), 0755)
	os.MkdirAll(filepath.Join(dir, "subdir2"), 0755)

	// Create sample files
	fileContents := map[string]string{
		"readme.txt":         "This is a readme file.\nIt has multiple lines.",
		"data.txt":           "Line 1\nLine 2\nLine 3",
		"config.json":        `{"key": "value"}`,
		"script.go":          "package main\n\nfunc main() {}",
		"subdir1/notes.txt":  "Some notes here.",
		"subdir1/data.csv":   "a,b,c\n1,2,3",
		"subdir2/report.txt": "Quarterly report\nRevenue: $1M\nProfit: $100K",
	}

	for name, content := range fileContents {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			panic(err)
		}
	}

	return dir
}
