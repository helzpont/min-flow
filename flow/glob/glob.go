// Package glob provides stream adapters for file path matching and directory traversal.
// It enables file system operations as part of flow pipelines.
package glob

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is the default buffer size for glob operations.
const DefaultBufferSize = 64

// FileInfo contains information about a file or directory.
type FileInfo struct {
	Path    string
	Name    string
	Size    int64
	Mode    fs.FileMode
	IsDir   bool
	ModTime int64
}

// Match creates a Stream that emits file paths matching a glob pattern.
// Patterns are matched using filepath.Glob.
func Match(pattern string) core.Stream[string] {
	return MatchBuffered(pattern, DefaultBufferSize)
}

// MatchBuffered creates a Match stream with a specified buffer size.
func MatchBuffered(pattern string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)
		go func() {
			defer close(out)
			matches, err := filepath.Glob(pattern)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			for _, match := range matches {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(match):
				}
			}
		}()
		return out
	})
}

// Walk creates a Stream that emits all file and directory paths under a root directory.
func Walk(root string) core.Stream[string] {
	return WalkBuffered(root, DefaultBufferSize)
}

// WalkBuffered creates a Walk stream with a specified buffer size.
func WalkBuffered(root string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)
		go func() {
			defer close(out)
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- core.Err[string](err):
					}
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- core.Ok(path):
				}
				return nil
			})
			if err != nil && err != ctx.Err() {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
			}
		}()
		return out
	})
}

// WalkFiles creates a Stream that emits only file paths (not directories).
func WalkFiles(root string) core.Stream[string] {
	return WalkFilesBuffered(root, DefaultBufferSize)
}

// WalkFilesBuffered creates a WalkFiles stream with a specified buffer size.
func WalkFilesBuffered(root string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)
		go func() {
			defer close(out)
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- core.Err[string](err):
					}
					return nil
				}
				if d.IsDir() {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- core.Ok(path):
				}
				return nil
			})
			if err != nil && err != ctx.Err() {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
			}
		}()
		return out
	})
}

// WalkDirs creates a Stream that emits only directory paths.
func WalkDirs(root string) core.Stream[string] {
	return WalkDirsBuffered(root, DefaultBufferSize)
}

// WalkDirsBuffered creates a WalkDirs stream with a specified buffer size.
func WalkDirsBuffered(root string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)
		go func() {
			defer close(out)
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case out <- core.Err[string](err):
					}
					return nil
				}
				if !d.IsDir() {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- core.Ok(path):
				}
				return nil
			})
			if err != nil && err != ctx.Err() {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
			}
		}()
		return out
	})
}

// Filter creates a Transformer that filters paths matching a glob pattern.
// Only paths whose base name matches the pattern are passed through.
func Filter(pattern string) core.Transformer[string, string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[string] {
		out := make(chan core.Result[string], DefaultBufferSize)
		go func() {
			defer close(out)
			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if res.IsError() || res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
					continue
				}
				path := res.Value()
				matched, err := filepath.Match(pattern, filepath.Base(path))
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[string](err):
					}
					continue
				}
				if matched {
					select {
					case <-ctx.Done():
						return
					case out <- res:
					}
				}
			}
		}()
		return out
	})
}

// ListDir creates a Stream that emits immediate children of a directory.
func ListDir(dir string) core.Stream[string] {
	return ListDirBuffered(dir, DefaultBufferSize)
}

// ListDirBuffered creates a ListDir stream with a specified buffer size.
func ListDirBuffered(dir string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)
		go func() {
			defer close(out)
			entries, err := os.ReadDir(dir)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			for _, entry := range entries {
				path := filepath.Join(dir, entry.Name())
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(path):
				}
			}
		}()
		return out
	})
}

// Stat creates a Transformer that retrieves file info for each path.
func Stat() core.Transformer[string, FileInfo] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[FileInfo] {
		out := make(chan core.Result[FileInfo], DefaultBufferSize)
		go func() {
			defer close(out)
			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[FileInfo](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[FileInfo](res.Error()):
					}
					continue
				}
				path := res.Value()
				info, err := os.Stat(path)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[FileInfo](err):
					}
					continue
				}
				fileInfo := FileInfo{
					Path:    path,
					Name:    info.Name(),
					Size:    info.Size(),
					Mode:    info.Mode(),
					IsDir:   info.IsDir(),
					ModTime: info.ModTime().Unix(),
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(fileInfo):
				}
			}
		}()
		return out
	})
}
