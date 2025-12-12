// Package io provides stream adapters for file I/O operations.
// It enables reading from and writing to files as part of flow pipelines.
package io

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is re-exported from core for convenience.
const DefaultBufferSize = core.DefaultBufferSize

// ReadLines creates a Stream that emits each line from the given file path.
// Lines are emitted without the trailing newline character.
// The stream completes after all lines have been read.
// If the file cannot be opened, the stream emits an error and completes.
func ReadLines(path string) core.Stream[string] {
	return ReadLinesBuffered(path, DefaultBufferSize)
}

// ReadLinesBuffered creates a Stream that emits lines with a specified buffer size.
func ReadLinesBuffered(path string, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)

		go func() {
			defer close(out)

			file, err := os.Open(path)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(scanner.Text()):
				}
			}

			if err := scanner.Err(); err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
			}
		}()

		return out
	})
}

// ReadLinesFrom creates a Stream that reads lines from an io.Reader.
// This is useful for reading from stdin, network connections, or other readers.
func ReadLinesFrom(r io.Reader) core.Stream[string] {
	return ReadLinesFromBuffered(r, DefaultBufferSize)
}

// ReadLinesFromBuffered creates a Stream that reads lines with a specified buffer size.
func ReadLinesFromBuffered(r io.Reader, bufferSize int) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)

		go func() {
			defer close(out)

			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(scanner.Text()):
				}
			}

			if err := scanner.Err(); err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
			}
		}()

		return out
	})
}

// ReadBytes creates a Stream that reads the file in chunks of the specified size.
// Useful for processing binary files or large files without loading them entirely into memory.
func ReadBytes(path string, chunkSize int) core.Stream[[]byte] {
	return ReadBytesBuffered(path, chunkSize, DefaultBufferSize)
}

// ReadBytesBuffered creates a Stream that reads bytes with a specified channel buffer size.
func ReadBytesBuffered(path string, chunkSize int, bufferSize int) core.Stream[[]byte] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], bufferSize)

		go func() {
			defer close(out)

			file, err := os.Open(path)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]byte](err):
				}
				return
			}
			defer file.Close()

			buf := make([]byte, chunkSize)
			for {
				n, err := file.Read(buf)
				if n > 0 {
					chunk := make([]byte, n)
					copy(chunk, buf[:n])

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(chunk):
					}
				}

				if err == io.EOF {
					return
				}
				if err != nil {
					select {
					case <-ctx.Done():
					case out <- core.Err[[]byte](err):
					}
					return
				}
			}
		}()

		return out
	})
}

// WriteLines creates a Transformer that writes each string to a file, one per line.
// The file is created if it doesn't exist, or truncated if it does.
// Items pass through unchanged after being written.
func WriteLines(path string) core.Transformer[string, string] {
	return WriteLinesWithOptions(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

// AppendLines creates a Transformer that appends each string to a file, one per line.
// The file is created if it doesn't exist.
// Items pass through unchanged after being written.
func AppendLines(path string) core.Transformer[string, string] {
	return WriteLinesWithOptions(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// WriteLinesWithOptions creates a Transformer that writes lines with custom file options.
func WriteLinesWithOptions(path string, flag int, perm os.FileMode) core.Transformer[string, string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[string] {
		out := make(chan core.Result[string], DefaultBufferSize)

		go func() {
			defer close(out)

			file, err := os.OpenFile(path, flag, perm)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			defer file.Close()

			writer := bufio.NewWriter(file)
			defer writer.Flush()

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

				line := res.Value()
				if _, err := writer.WriteString(line + "\n"); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[string](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// WriteTo creates a Transformer that writes each string to an io.Writer.
// Items pass through unchanged after being written.
func WriteTo(w io.Writer) core.Transformer[string, string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[string] {
		out := make(chan core.Result[string], DefaultBufferSize)

		go func() {
			defer close(out)

			writer := bufio.NewWriter(w)
			defer writer.Flush()

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

				line := res.Value()
				if _, err := writer.WriteString(line + "\n"); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[string](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}
