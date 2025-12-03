package parallel

import (
	"context"
	"fmt"
	"sync"

	"github.com/lguimbarda/min-flow/flow/core"
)

// ParallelConfig provides configuration for parallel transformers.
// It can be registered as a delegate to provide default worker counts
// that can be overridden by function parameters.
type ParallelConfig struct {
	// Workers specifies the default number of worker goroutines.
	// A value of 0 or negative will use the function-level default (1 worker).
	Workers int
}

// Init implements the core.Delegate interface.
func (c *ParallelConfig) Init() error {
	return nil
}

// Close implements the core.Delegate interface.
func (c *ParallelConfig) Close() error {
	return nil
}

// Validate implements the core.Config interface.
// It returns an error if the configuration is invalid.
func (c *ParallelConfig) Validate() error {
	if c.Workers < 0 {
		return fmt.Errorf("parallel config: workers must be >= 0, got %d", c.Workers)
	}
	return nil
}

// WithWorkers returns a functional option that sets the number of workers.
func WithWorkers(n int) func(*ParallelConfig) {
	return func(c *ParallelConfig) {
		c.Workers = n
	}
}

// effectiveWorkers returns the number of workers to use, considering
// context config and the explicitly provided value.
// If n > 0, it takes precedence. Otherwise, config from context is used.
// If neither provides a valid value, returns 1.
func effectiveWorkers(ctx context.Context, n int) int {
	if n > 0 {
		return n
	}
	if cfg, ok := core.GetConfig[*ParallelConfig](ctx); ok && cfg.Workers > 0 {
		return cfg.Workers
	}
	return 1
}

// Map creates a Transformer that processes items concurrently using n workers.
// Each worker applies the given mapper function. Results may arrive out of order.
// If n <= 0, the number of workers is determined from context config or defaults to 1.
func Map[IN, OUT any](n int, mapper func(IN) OUT) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		workers := effectiveWorkers(ctx, n)
		out := make(chan core.Result[OUT])

		go func() {
			defer close(out)

			var wg sync.WaitGroup
			wg.Add(workers)

			for i := 0; i < workers; i++ {
				go func() {
					defer wg.Done()
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
							case out <- core.Err[OUT](res.Error()):
							}
							continue
						}

						if res.IsSentinel() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Sentinel[OUT](res.Error()):
							}
							continue
						}

						// Apply mapper with panic recovery
						result := safeMap(mapper, res.Value())
						select {
						case <-ctx.Done():
							return
						case out <- result:
						}
					}
				}()
			}

			wg.Wait()
		}()

		return out
	})
}

// safeMap applies a mapper function with panic recovery.
func safeMap[IN, OUT any](mapper func(IN) OUT, value IN) (result core.Result[OUT]) {
	defer func() {
		if r := recover(); r != nil {
			result = core.Err[OUT](core.NewPanicError(r))
		}
	}()
	return core.Ok(mapper(value))
}

// ParallelMap is a deprecated alias for Map - use Map instead.
// Deprecated: Use Map instead.
func ParallelMap[IN, OUT any](n int, mapper func(IN) OUT) core.Transformer[IN, OUT] {
	return Map(n, mapper)
}

// FlatMap creates a Transformer that applies a flatMapper concurrently using n workers.
// Each worker can emit zero or more results per input. Results may arrive out of order.
// If n <= 0, the number of workers is determined from context config or defaults to 1.
func FlatMap[IN, OUT any](n int, flatMapper func(IN) []OUT) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		workers := effectiveWorkers(ctx, n)
		out := make(chan core.Result[OUT])

		go func() {
			defer close(out)

			var wg sync.WaitGroup
			wg.Add(workers)

			for i := 0; i < workers; i++ {
				go func() {
					defer wg.Done()
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
							case out <- core.Err[OUT](res.Error()):
							}
							continue
						}

						if res.IsSentinel() {
							select {
							case <-ctx.Done():
								return
							case out <- core.Sentinel[OUT](res.Error()):
							}
							continue
						}

						// Apply flatMapper with panic recovery
						results := safeFlatMap(flatMapper, res.Value())
						for _, r := range results {
							select {
							case <-ctx.Done():
								return
							case out <- r:
							}
						}
					}
				}()
			}

			wg.Wait()
		}()

		return out
	})
}

// safeFlatMap applies a flatMapper function with panic recovery.
func safeFlatMap[IN, OUT any](flatMapper func(IN) []OUT, value IN) (results []core.Result[OUT]) {
	defer func() {
		if r := recover(); r != nil {
			results = []core.Result[OUT]{core.Err[OUT](core.NewPanicError(r))}
		}
	}()

	values := flatMapper(value)
	results = make([]core.Result[OUT], len(values))
	for i, v := range values {
		results[i] = core.Ok(v)
	}
	return results
}

// Ordered creates a Transformer that processes items concurrently but preserves order.
// Uses a sliding window approach: processes up to n items in parallel while maintaining
// input order in the output. More expensive than Map but guarantees ordering.
// If n <= 0, the number of workers is determined from context config or defaults to 1.
func Ordered[IN, OUT any](n int, mapper func(IN) OUT) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		workers := effectiveWorkers(ctx, n)
		out := make(chan core.Result[OUT])

		go func() {
			defer close(out)

			type indexedResult struct {
				index  int
				result core.Result[OUT]
			}

			// Semaphore to limit concurrent workers
			sem := make(chan struct{}, workers)
			resultChan := make(chan indexedResult, workers)

			var wg sync.WaitGroup
			var collectorDone sync.WaitGroup
			collectorDone.Add(1)

			// Collector goroutine - maintains order
			go func() {
				defer collectorDone.Done()
				results := make(map[int]core.Result[OUT])
				nextIndex := 0

				for ir := range resultChan {
					results[ir.index] = ir.result

					// Emit results in order
					for {
						if r, ok := results[nextIndex]; ok {
							delete(results, nextIndex)
							nextIndex++
							select {
							case <-ctx.Done():
								return
							case out <- r:
							}
						} else {
							break
						}
					}
				}

				// Emit any remaining results in order
				for {
					if r, ok := results[nextIndex]; ok {
						delete(results, nextIndex)
						nextIndex++
						select {
						case <-ctx.Done():
							return
						case out <- r:
						}
					} else {
						break
					}
				}
			}()

			index := 0
		inputLoop:
			for res := range in {
				select {
				case <-ctx.Done():
					break inputLoop
				case sem <- struct{}{}:
				}

				wg.Add(1)
				go func(idx int, r core.Result[IN]) {
					defer func() {
						<-sem
						wg.Done()
					}()

					var result core.Result[OUT]
					if r.IsError() {
						result = core.Err[OUT](r.Error())
					} else if r.IsSentinel() {
						result = core.Sentinel[OUT](r.Error())
					} else {
						result = safeMap(mapper, r.Value())
					}

					select {
					case <-ctx.Done():
					case resultChan <- indexedResult{index: idx, result: result}:
					}
				}(index, res)
				index++
			}

			wg.Wait()
			close(resultChan)
			collectorDone.Wait()
		}()

		return out
	})
}

// AsyncMap creates a Transformer that applies an async mapper function to each item.
// The mapper function itself handles its own concurrency (e.g., making HTTP requests).
// Results are emitted as they complete (out of order).
func AsyncMap[IN, OUT any](mapper func(context.Context, IN) (OUT, error)) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])

		go func() {
			defer close(out)

			var wg sync.WaitGroup

			for res := range in {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[OUT](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Sentinel[OUT](res.Error()):
					}
					continue
				}

				wg.Add(1)
				go func(value IN) {
					defer wg.Done()

					result, err := mapper(ctx, value)
					var r core.Result[OUT]
					if err != nil {
						r = core.Err[OUT](err)
					} else {
						r = core.Ok(result)
					}

					select {
					case <-ctx.Done():
					case out <- r:
					}
				}(res.Value())
			}

			wg.Wait()
		}()

		return out
	})
}

// AsyncMapOrdered is like AsyncMap but preserves input order in output.
func AsyncMapOrdered[IN, OUT any](mapper func(context.Context, IN) (OUT, error)) core.Transformer[IN, OUT] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[IN]) <-chan core.Result[OUT] {
		out := make(chan core.Result[OUT])

		go func() {
			defer close(out)

			type indexedResult struct {
				index  int
				result core.Result[OUT]
			}

			resultChan := make(chan indexedResult)
			var wg sync.WaitGroup
			var collectorDone sync.WaitGroup
			collectorDone.Add(1)

			// Collector goroutine
			go func() {
				defer collectorDone.Done()
				results := make(map[int]core.Result[OUT])
				nextIndex := 0

				for ir := range resultChan {
					results[ir.index] = ir.result

					for {
						if r, ok := results[nextIndex]; ok {
							delete(results, nextIndex)
							nextIndex++
							select {
							case <-ctx.Done():
								return
							case out <- r:
							}
						} else {
							break
						}
					}
				}

				for {
					if r, ok := results[nextIndex]; ok {
						delete(results, nextIndex)
						nextIndex++
						select {
						case <-ctx.Done():
							return
						case out <- r:
						}
					} else {
						break
					}
				}
			}()

			index := 0
			for res := range in {
				if res.IsError() {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						select {
						case <-ctx.Done():
						case resultChan <- indexedResult{index: idx, result: core.Err[OUT](res.Error())}:
						}
					}(index)
					index++
					continue
				}

				if res.IsSentinel() {
					wg.Add(1)
					go func(idx int) {
						defer wg.Done()
						select {
						case <-ctx.Done():
						case resultChan <- indexedResult{index: idx, result: core.Sentinel[OUT](res.Error())}:
						}
					}(index)
					index++
					continue
				}

				wg.Add(1)
				go func(idx int, value IN) {
					defer wg.Done()

					result, err := mapper(ctx, value)
					var r core.Result[OUT]
					if err != nil {
						r = core.Err[OUT](err)
					} else {
						r = core.Ok(result)
					}

					select {
					case <-ctx.Done():
					case resultChan <- indexedResult{index: idx, result: r}:
					}
				}(index, res.Value())
				index++
			}

			wg.Wait()
			close(resultChan)
			collectorDone.Wait()
		}()

		return out
	})
}
