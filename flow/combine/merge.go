package combine

import (
	"context"
	"reflect"
	"sync"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Merge combines multiple streams into a single stream.
// Items are emitted as they arrive from any source (interleaved).
// The output stream closes when all input streams are exhausted.
func Merge[T any](streams ...core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			if len(streams) == 0 {
				return
			}

			var wg sync.WaitGroup
			wg.Add(len(streams))

			for _, stream := range streams {
				go func(s core.Stream[T]) {
					defer wg.Done()
					for res := range s.Emit(ctx) {
						select {
						case <-ctx.Done():
							return
						case out <- res:
						}
					}
				}(stream)
			}

			wg.Wait()
		}()

		return out
	})
}

// Concat concatenates multiple streams sequentially.
// Items from the second stream only start emitting after the first completes, etc.
// The output stream closes when all input streams are exhausted.
func Concat[T any](streams ...core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for _, stream := range streams {
				for res := range stream.Emit(ctx) {
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

// Zip combines items from two streams pairwise.
// Emits pairs until either stream is exhausted. Extra items from the longer stream are dropped.
func Zip[A, B any](streamA core.Stream[A], streamB core.Stream[B]) core.Stream[struct {
	A A
	B B
}] {
	return ZipWith(streamA, streamB, func(a A, b B) struct {
		A A
		B B
	} {
		return struct {
			A A
			B B
		}{A: a, B: b}
	})
}

// ZipWith combines items from two streams using a combiner function.
// Emits combined results until either stream is exhausted.
func ZipWith[A, B, C any](streamA core.Stream[A], streamB core.Stream[B], combiner func(A, B) C) core.Stream[C] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[C] {
		out := make(chan core.Result[C])

		go func() {
			defer close(out)

			chanA := streamA.Emit(ctx)
			chanB := streamB.Emit(ctx)

			for {
				// Get next from A
				resA, okA := <-chanA
				if !okA {
					return
				}

				// Get next from B
				resB, okB := <-chanB
				if !okB {
					return
				}

				// Handle errors
				if resA.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[C](resA.Error()):
					}
					continue
				}

				if resB.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[C](resB.Error()):
					}
					continue
				}

				// Handle sentinels
				if resA.IsSentinel() || resB.IsSentinel() {
					// If either is a sentinel, we're done
					return
				}

				// Combine values
				combined := combiner(resA.Value(), resB.Value())
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(combined):
				}
			}
		}()

		return out
	})
}

// ZipLongest combines items from two streams, using zero values for shorter stream.
// Continues until both streams are exhausted.
func ZipLongest[A, B any](streamA core.Stream[A], streamB core.Stream[B]) core.Stream[struct {
	A    A
	B    B
	HasA bool
	HasB bool
}] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[struct {
		A    A
		B    B
		HasA bool
		HasB bool
	}] {
		out := make(chan core.Result[struct {
			A    A
			B    B
			HasA bool
			HasB bool
		}])

		go func() {
			defer close(out)

			chanA := streamA.Emit(ctx)
			chanB := streamB.Emit(ctx)
			doneA := false
			doneB := false

			for !doneA || !doneB {
				var a A
				var b B
				hasA := false
				hasB := false

				if !doneA {
					resA, okA := <-chanA
					if !okA {
						doneA = true
					} else if resA.IsValue() {
						a = resA.Value()
						hasA = true
					} else if resA.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[struct {
							A    A
							B    B
							HasA bool
							HasB bool
						}](resA.Error()):
						}
						continue
					}
				}

				if !doneB {
					resB, okB := <-chanB
					if !okB {
						doneB = true
					} else if resB.IsValue() {
						b = resB.Value()
						hasB = true
					} else if resB.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[struct {
							A    A
							B    B
							HasA bool
							HasB bool
						}](resB.Error()):
						}
						continue
					}
				}

				// Only emit if we have at least one value
				if hasA || hasB {
					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(struct {
						A    A
						B    B
						HasA bool
						HasB bool
					}{A: a, B: b, HasA: hasA, HasB: hasB}):
					}
				}
			}
		}()

		return out
	})
}

// Interleave alternates items from multiple streams in round-robin fashion.
// Continues until all streams are exhausted.
func Interleave[T any](streams ...core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			if len(streams) == 0 {
				return
			}

			channels := make([]<-chan core.Result[T], len(streams))
			for i, s := range streams {
				channels[i] = s.Emit(ctx)
			}

			active := make([]bool, len(channels))
			for i := range active {
				active[i] = true
			}

			activeCount := len(channels)
			idx := 0

			for activeCount > 0 {
				// Find next active channel
				for !active[idx] {
					idx = (idx + 1) % len(channels)
				}

				res, ok := <-channels[idx]
				if !ok {
					active[idx] = false
					activeCount--
					idx = (idx + 1) % len(channels)
					continue
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}

				idx = (idx + 1) % len(channels)
			}
		}()

		return out
	})
}

// CombineLatest emits a slice containing the latest value from each stream
// whenever any stream emits a new value. Only starts emitting after all
// streams have emitted at least one value.
func CombineLatest[T any](streams ...core.Stream[T]) core.Stream[[]T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])

		go func() {
			defer close(out)

			if len(streams) == 0 {
				return
			}

			latest := make([]T, len(streams))
			hasValue := make([]bool, len(streams))
			allHaveValue := false

			var mu sync.Mutex
			var wg sync.WaitGroup
			wg.Add(len(streams))

			// Channel to signal new values
			updates := make(chan struct{}, len(streams))

			for i, stream := range streams {
				go func(idx int, s core.Stream[T]) {
					defer wg.Done()
					for res := range s.Emit(ctx) {
						if res.IsValue() {
							shouldSignal := false
							mu.Lock()
							latest[idx] = res.Value()
							hasValue[idx] = true
							if !allHaveValue {
								allHaveValue = true
								for _, hv := range hasValue {
									if !hv {
										allHaveValue = false
										break
									}
								}
							}
							shouldSignal = allHaveValue
							mu.Unlock()

							if shouldSignal {
								select {
								case updates <- struct{}{}:
								default:
								}
							}
						}
					}
				}(i, stream)
			}

			// Close updates when all streams done
			go func() {
				wg.Wait()
				close(updates)
			}()

			for range updates {
				mu.Lock()
				if allHaveValue {
					snapshot := make([]T, len(latest))
					copy(snapshot, latest)
					mu.Unlock()

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(snapshot):
					}
				} else {
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// WithLatestFrom combines a source stream with the latest values from other streams.
// Each time the source emits, it combines with the most recent values from all other streams.
// Only emits when the source emits AND all other streams have emitted at least once.
func WithLatestFrom[T, U any](source core.Stream[T], other core.Stream[U]) core.Stream[struct {
	Source T
	Other  U
}] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[struct {
		Source T
		Other  U
	}] {
		out := make(chan core.Result[struct {
			Source T
			Other  U
		}])

		go func() {
			defer close(out)

			var latestOther U
			hasOther := false
			var mu sync.Mutex

			// Start consuming 'other' in background
			otherCh := other.Emit(ctx)
			go func() {
				for res := range otherCh {
					if res.IsValue() {
						mu.Lock()
						latestOther = res.Value()
						hasOther = true
						mu.Unlock()
					}
				}
			}()

			// Consume source and combine with latest other
			for res := range source.Emit(ctx) {
				if res.IsError() {
					select {
					case <-ctx.Done():
						return
					case out <- core.Err[struct {
						Source T
						Other  U
					}](res.Error()):
					}
					continue
				}

				if res.IsSentinel() {
					continue
				}

				mu.Lock()
				if hasOther {
					combined := struct {
						Source T
						Other  U
					}{Source: res.Value(), Other: latestOther}
					mu.Unlock()

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(combined):
					}
				} else {
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// Race returns a stream that emits from whichever input stream emits first,
// then cancels the other streams.
func Race[T any](streams ...core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			if len(streams) == 0 {
				return
			}

			n := len(streams)

			// Create cancellation contexts for each stream
			cancels := make([]context.CancelFunc, n)
			channels := make([]<-chan core.Result[T], n)

			for i, stream := range streams {
				streamCtx, cancel := context.WithCancel(ctx)
				cancels[i] = cancel
				channels[i] = stream.Emit(streamCtx)
			}

			// Wait for any stream to emit first
			winnerIdx := -1
			var winnerCh <-chan core.Result[T]

			// Use reflect.Select to wait on all channels
			cases := make([]reflect.SelectCase, n+1)
			for i, ch := range channels {
				cases[i] = reflect.SelectCase{
					Dir:  reflect.SelectRecv,
					Chan: reflect.ValueOf(ch),
				}
			}
			cases[n] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(ctx.Done()),
			}

			chosen, recv, ok := reflect.Select(cases)
			if chosen == n || !ok {
				// Context cancelled or channel closed
				for _, cancel := range cancels {
					cancel()
				}
				return
			}

			winnerIdx = chosen
			winnerCh = channels[winnerIdx]

			// Cancel all other streams
			for i, cancel := range cancels {
				if i != winnerIdx {
					cancel()
				}
			}

			// Emit the first result
			result := recv.Interface().(core.Result[T])
			select {
			case <-ctx.Done():
				cancels[winnerIdx]()
				return
			case out <- result:
			}

			// Continue emitting from the winner
			for res := range winnerCh {
				select {
				case <-ctx.Done():
					cancels[winnerIdx]()
					return
				case out <- res:
				}
			}

			cancels[winnerIdx]()
		}()

		return out
	})
}

// Amb is an alias for Race - returns the stream that emits first.
func Amb[T any](streams ...core.Stream[T]) core.Stream[T] {
	return Race(streams...)
}

// SampleOn samples the source stream whenever the notifier stream emits.
// Each time the notifier emits, the most recent value from source is emitted.
// Note: This is different from Sample operator which samples by time.
func SampleOn[T, N any](source core.Stream[T], notifier core.Stream[N]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var latest T
			hasValue := false
			var mu sync.Mutex

			// Consume source in background
			sourceCh := source.Emit(ctx)
			sourceDone := make(chan struct{})
			go func() {
				defer close(sourceDone)
				for res := range sourceCh {
					if res.IsValue() {
						mu.Lock()
						latest = res.Value()
						hasValue = true
						mu.Unlock()
					}
				}
			}()

			// Sample on notifier emissions
			for range notifier.Emit(ctx) {
				mu.Lock()
				if hasValue {
					val := latest
					hasValue = false // Reset after sampling
					mu.Unlock()

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(val):
					}
				} else {
					mu.Unlock()
				}
			}
		}()

		return out
	})
}

// BufferWith collects items into groups using a notifier stream.
// Each time the notifier emits, a batch of collected items is emitted.
// Note: This is different from Buffer operator which buffers by count.
func BufferWith[T, N any](source core.Stream[T], notifier core.Stream[N]) core.Stream[[]T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])

		go func() {
			defer close(out)

			var buffer []T
			var mu sync.Mutex

			// Consume source in background
			sourceCh := source.Emit(ctx)
			sourceDone := make(chan struct{})
			go func() {
				defer close(sourceDone)
				for res := range sourceCh {
					if res.IsValue() {
						mu.Lock()
						buffer = append(buffer, res.Value())
						mu.Unlock()
					}
				}
			}()

			// Emit buffer on notifier emissions
			for range notifier.Emit(ctx) {
				mu.Lock()
				if len(buffer) > 0 {
					batch := buffer
					buffer = nil
					mu.Unlock()

					select {
					case <-ctx.Done():
						return
					case out <- core.Ok(batch):
					}
				} else {
					mu.Unlock()
				}
			}

			// Emit remaining buffer
			mu.Lock()
			if len(buffer) > 0 {
				batch := buffer
				mu.Unlock()
				select {
				case <-ctx.Done():
				case out <- core.Ok(batch):
				}
			} else {
				mu.Unlock()
			}
		}()

		return out
	})
}

// Fork applies multiple transformers to the same stream in parallel,
// returning a slice of result streams.
func Fork[IN, OUT any](stream core.Stream[IN], transformers ...core.Transformer[IN, OUT]) []core.Stream[OUT] {
	n := len(transformers)
	if n == 0 {
		return nil
	}

	// Fan out the input stream
	inputs := FanOut(n, stream)

	// Apply each transformer to its corresponding input
	outputs := make([]core.Stream[OUT], n)
	for i, t := range transformers {
		outputs[i] = t.Apply(context.Background(), inputs[i])
	}

	return outputs
}

// Gather applies multiple transformers to the same stream and merges results.
func Gather[IN, OUT any](stream core.Stream[IN], transformers ...core.Transformer[IN, OUT]) core.Stream[OUT] {
	forked := Fork(stream, transformers...)
	return Merge(forked...)
}

// Switch switches between streams based on a selector stream.
// When the selector emits, the corresponding indexed stream becomes active.
func Switch[T any](selector core.Stream[int], streams ...core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			if len(streams) == 0 {
				return
			}

			var activeCh <-chan core.Result[T]
			var activeCancel context.CancelFunc
			var mu sync.Mutex

			// Listen for selector changes
			go func() {
				for res := range selector.Emit(ctx) {
					if res.IsValue() {
						idx := res.Value()
						if idx >= 0 && idx < len(streams) {
							mu.Lock()
							// Cancel current stream
							if activeCancel != nil {
								activeCancel()
							}
							// Start new stream
							streamCtx, cancel := context.WithCancel(ctx)
							activeCancel = cancel
							activeCh = streams[idx].Emit(streamCtx)
							mu.Unlock()
						}
					}
				}
			}()

			// Forward from active stream
			for {
				mu.Lock()
				ch := activeCh
				mu.Unlock()

				if ch == nil {
					select {
					case <-ctx.Done():
						return
					default:
						continue
					}
				}

				select {
				case <-ctx.Done():
					if activeCancel != nil {
						activeCancel()
					}
					return
				case res, ok := <-ch:
					if !ok {
						mu.Lock()
						activeCh = nil
						mu.Unlock()
						continue
					}
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

// SequenceEqual compares two streams for equality and returns a single boolean result.
func SequenceEqualStream[T comparable](streamA, streamB core.Stream[T]) core.Stream[bool] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[bool] {
		out := make(chan core.Result[bool])

		go func() {
			defer close(out)

			chanA := streamA.Emit(ctx)
			chanB := streamB.Emit(ctx)

			for {
				resA, okA := <-chanA
				resB, okB := <-chanB

				// Both exhausted - equal
				if !okA && !okB {
					select {
					case <-ctx.Done():
					case out <- core.Ok(true):
					}
					return
				}

				// One exhausted, other not - not equal
				if okA != okB {
					select {
					case <-ctx.Done():
					case out <- core.Ok(false):
					}
					return
				}

				// Handle errors
				if resA.IsError() || resB.IsError() {
					select {
					case <-ctx.Done():
					case out <- core.Ok(false):
					}
					return
				}

				// Compare values
				if resA.Value() != resB.Value() {
					select {
					case <-ctx.Done():
					case out <- core.Ok(false):
					}
					return
				}
			}
		}()

		return out
	})
}

// IfEmpty returns a stream that switches to an alternative if the source is empty.
func IfEmpty[T any](source core.Stream[T], alternative core.Stream[T]) core.Stream[T] {
	return core.Emit(func(ctx context.Context) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			emitted := false
			for res := range source.Emit(ctx) {
				emitted = true
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}

			if !emitted {
				for res := range alternative.Emit(ctx) {
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
