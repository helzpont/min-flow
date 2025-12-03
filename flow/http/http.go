// Package http provides stream adapters for HTTP operations.
// It enables making HTTP requests as part of flow pipelines.
package http

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DefaultBufferSize is the default buffer size for HTTP operations.
const DefaultBufferSize = 64

// Response contains HTTP response data.
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Get creates a Stream that makes a GET request and emits the response.
func Get(url string) core.Stream[Response] {
	return Request(http.DefaultClient, http.MethodGet, url, nil)
}

// GetWithClient creates a Stream that makes a GET request with a custom client.
func GetWithClient(client *http.Client, url string) core.Stream[Response] {
	return Request(client, http.MethodGet, url, nil)
}

// Request creates a Stream that makes an HTTP request and emits the response.
func Request(client *http.Client, method, url string, body io.Reader) core.Stream[Response] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[Response] {
		out := make(chan core.Result[Response], 1)
		go func() {
			defer close(out)
			req, err := http.NewRequestWithContext(ctx, method, url, body)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			defer resp.Body.Close()
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			response := Response{
				StatusCode: resp.StatusCode,
				Headers:    resp.Header,
				Body:       respBody,
			}
			select {
			case <-ctx.Done():
			case out <- core.Ok(response):
			}
		}()
		return out
	})
}

// GetLines creates a Stream that makes a GET request and emits response lines.
func GetLines(url string) core.Stream[string] {
	return GetLinesWithClient(http.DefaultClient, url)
}

// GetLinesWithClient creates a GetLines stream with a custom client.
func GetLinesWithClient(client *http.Client, url string) core.Stream[string] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[string] {
		out := make(chan core.Result[string], DefaultBufferSize)
		go func() {
			defer close(out)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[string](err):
				}
				return
			}
			defer resp.Body.Close()
			scanner := bufio.NewScanner(resp.Body)
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

// GetBytes creates a Stream that makes a GET request and emits response bytes in chunks.
func GetBytes(url string, chunkSize int) core.Stream[[]byte] {
	return GetBytesWithClient(http.DefaultClient, url, chunkSize)
}

// GetBytesWithClient creates a GetBytes stream with a custom client.
func GetBytesWithClient(client *http.Client, url string, chunkSize int) core.Stream[[]byte] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]byte] {
		out := make(chan core.Result[[]byte], DefaultBufferSize)
		go func() {
			defer close(out)
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]byte](err):
				}
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[[]byte](err):
				}
				return
			}
			defer resp.Body.Close()
			buf := make([]byte, chunkSize)
			for {
				n, err := resp.Body.Read(buf)
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

// GetEach creates a Transformer that makes a GET request for each URL.
func GetEach(client *http.Client) core.Transformer[string, Response] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[string]) <-chan core.Result[Response] {
		out := make(chan core.Result[Response], DefaultBufferSize)
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
					case out <- core.Err[Response](res.Error()):
					}
					continue
				}
				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[Response](res.Error()):
					}
					continue
				}
				url := res.Value()
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[Response](err):
					}
					continue
				}
				resp, err := client.Do(req)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[Response](err):
					}
					continue
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[Response](err):
					}
					continue
				}
				response := Response{
					StatusCode: resp.StatusCode,
					Headers:    resp.Header,
					Body:       body,
				}
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(response):
				}
			}
		}()
		return out
	})
}

// Post creates a Stream that makes a POST request.
func Post(url, contentType string, body io.Reader) core.Stream[Response] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[Response] {
		out := make(chan core.Result[Response], 1)
		go func() {
			defer close(out)
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			req.Header.Set("Content-Type", contentType)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			defer resp.Body.Close()
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				select {
				case <-ctx.Done():
				case out <- core.Err[Response](err):
				}
				return
			}
			response := Response{
				StatusCode: resp.StatusCode,
				Headers:    resp.Header,
				Body:       respBody,
			}
			select {
			case <-ctx.Done():
			case out <- core.Ok(response):
			}
		}()
		return out
	})
}

// PostJSON creates a Stream that posts JSON data.
func PostJSON(url string, jsonBody string) core.Stream[Response] {
	return Post(url, "application/json", strings.NewReader(jsonBody))
}
