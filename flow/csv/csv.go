// Package csv provides stream adapters for CSV encoding and decoding.
// It enables reading and writing CSV data as part of flow pipelines.
package csv

import (
	"context"
	"encoding/csv"
	"io"
	"os"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is re-exported from core for convenience.
const DefaultBufferSize = core.DefaultBufferSize

// ReaderOption configures a CSV reader.
type ReaderOption func(*csv.Reader)

// WithComma sets the field delimiter (default is ',').
func WithComma(comma rune) ReaderOption {
	return func(r *csv.Reader) {
		r.Comma = comma
	}
}

// WithComment sets the comment character. Lines beginning with this
// character are ignored.
func WithComment(comment rune) ReaderOption {
	return func(r *csv.Reader) {
		r.Comment = comment
	}
}

// WithFieldsPerRecord sets the expected number of fields per record.
// If positive, each record must have exactly that many fields.
// If 0, the number is set to the first record's field count.
// If negative, no check is made and records may have variable fields.
func WithFieldsPerRecord(n int) ReaderOption {
	return func(r *csv.Reader) {
		r.FieldsPerRecord = n
	}
}

// WithLazyQuotes allows lazy quotes in quoted fields.
func WithLazyQuotes(lazy bool) ReaderOption {
	return func(r *csv.Reader) {
		r.LazyQuotes = lazy
	}
}

// WithTrimLeadingSpace trims leading whitespace from fields.
func WithTrimLeadingSpace(trim bool) ReaderOption {
	return func(r *csv.Reader) {
		r.TrimLeadingSpace = trim
	}
}

// ReadRecords creates a Stream that emits each row from a CSV file as a string slice.
// The stream completes after all rows have been read.
func ReadRecords(path string) core.Stream[[]string] {
	return ReadRecordsBuffered(path, DefaultBufferSize)
}

// ReadRecordsBuffered creates a ReadRecords stream with a specified buffer size.
func ReadRecordsBuffered(path string, bufferSize int) core.Stream[[]string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], bufferSize)
		go func() {
			defer close(out)
			file, err := os.Open(path)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]string](err):
				}
				return
			}
			defer file.Close()
			reader := csv.NewReader(file)
			for {
				record, err := reader.Read()
				if err == io.EOF {
					return
				}
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]string](err):
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(record):
				}
			}
		}()
		return out
	})
}

// ReadRecordsFrom creates a Stream that reads CSV records from an io.Reader.
func ReadRecordsFrom(r io.Reader) core.Stream[[]string] {
	return ReadRecordsFromBuffered(r, DefaultBufferSize)
}

// ReadRecordsFromBuffered creates a ReadRecordsFrom stream with a specified buffer size.
func ReadRecordsFromBuffered(r io.Reader, bufferSize int) core.Stream[[]string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], bufferSize)
		go func() {
			defer close(out)
			reader := csv.NewReader(r)
			for {
				record, err := reader.Read()
				if err == io.EOF {
					return
				}
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]string](err):
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(record):
				}
			}
		}()
		return out
	})
}

// ReadRecordsWithOptions creates a Stream with custom CSV reader options.
func ReadRecordsWithOptions(path string, opts ...ReaderOption) core.Stream[[]string] {
	return ReadRecordsWithOptionsBuffered(path, DefaultBufferSize, opts...)
}

// ReadRecordsWithOptionsBuffered creates a ReadRecordsWithOptions stream with buffer size.
func ReadRecordsWithOptionsBuffered(path string, bufferSize int, opts ...ReaderOption) core.Stream[[]string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], bufferSize)
		go func() {
			defer close(out)
			file, err := os.Open(path)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]string](err):
				}
				return
			}
			defer file.Close()
			reader := csv.NewReader(file)
			for _, opt := range opts {
				opt(reader)
			}
			for {
				record, err := reader.Read()
				if err == io.EOF {
					return
				}
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]string](err):
					}
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(record):
				}
			}
		}()
		return out
	})
}

// WriteRecords creates a Transformer that writes CSV records to a file.
// The file is created if it doesn't exist, or truncated if it does.
// Records pass through unchanged after being written.
func WriteRecords(path string) core.Transformer[[]string, []string] {
	return WriteRecordsWithOptions(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

// AppendRecords creates a Transformer that appends CSV records to a file.
func AppendRecords(path string) core.Transformer[[]string, []string] {
	return WriteRecordsWithOptions(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// WriteRecordsWithOptions creates a Transformer with custom file options.
func WriteRecordsWithOptions(path string, flag int, perm os.FileMode) core.Transformer[[]string, []string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[[]string]) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], DefaultBufferSize)
		go func() {
			defer close(out)
			file, err := os.OpenFile(path, flag, perm)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]string](err):
				}
				return
			}
			defer file.Close()
			writer := csv.NewWriter(file)
			defer writer.Flush()
			for res := range in {
				select {
				case <-ctx.Done():
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
				record := res.Value()
				if err := writer.Write(record); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]string](err):
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

// WriteRecordsTo creates a Transformer that writes CSV records to an io.Writer.
func WriteRecordsTo(w io.Writer) core.Transformer[[]string, []string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[[]string]) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], DefaultBufferSize)
		go func() {
			defer close(out)
			writer := csv.NewWriter(w)
			defer writer.Flush()
			for res := range in {
				select {
				case <-ctx.Done():
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
				record := res.Value()
				if err := writer.Write(record); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]string](err):
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

// SkipHeader creates a Transformer that skips the first record (header row).
func SkipHeader() core.Transformer[[]string, []string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[[]string]) <-chan core.Result[[]string] {
		out := make(chan core.Result[[]string], DefaultBufferSize)
		go func() {
			defer close(out)
			first := true
			for res := range in {
				select {
				case <-ctx.Done():
				default:
				}
				if first && res.IsValue() {
					first = false
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
