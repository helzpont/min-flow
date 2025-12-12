package observe

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lguimbarda/min-flow/flow/core"
)

// This file contains interceptor-based observers that can be registered with
// a Registry and invoked automatically when streams are processed through
// core.Intercept(). These interceptors are for observation/side effects only
// and do not modify the data flow.
//
// For observation that requires a pipeline stage, use the transformer-based
// observers in observe.go and lifecycle.go instead.

// MetricsInterceptor collects stream metrics as an interceptor.
// Register it with a Registry to automatically collect metrics
// when streams are processed through core.Intercept().
type MetricsInterceptor struct {
	mu      sync.Mutex
	metrics StreamMetrics

	lastItemTime time.Time
	totalLatency time.Duration
	latencyCount int64

	onComplete func(StreamMetrics)
}

// NewMetricsInterceptor creates a new MetricsInterceptor.
// The onComplete callback is called when StreamEnd is received.
func NewMetricsInterceptor(onComplete func(StreamMetrics)) *MetricsInterceptor {
	return &MetricsInterceptor{
		metrics: StreamMetrics{
			MinLatency: time.Duration(1<<63 - 1), // Max duration
		},
		onComplete: onComplete,
	}
}

func (m *MetricsInterceptor) Init() error  { return nil }
func (m *MetricsInterceptor) Close() error { return nil }

func (m *MetricsInterceptor) Events() []core.Event {
	return []core.Event{
		core.StreamStart,
		core.StreamEnd,
		core.ItemReceived,
	}
}

func (m *MetricsInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch event {
	case core.StreamStart:
		m.metrics.StartTime = time.Now()

	case core.StreamEnd:
		m.metrics.EndTime = time.Now()
		if m.metrics.TotalItems > 0 {
			duration := m.metrics.EndTime.Sub(m.metrics.StartTime).Seconds()
			if duration > 0 {
				m.metrics.ItemsPerSecond = float64(m.metrics.TotalItems) / duration
			}
			if m.latencyCount > 0 {
				m.metrics.AvgLatency = m.totalLatency / time.Duration(m.latencyCount)
			}
		}
		if m.onComplete != nil {
			m.onComplete(m.metrics)
		}

	case core.ItemReceived:
		now := time.Now()
		m.metrics.TotalItems++

		// Track first/last item times
		if m.metrics.TotalItems == 1 {
			m.metrics.FirstItemTime = now
		}
		m.metrics.LastItemTime = now

		// Calculate latency
		if !m.lastItemTime.IsZero() {
			latency := now.Sub(m.lastItemTime)
			if latency < m.metrics.MinLatency {
				m.metrics.MinLatency = latency
			}
			if latency > m.metrics.MaxLatency {
				m.metrics.MaxLatency = latency
			}
			m.totalLatency += latency
			m.latencyCount++
		}
		m.lastItemTime = now

		// Categorize result
		if len(args) > 0 {
			if res, ok := args[0].(interface{ IsError() bool }); ok {
				if res.IsError() {
					m.metrics.ErrorCount++
				} else if sentinel, ok := args[0].(interface{ IsSentinel() bool }); ok && sentinel.IsSentinel() {
					m.metrics.SentinelCount++
				} else {
					m.metrics.ValueCount++
				}
			}
		}
	}

	return nil
}

// Metrics returns a copy of the current metrics.
func (m *MetricsInterceptor) Metrics() StreamMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metrics
}

// LiveMetricsInterceptor updates live metrics that can be read concurrently.
type LiveMetricsInterceptor struct {
	metrics *LiveMetrics
}

// NewLiveMetricsInterceptor creates an interceptor that updates the given LiveMetrics.
func NewLiveMetricsInterceptor(metrics *LiveMetrics) *LiveMetricsInterceptor {
	return &LiveMetricsInterceptor{metrics: metrics}
}

func (m *LiveMetricsInterceptor) Init() error  { return nil }
func (m *LiveMetricsInterceptor) Close() error { return nil }

func (m *LiveMetricsInterceptor) Events() []core.Event {
	return []core.Event{
		core.StreamStart,
		core.ValueReceived,
		core.ErrorOccurred,
		core.SentinelReceived,
	}
}

func (m *LiveMetricsInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	switch event {
	case core.StreamStart:
		now := time.Now()
		m.metrics.startTime.Store(&now)

	case core.ValueReceived:
		m.metrics.totalItems.Add(1)
		m.metrics.valueCount.Add(1)
		now := time.Now()
		m.metrics.lastItemTime.Store(&now)

	case core.ErrorOccurred:
		m.metrics.totalItems.Add(1)
		m.metrics.errorCount.Add(1)
		now := time.Now()
		m.metrics.lastItemTime.Store(&now)

	case core.SentinelReceived:
		m.metrics.totalItems.Add(1)
		m.metrics.sentinelCount.Add(1)
		now := time.Now()
		m.metrics.lastItemTime.Store(&now)
	}

	return nil
}

// LogInterceptor logs events as they occur.
type LogInterceptor struct {
	logger func(format string, args ...any)
	events []core.Event
}

// NewLogInterceptor creates a logging interceptor.
// The logger function is called for each matching event.
// If events is empty, all common events are logged.
func NewLogInterceptor(logger func(format string, args ...any), events ...core.Event) *LogInterceptor {
	if len(events) == 0 {
		events = []core.Event{
			core.StreamStart,
			core.StreamEnd,
			core.ItemReceived,
			core.ValueReceived,
			core.ErrorOccurred,
			core.SentinelReceived,
		}
	}
	return &LogInterceptor{
		logger: logger,
		events: events,
	}
}

func (l *LogInterceptor) Init() error  { return nil }
func (l *LogInterceptor) Close() error { return nil }

func (l *LogInterceptor) Events() []core.Event {
	return l.events
}

func (l *LogInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	if l.logger != nil {
		l.logger("[%s] args=%v", event, args)
	}
	return nil
}

// CounterInterceptor provides simple atomic counters for values, errors, and total items.
type CounterInterceptor struct {
	values    atomic.Int64
	errors    atomic.Int64
	sentinels atomic.Int64
	total     atomic.Int64
}

// NewCounterInterceptor creates a new counter interceptor.
func NewCounterInterceptor() *CounterInterceptor {
	return &CounterInterceptor{}
}

func (c *CounterInterceptor) Init() error  { return nil }
func (c *CounterInterceptor) Close() error { return nil }

func (c *CounterInterceptor) Events() []core.Event {
	return []core.Event{
		core.ValueReceived,
		core.ErrorOccurred,
		core.SentinelReceived,
	}
}

func (c *CounterInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	c.total.Add(1)
	switch event {
	case core.ValueReceived:
		c.values.Add(1)
	case core.ErrorOccurred:
		c.errors.Add(1)
	case core.SentinelReceived:
		c.sentinels.Add(1)
	}
	return nil
}

// Values returns the count of successful values.
func (c *CounterInterceptor) Values() int64 { return c.values.Load() }

// Errors returns the count of errors.
func (c *CounterInterceptor) Errors() int64 { return c.errors.Load() }

// Sentinels returns the count of sentinels.
func (c *CounterInterceptor) Sentinels() int64 { return c.sentinels.Load() }

// Total returns the total count of all items.
func (c *CounterInterceptor) Total() int64 { return c.total.Load() }

// CallbackInterceptor invokes callbacks for specific events.
type CallbackInterceptor struct {
	onValue    func(any)
	onError    func(error)
	onStart    func()
	onComplete func()
}

// CallbackInterceptorOption configures a CallbackInterceptor.
type CallbackInterceptorOption func(*CallbackInterceptor)

// WithOnValue sets a callback for value events.
func WithOnValue(fn func(any)) CallbackInterceptorOption {
	return func(c *CallbackInterceptor) {
		c.onValue = fn
	}
}

// WithOnError sets a callback for error events.
func WithOnError(fn func(error)) CallbackInterceptorOption {
	return func(c *CallbackInterceptor) {
		c.onError = fn
	}
}

// WithOnStart sets a callback for stream start.
func WithOnStart(fn func()) CallbackInterceptorOption {
	return func(c *CallbackInterceptor) {
		c.onStart = fn
	}
}

// WithOnComplete sets a callback for stream completion.
func WithOnComplete(fn func()) CallbackInterceptorOption {
	return func(c *CallbackInterceptor) {
		c.onComplete = fn
	}
}

// NewCallbackInterceptor creates a callback interceptor with the given options.
func NewCallbackInterceptor(opts ...CallbackInterceptorOption) *CallbackInterceptor {
	c := &CallbackInterceptor{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CallbackInterceptor) Init() error  { return nil }
func (c *CallbackInterceptor) Close() error { return nil }

func (c *CallbackInterceptor) Events() []core.Event {
	var events []core.Event
	if c.onStart != nil {
		events = append(events, core.StreamStart)
	}
	if c.onComplete != nil {
		events = append(events, core.StreamEnd)
	}
	if c.onValue != nil {
		events = append(events, core.ValueReceived)
	}
	if c.onError != nil {
		events = append(events, core.ErrorOccurred)
	}
	if len(events) == 0 {
		return []core.Event{} // No events to handle
	}
	return events
}

func (c *CallbackInterceptor) Do(ctx context.Context, event core.Event, args ...any) error {
	switch event {
	case core.StreamStart:
		if c.onStart != nil {
			c.onStart()
		}
	case core.StreamEnd:
		if c.onComplete != nil {
			c.onComplete()
		}
	case core.ValueReceived:
		if c.onValue != nil && len(args) > 0 {
			c.onValue(args[0])
		}
	case core.ErrorOccurred:
		if c.onError != nil && len(args) > 0 {
			if err, ok := args[0].(error); ok {
				c.onError(err)
			}
		}
	}
	return nil
}
