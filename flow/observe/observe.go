// Package operators provides stream transformation operators for the min-flow framework.
// This file contains observability operators for monitoring, metrics, and debugging streams.
package observe

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// StreamMetrics holds statistics about a stream's execution.
type StreamMetrics struct {
	// Counts
	TotalItems    int64
	ValueCount    int64
	ErrorCount    int64
	SentinelCount int64

	// Timing
	StartTime     time.Time
	EndTime       time.Time
	FirstItemTime time.Time
	LastItemTime  time.Time

	// Throughput
	ItemsPerSecond float64

	// Latency (time between items)
	MinLatency time.Duration
	MaxLatency time.Duration
	AvgLatency time.Duration
}

// Meter creates a Transformer that collects metrics about the stream.
// The onComplete callback is called with the final metrics when the stream completes.
func Meter[T any](onComplete func(StreamMetrics)) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			metrics := StreamMetrics{
				StartTime:  time.Now(),
				MinLatency: time.Duration(1<<63 - 1), // Max duration
			}

			var lastItemTime time.Time
			var totalLatency time.Duration
			var latencyCount int64

			defer func() {
				metrics.EndTime = time.Now()
				if metrics.TotalItems > 0 {
					duration := metrics.EndTime.Sub(metrics.StartTime).Seconds()
					if duration > 0 {
						metrics.ItemsPerSecond = float64(metrics.TotalItems) / duration
					}
					if latencyCount > 0 {
						metrics.AvgLatency = totalLatency / time.Duration(latencyCount)
					}
				}
				if onComplete != nil {
					onComplete(metrics)
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					now := time.Now()
					metrics.TotalItems++

					// Track first/last item times
					if metrics.TotalItems == 1 {
						metrics.FirstItemTime = now
					}
					metrics.LastItemTime = now

					// Calculate latency
					if !lastItemTime.IsZero() {
						latency := now.Sub(lastItemTime)
						if latency < metrics.MinLatency {
							metrics.MinLatency = latency
						}
						if latency > metrics.MaxLatency {
							metrics.MaxLatency = latency
						}
						totalLatency += latency
						latencyCount++
					}
					lastItemTime = now

					// Categorize result
					if res.IsError() {
						metrics.ErrorCount++
					} else if res.IsSentinel() {
						metrics.SentinelCount++
					} else {
						metrics.ValueCount++
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

// LiveMetrics holds real-time metrics that can be read concurrently.
type LiveMetrics struct {
	totalItems    atomic.Int64
	valueCount    atomic.Int64
	errorCount    atomic.Int64
	sentinelCount atomic.Int64
	startTime     atomic.Int64 // Unix nano
	lastItemTime  atomic.Int64 // Unix nano
}

// TotalItems returns the total number of items processed.
func (m *LiveMetrics) TotalItems() int64 { return m.totalItems.Load() }

// ValueCount returns the number of successful values.
func (m *LiveMetrics) ValueCount() int64 { return m.valueCount.Load() }

// ErrorCount returns the number of errors.
func (m *LiveMetrics) ErrorCount() int64 { return m.errorCount.Load() }

// SentinelCount returns the number of sentinels.
func (m *LiveMetrics) SentinelCount() int64 { return m.sentinelCount.Load() }

// StartTime returns when the stream started.
func (m *LiveMetrics) StartTime() time.Time {
	return time.Unix(0, m.startTime.Load())
}

// LastItemTime returns when the last item was processed.
func (m *LiveMetrics) LastItemTime() time.Time {
	return time.Unix(0, m.lastItemTime.Load())
}

// Duration returns how long the stream has been running.
func (m *LiveMetrics) Duration() time.Duration {
	start := m.startTime.Load()
	if start == 0 {
		return 0
	}
	return time.Since(time.Unix(0, start))
}

// ItemsPerSecond returns the current throughput.
func (m *LiveMetrics) ItemsPerSecond() float64 {
	duration := m.Duration().Seconds()
	if duration <= 0 {
		return 0
	}
	return float64(m.TotalItems()) / duration
}

// MeterLive creates a Transformer that updates live metrics that can be
// read concurrently while the stream is running.
func MeterLive[T any](metrics *LiveMetrics) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			metrics.startTime.Store(time.Now().UnixNano())

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					metrics.totalItems.Add(1)
					metrics.lastItemTime.Store(time.Now().UnixNano())

					if res.IsError() {
						metrics.errorCount.Add(1)
					} else if res.IsSentinel() {
						metrics.sentinelCount.Add(1)
					} else {
						metrics.valueCount.Add(1)
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

// ProgressReport holds information for progress reporting.
type ProgressReport struct {
	Processed int64
	Total     int64 // -1 if unknown
	Percent   float64
	Elapsed   time.Duration
	Remaining time.Duration // Estimated, -1 if unknown
}

// Progress creates a Transformer that reports progress.
// If total is known, pass it; otherwise pass -1.
// The onProgress callback is called periodically or on each item based on interval.
func Progress[T any](total int64, interval time.Duration, onProgress func(ProgressReport)) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			var processed int64
			startTime := time.Now()
			lastReport := startTime

			report := func() {
				elapsed := time.Since(startTime)
				r := ProgressReport{
					Processed: processed,
					Total:     total,
					Elapsed:   elapsed,
					Remaining: -1,
				}

				if total > 0 {
					r.Percent = float64(processed) / float64(total) * 100
					if processed > 0 {
						rate := float64(processed) / elapsed.Seconds()
						remaining := float64(total-processed) / rate
						r.Remaining = time.Duration(remaining * float64(time.Second))
					}
				}

				if onProgress != nil {
					onProgress(r)
				}
			}

			for {
				select {
				case <-ctx.Done():
					report() // Final report
					return
				case res, ok := <-in:
					if !ok {
						report() // Final report
						return
					}

					if res.IsValue() {
						processed++
					}

					// Report based on interval
					if interval > 0 && time.Since(lastReport) >= interval {
						report()
						lastReport = time.Now()
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

// Spy creates a Transformer that allows inspection of all items without modification.
// Unlike Tap (which only sees values), Spy sees the full Result including errors.
func Spy[T any](inspector func(*core.Result[T])) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					if inspector != nil {
						inspector(res)
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

// DebugEvent represents different events in a stream's lifecycle.
type DebugEvent int

const (
	DebugEventSubscribe DebugEvent = iota
	DebugEventValue
	DebugEventError
	DebugEventSentinel
	DebugEventComplete
	DebugEventCancel
)

// DebugInfo contains information about a debug event.
type DebugInfo[T any] struct {
	Event     DebugEvent
	Value     T
	Error     error
	Timestamp time.Time
	Index     int64
}

// Debug creates a Transformer that provides detailed debugging information
// for each event in the stream's lifecycle.
func Debug[T any](handler func(DebugInfo[T])) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			var index int64
			emit := func(event DebugEvent, val T, err error) {
				if handler != nil {
					handler(DebugInfo[T]{
						Event:     event,
						Value:     val,
						Error:     err,
						Timestamp: time.Now(),
						Index:     index,
					})
				}
			}

			// Subscribe event
			emit(DebugEventSubscribe, *new(T), nil)

			defer func() {
				select {
				case <-ctx.Done():
					emit(DebugEventCancel, *new(T), ctx.Err())
				default:
					emit(DebugEventComplete, *new(T), nil)
				}
			}()

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					index++

					if res.IsError() {
						emit(DebugEventError, *new(T), res.Error())
					} else if res.IsSentinel() {
						emit(DebugEventSentinel, *new(T), nil)
					} else {
						emit(DebugEventValue, res.Value(), nil)
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

// RateMeter tracks the rate of items per second over a sliding window.
type RateMeter struct {
	mu         sync.Mutex
	window     time.Duration
	counts     []int64
	times      []time.Time
	totalCount int64
}

// NewRateMeter creates a new rate meter with the specified window size.
func NewRateMeter(window time.Duration) *RateMeter {
	return &RateMeter{
		window: window,
	}
}

// Add records a new item.
func (r *RateMeter) Add(count int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	r.counts = append(r.counts, count)
	r.times = append(r.times, now)
	r.totalCount += count

	// Remove old entries
	cutoff := now.Add(-r.window)
	for len(r.times) > 0 && r.times[0].Before(cutoff) {
		r.totalCount -= r.counts[0]
		r.counts = r.counts[1:]
		r.times = r.times[1:]
	}
}

// Rate returns the current rate per second.
func (r *RateMeter) Rate() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.times) == 0 {
		return 0
	}

	duration := time.Since(r.times[0]).Seconds()
	if duration <= 0 {
		return 0
	}

	return float64(r.totalCount) / duration
}

// TotalCount returns the total count within the window.
func (r *RateMeter) TotalCount() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.totalCount
}

// MeterRate creates a Transformer that tracks the rate of items using a RateMeter.
func MeterRate[T any](meter *RateMeter) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					if res.IsValue() {
						meter.Add(1)
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

// Histogram tracks the distribution of values.
type Histogram[T comparable] struct {
	mu     sync.RWMutex
	counts map[T]int64
	total  int64
}

// NewHistogram creates a new histogram.
func NewHistogram[T comparable]() *Histogram[T] {
	return &Histogram[T]{
		counts: make(map[T]int64),
	}
}

// Add records a value.
func (h *Histogram[T]) Add(value T) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.counts[value]++
	h.total++
}

// Count returns the count for a specific value.
func (h *Histogram[T]) Count(value T) int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.counts[value]
}

// Total returns the total count.
func (h *Histogram[T]) Total() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.total
}

// Counts returns a copy of all counts.
func (h *Histogram[T]) Counts() map[T]int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make(map[T]int64, len(h.counts))
	for k, v := range h.counts {
		result[k] = v
	}
	return result
}

// MeterHistogram creates a Transformer that tracks value distribution.
func MeterHistogram[T comparable](histogram *Histogram[T]) core.Transformer[T, T] {
	return core.Transmitter[T, T](func(ctx context.Context, in <-chan *core.Result[T]) <-chan *core.Result[T] {
		out := make(chan *core.Result[T])
		go func() {
			defer close(out)

			for {
				select {
				case <-ctx.Done():
					return
				case res, ok := <-in:
					if !ok {
						return
					}

					if res.IsValue() {
						histogram.Add(res.Value())
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
