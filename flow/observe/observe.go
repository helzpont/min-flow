// Package observe provides observability tools for the min-flow framework.
// This includes metrics types, rate meters, and histograms for monitoring streams.
package observe

import (
	"sync"
	"sync/atomic"
	"time"
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

// Duration returns the total duration of stream processing.
func (m *StreamMetrics) Duration() time.Duration {
	return m.EndTime.Sub(m.StartTime)
}

// LiveMetrics holds real-time metrics that can be read concurrently.
type LiveMetrics struct {
	totalItems    atomic.Int64
	valueCount    atomic.Int64
	errorCount    atomic.Int64
	sentinelCount atomic.Int64
	startTime     atomic.Pointer[time.Time]
	lastItemTime  atomic.Pointer[time.Time]
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
	if t := m.startTime.Load(); t != nil {
		return *t
	}
	return time.Time{}
}

// LastItemTime returns when the last item was processed.
func (m *LiveMetrics) LastItemTime() time.Time {
	if t := m.lastItemTime.Load(); t != nil {
		return *t
	}
	return time.Time{}
}

// Duration returns how long the stream has been running.
func (m *LiveMetrics) Duration() time.Duration {
	start := m.startTime.Load()
	if start == nil {
		return 0
	}
	return time.Since(*start)
}

// ItemsPerSecond returns the current throughput.
func (m *LiveMetrics) ItemsPerSecond() float64 {
	duration := m.Duration().Seconds()
	if duration <= 0 {
		return 0
	}
	return float64(m.TotalItems()) / duration
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
