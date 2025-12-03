// Package json provides stream adapters for JSON encoding and decoding.
// It enables parsing JSON data as part of flow pipelines.
package json

import (
	"context"
	"encoding/json"
	"io"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is the default buffer size for JSON operations.
const DefaultBufferSize = 64

// Decode creates a Transformer that decodes JSON strings into typed values.
// Each input string is expected to be a valid JSON document.
// Invalid JSON results in an error Result that passes through the stream.
func Decode[T any]() core.Transformer[string, T] {
	return DecodeBuffered[T](DefaultBufferSize)
}

// DecodeBuffered creates a Decode transformer with a specified buffer size.
func DecodeBuffered[T any](bufferSize int) core.Transformer[string, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[T] {
		out := make(chan core.Result[T], bufferSize)

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
					case out <- core.Err[T](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[T](res.Error()):
					}
					continue
				}

				var value T
				if err := json.Unmarshal([]byte(res.Value()), &value); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}
		}()

		return out
	})
}

// DecodeBytes creates a Transformer that decodes JSON byte slices into typed values.
func DecodeBytes[T any]() core.Transformer[[]byte, T] {
	return DecodeBytesBuffered[T](DefaultBufferSize)
}

// DecodeBytesBuffered creates a DecodeBytes transformer with a specified buffer size.
func DecodeBytesBuffered[T any](bufferSize int) core.Transformer[[]byte, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[[]byte]) <-chan core.Result[T] {
		out := make(chan core.Result[T], bufferSize)

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
					case out <- core.Err[T](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[T](res.Error()):
					}
					continue
				}

				var value T
				if err := json.Unmarshal(res.Value(), &value); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}
		}()

		return out
	})
}

// Encode creates a Transformer that encodes typed values into JSON strings.
func Encode[T any]() core.Transformer[T, string] {
	return EncodeBuffered[T](DefaultBufferSize)
}

// EncodeBuffered creates an Encode transformer with a specified buffer size.
func EncodeBuffered[T any](bufferSize int) core.Transformer[T, string] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[string] {
		out := make(chan core.Result[string], bufferSize)

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
					case out <- core.Err[string](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[string](res.Error()):
					}
					continue
				}

				data, err := json.Marshal(res.Value())
				if err != nil {
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
				case out <- core.Ok(string(data)):
				}
			}
		}()

		return out
	})
}

// EncodeBytes creates a Transformer that encodes typed values into JSON byte slices.
func EncodeBytes[T any]() core.Transformer[T, []byte] {
	return EncodeBytesBuffered[T](DefaultBufferSize)
}

// EncodeBytesBuffered creates an EncodeBytes transformer with a specified buffer size.
func EncodeBytesBuffered[T any](bufferSize int) core.Transformer[T, []byte] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], bufferSize)

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
					case out <- core.Err[[]byte](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[[]byte](res.Error()):
					}
					continue
				}

				data, err := json.Marshal(res.Value())
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[[]byte](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(data):
				}
			}
		}()

		return out
	})
}

// DecodeStream creates a Stream that reads and decodes JSON objects from a reader.
// The reader should contain a stream of newline-delimited JSON (NDJSON/JSON Lines).
func DecodeStream[T any](r io.Reader) core.Stream[T] {
	return DecodeStreamBuffered[T](r, DefaultBufferSize)
}

// DecodeStreamBuffered creates a DecodeStream with a specified buffer size.
func DecodeStreamBuffered[T any](r io.Reader, bufferSize int) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], bufferSize)

		go func() {
			defer close(out)

			decoder := json.NewDecoder(r)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var value T
				if err := decoder.Decode(&value); err != nil {
					if err == io.EOF {
						return
					}
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}
		}()

		return out
	})
}

// DecodeArray creates a Stream that reads a JSON array from a reader and emits each element.
func DecodeArray[T any](r io.Reader) core.Stream[T] {
	return DecodeArrayBuffered[T](r, DefaultBufferSize)
}

// DecodeArrayBuffered creates a DecodeArray stream with a specified buffer size.
func DecodeArrayBuffered[T any](r io.Reader, bufferSize int) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T], bufferSize)

		go func() {
			defer close(out)

			decoder := json.NewDecoder(r)

			token, err := decoder.Token()
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](err):
				}
				return
			}

			delim, ok := token.(json.Delim)
			if !ok || delim != '[' {
				select {
				case <-ctx.Done():
				case out <- core.Err[T](json.Unmarshal([]byte("expected array"), nil)):
				}
				return
			}

			for decoder.More() {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var value T
				if err := decoder.Decode(&value); err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[T](err):
					}
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(value):
				}
			}

			_, _ = decoder.Token()
		}()

		return out
	})
}
