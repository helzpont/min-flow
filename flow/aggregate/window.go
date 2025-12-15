// Package operators provides stream transformation operators for the min-flow framework.
// This file contains windowing operators for grouping stream items by time.
// Note: Count-based windowing (Window with size/step) is in batch.go.
package aggregate

import (
	"context"
	"sync"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// WindowTime creates a Transformer that groups items into time-based windows.
// Each window collects items received during the specified duration,
// then emits them as a slice when the window closes.
func WindowTime[T any](d time.Duration) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			ticker := time.NewTicker(d)
			defer ticker.Stop()

			var window []T

			for {
				select {
				case <-ctx.Done():
					// Emit any remaining items before closing
					if len(window) > 0 {
						select {
						case out <- core.Ok(window):
						default:
						}
					}
					return

				case <-ticker.C:
					// Window closed - emit collected items
					if len(window) > 0 {
						windowCopy := make([]T, len(window))
						copy(windowCopy, window)
						select {
						case <-ctx.Done():
							return
						case out <- core.Ok(windowCopy):
						}
					}
					window = nil

				case res, ok := <-in:
					if !ok {
						// Input closed - emit remaining items
						if len(window) > 0 {
							select {
							case <-ctx.Done():
							case out <- core.Ok(window):
							}
						}
						return
					}

					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					window = append(window, res.Value())
				}
			}
		}()
		return out
	})
}

// TumblingWindow creates a Transformer that groups items into non-overlapping
// time-based windows. This is an alias for WindowTime with clearer semantics.
func TumblingWindow[T any](d time.Duration) core.Transformer[T, []T] {
	return WindowTime[T](d)
}

// SessionWindow creates a Transformer that groups items into session-based windows.
// A session window collects items until there's a gap of the specified duration
// with no items. When the gap is detected, the window is emitted and a new session begins.
func SessionWindow[T any](timeout time.Duration) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			var window []T
			var timer *time.Timer
			var timerC <-chan time.Time

			emitWindow := func() {
				if len(window) > 0 {
					windowCopy := make([]T, len(window))
					copy(windowCopy, window)
					select {
					case <-ctx.Done():
					case out <- core.Ok(windowCopy):
					}
					window = nil
				}
			}

			for {
				select {
				case <-ctx.Done():
					if timer != nil {
						timer.Stop()
					}
					emitWindow()
					return

				case <-timerC:
					// Session timeout - emit window
					emitWindow()
					timer = nil
					timerC = nil

				case res, ok := <-in:
					if !ok {
						if timer != nil {
							timer.Stop()
						}
						emitWindow()
						return
					}

					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					// Reset session timer
					if timer != nil {
						timer.Stop()
					}
					timer = time.NewTimer(timeout)
					timerC = timer.C

					window = append(window, res.Value())
				}
			}
		}()
		return out
	})
}

// WindowWithBoundary creates a Transformer that groups items based on a boundary stream.
// Each time the boundary stream emits, the current window is closed and emitted.
func WindowWithBoundary[T, B any](boundary core.Stream[B]) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			boundaryCh := boundary.Emit(ctx)
			var window []T
			var mu sync.Mutex

			emitWindow := func() {
				mu.Lock()
				if len(window) > 0 {
					windowCopy := make([]T, len(window))
					copy(windowCopy, window)
					window = nil
					mu.Unlock()
					select {
					case <-ctx.Done():
					case out <- core.Ok(windowCopy):
					}
				} else {
					mu.Unlock()
				}
			}

			for {
				select {
				case <-ctx.Done():
					emitWindow()
					return

				case _, ok := <-boundaryCh:
					if !ok {
						// Boundary closed - emit remaining and close
						emitWindow()
						return
					}
					emitWindow()

				case res, ok := <-in:
					if !ok {
						emitWindow()
						return
					}

					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					mu.Lock()
					window = append(window, res.Value())
					mu.Unlock()
				}
			}
		}()
		return out
	})
}

// GroupByTime creates a Transformer that groups items by a time key derived from each item.
// Items with the same time key (truncated to the specified duration) are grouped together.
// The stream must be ordered by time for correct grouping.
func GroupByTime[T any](keyFn func(T) time.Time, d time.Duration) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			var window []T
			var currentKey time.Time
			first := true

			emitWindow := func() {
				if len(window) > 0 {
					windowCopy := make([]T, len(window))
					copy(windowCopy, window)
					select {
					case <-ctx.Done():
					case out <- core.Ok(windowCopy):
					}
					window = nil
				}
			}

			for {
				select {
				case <-ctx.Done():
					emitWindow()
					return

				case res, ok := <-in:
					if !ok {
						emitWindow()
						return
					}

					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					itemTime := keyFn(res.Value())
					itemKey := itemTime.Truncate(d)

					if first {
						currentKey = itemKey
						first = false
					} else if !itemKey.Equal(currentKey) {
						// New time bucket - emit current window
						emitWindow()
						currentKey = itemKey
					}

					window = append(window, res.Value())
				}
			}
		}()
		return out
	})
}

// HoppingWindow creates a Transformer that emits time-based windows that overlap.
// Windows of the specified size are emitted every hop interval.
// If hop < size, windows overlap. If hop == size, this is equivalent to TumblingWindow.
func HoppingWindow[T any](size, hop time.Duration) core.Transformer[T, []T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[[]T] {
		out := make(chan core.Result[[]T])
		go func() {
			defer close(out)

			type timestampedItem struct {
				value T
				ts    time.Time
			}

			var buffer []timestampedItem
			var mu sync.Mutex

			hopTicker := time.NewTicker(hop)
			defer hopTicker.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-hopTicker.C:
					mu.Lock()
					now := time.Now()
					cutoff := now.Add(-size)

					// Collect items within the window
					var window []T
					for _, item := range buffer {
						if item.ts.After(cutoff) || item.ts.Equal(cutoff) {
							window = append(window, item.value)
						}
					}

					// Remove items older than the window
					var newBuffer []timestampedItem
					for _, item := range buffer {
						if item.ts.After(cutoff) {
							newBuffer = append(newBuffer, item)
						}
					}
					buffer = newBuffer
					mu.Unlock()

					if len(window) > 0 {
						select {
						case <-ctx.Done():
							return
						case out <- core.Ok(window):
						}
					}

				case res, ok := <-in:
					if !ok {
						return
					}

					if res.IsError() {
						select {
						case <-ctx.Done():
							return
						case out <- core.Err[[]T](res.Error()):
						}
						continue
					}
					if res.IsSentinel() {
						continue
					}

					mu.Lock()
					buffer = append(buffer, timestampedItem{
						value: res.Value(),
						ts:    time.Now(),
					})
					mu.Unlock()
				}
			}
		}()
		return out
	})
}
