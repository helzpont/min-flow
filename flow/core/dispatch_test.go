package core

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// testInterceptor tracks which events it received
type testInterceptor struct {
	mu       sync.Mutex
	events   []Event
	args     []any
	patterns []Event
}

func newTestInterceptor(patterns ...string) *testInterceptor {
	evPatterns := make([]Event, len(patterns))
	for i, p := range patterns {
		evPatterns[i] = Event(p)
	}
	return &testInterceptor{
		patterns: evPatterns,
	}
}

func (ti *testInterceptor) Events() []Event { return ti.patterns }

func (ti *testInterceptor) Do(ctx context.Context, event Event, args ...any) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	ti.events = append(ti.events, event)
	if len(args) > 0 {
		ti.args = append(ti.args, args[0])
	}
	return nil
}

func (ti *testInterceptor) Init() error  { return nil }
func (ti *testInterceptor) Close() error { return nil }

func (ti *testInterceptor) getEvents() []Event {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	result := make([]Event, len(ti.events))
	copy(result, ti.events)
	return result
}

func (ti *testInterceptor) getArgs() []any {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	result := make([]any, len(ti.args))
	copy(result, ti.args)
	return result
}

// testStreamFromSlice creates a stream from a slice for testing
func testStreamFromSlice[T any](data []T) Stream[T] {
	return Emit(func(ctx context.Context) <-chan Result[T] {
		ch := make(chan Result[T], len(data))
		for _, v := range data {
			ch <- Ok(v)
		}
		close(ch)
		return ch
	})
}

// TestMapper_AutoInvokesInterceptors verifies that Mapper automatically fires interceptor events
func TestMapper_AutoInvokesInterceptors(t *testing.T) {
	tests := []struct {
		name           string
		patterns       []string
		input          []int
		wantEventTypes []Event
	}{
		{
			name:     "receives StreamStart and StreamEnd",
			patterns: []string{string(StreamStart), string(StreamEnd)},
			input:    []int{1, 2, 3},
			wantEventTypes: []Event{
				StreamStart,
				StreamEnd,
			},
		},
		{
			name:     "receives ItemReceived for each input",
			patterns: []string{string(ItemReceived)},
			input:    []int{1, 2, 3},
			wantEventTypes: []Event{
				ItemReceived,
				ItemReceived,
				ItemReceived,
			},
		},
		{
			name:     "receives ValueReceived for each successful output",
			patterns: []string{string(ValueReceived)},
			input:    []int{1, 2, 3},
			wantEventTypes: []Event{
				ValueReceived,
				ValueReceived,
				ValueReceived,
			},
		},
		{
			name:     "receives ItemEmitted for each output",
			patterns: []string{string(ItemEmitted)},
			input:    []int{1, 2, 3},
			wantEventTypes: []Event{
				ItemEmitted,
				ItemEmitted,
				ItemEmitted,
			},
		},
		{
			name:     "receives all events with wildcard",
			patterns: []string{"*"},
			input:    []int{1, 2},
			wantEventTypes: []Event{
				StreamStart,
				ItemReceived,
				ValueReceived,
				ItemEmitted,
				ItemReceived,
				ValueReceived,
				ItemEmitted,
				StreamEnd,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interceptor := newTestInterceptor(tt.patterns...)

			ctx, registry := WithRegistry(context.Background())
			if err := registry.Register(interceptor); err != nil {
				t.Fatalf("failed to register interceptor: %v", err)
			}

			// Create a simple mapper that doubles values
			doubler := Map(func(x int) (int, error) {
				return x * 2, nil
			})

			// Create input stream
			input := testStreamFromSlice(tt.input)

			// Apply mapper (should auto-invoke interceptors)
			output := doubler.Apply(ctx, input)

			// Consume the output to trigger processing
			_, _ = Slice(ctx, output)

			// Verify events were fired
			events := interceptor.getEvents()
			if len(events) != len(tt.wantEventTypes) {
				t.Errorf("got %d events, want %d", len(events), len(tt.wantEventTypes))
				t.Errorf("events: %v", events)
				return
			}

			for i, want := range tt.wantEventTypes {
				if events[i] != want {
					t.Errorf("event[%d] = %v, want %v", i, events[i], want)
				}
			}
		})
	}
}

// TestFlatMapper_AutoInvokesInterceptors verifies that FlatMapper automatically fires interceptor events
func TestFlatMapper_AutoInvokesInterceptors(t *testing.T) {
	interceptor := newTestInterceptor("*")

	ctx, registry := WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	// Create a flatmapper that duplicates each value
	duplicator := FlatMap(func(x int) ([]int, error) {
		return []int{x, x}, nil
	})

	// Create input stream with 2 items
	input := testStreamFromSlice([]int{1, 2})

	// Apply flatmapper (should auto-invoke interceptors)
	output := duplicator.Apply(ctx, input)

	// Consume the output
	_, _ = Slice(ctx, output)

	// For input [1, 2], each produces 2 outputs, so we expect:
	// StreamStart
	// ItemReceived (for 1)
	// ValueReceived + ItemEmitted (for first 1)
	// ValueReceived + ItemEmitted (for second 1)
	// ItemReceived (for 2)
	// ValueReceived + ItemEmitted (for first 2)
	// ValueReceived + ItemEmitted (for second 2)
	// StreamEnd
	events := interceptor.getEvents()

	// Count specific event types
	counts := make(map[Event]int)
	for _, e := range events {
		counts[e]++
	}

	if counts[StreamStart] != 1 {
		t.Errorf("StreamStart count = %d, want 1", counts[StreamStart])
	}
	if counts[StreamEnd] != 1 {
		t.Errorf("StreamEnd count = %d, want 1", counts[StreamEnd])
	}
	if counts[ItemReceived] != 2 {
		t.Errorf("ItemReceived count = %d, want 2 (one per input)", counts[ItemReceived])
	}
	if counts[ValueReceived] != 4 {
		t.Errorf("ValueReceived count = %d, want 4 (two per input)", counts[ValueReceived])
	}
	if counts[ItemEmitted] != 4 {
		t.Errorf("ItemEmitted count = %d, want 4 (two per input)", counts[ItemEmitted])
	}
}

// TestMapper_NoRegistryNoEvents verifies that without a registry, no events are fired (no panic)
func TestMapper_NoRegistryNoEvents(t *testing.T) {
	// Use context without registry
	ctx := context.Background()

	doubler := Map(func(x int) (int, error) {
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, 2, 3})
	output := doubler.Apply(ctx, input)

	// Should still work correctly
	values, err := Slice(ctx, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{2, 4, 6}
	if len(values) != len(expected) {
		t.Errorf("got %d values, want %d", len(values), len(expected))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

// TestDispatch_ErrorOccurred verifies ErrorOccurred is fired for errors
func TestDispatch_ErrorOccurred(t *testing.T) {
	interceptor := newTestInterceptor(string(ErrorOccurred))

	ctx, registry := WithRegistry(context.Background())
	_ = registry.Register(interceptor)

	errNegative := errors.New("negative value")

	// Create a mapper that errors on negative numbers
	mapper := Map(func(x int) (int, error) {
		if x < 0 {
			return 0, errNegative
		}
		return x * 2, nil
	})

	input := testStreamFromSlice([]int{1, -1, 2, -2, 3})
	output := mapper.Apply(ctx, input)

	// Consume all (errors don't stop the stream)
	_, _ = Slice(ctx, output)

	events := interceptor.getEvents()
	if len(events) != 2 {
		t.Errorf("got %d ErrorOccurred events, want 2 (for -1 and -2)", len(events))
	}
}

// TestIterFlatMapper_AutoInvokesInterceptors verifies IterFlatMapper fires interceptor events
func TestIterFlatMapper_AutoInvokesInterceptors(t *testing.T) {
	interceptor := newTestInterceptor("*")

	ctx, registry := WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	// Create an iter flatmapper that duplicates each value using IterFlatMapSlice
	duplicator := IterFlatMapSlice(func(x int) ([]int, error) {
		return []int{x, x}, nil
	})

	// Create input stream with 2 items
	input := testStreamFromSlice([]int{1, 2})

	// Apply iter flatmapper (should auto-invoke interceptors)
	output := duplicator.Apply(ctx, input)

	// Consume the output
	_, _ = Slice(ctx, output)

	// Count specific event types
	events := interceptor.getEvents()
	counts := make(map[Event]int)
	for _, e := range events {
		counts[e]++
	}

	if counts[StreamStart] != 1 {
		t.Errorf("StreamStart count = %d, want 1", counts[StreamStart])
	}
	if counts[StreamEnd] != 1 {
		t.Errorf("StreamEnd count = %d, want 1", counts[StreamEnd])
	}
	if counts[ItemReceived] != 2 {
		t.Errorf("ItemReceived count = %d, want 2 (one per input)", counts[ItemReceived])
	}
	if counts[ValueReceived] != 4 {
		t.Errorf("ValueReceived count = %d, want 4 (two per input)", counts[ValueReceived])
	}
	if counts[ItemEmitted] != 4 {
		t.Errorf("ItemEmitted count = %d, want 4 (two per input)", counts[ItemEmitted])
	}
}

// TestIntercept_AutoInvokesInterceptors verifies Intercept transmitter fires interceptor events
func TestIntercept_AutoInvokesInterceptors(t *testing.T) {
	interceptor := newTestInterceptor("*")

	ctx, registry := WithRegistry(context.Background())
	if err := registry.Register(interceptor); err != nil {
		t.Fatalf("failed to register interceptor: %v", err)
	}

	// Create input stream with 3 items
	input := testStreamFromSlice([]int{1, 2, 3})

	// Apply Intercept transmitter
	output := Intercept[int]().Apply(ctx, input)

	// Consume the output
	_, _ = Slice(ctx, output)

	// Count specific event types
	events := interceptor.getEvents()
	counts := make(map[Event]int)
	for _, e := range events {
		counts[e]++
	}

	if counts[StreamStart] != 1 {
		t.Errorf("StreamStart count = %d, want 1", counts[StreamStart])
	}
	if counts[StreamEnd] != 1 {
		t.Errorf("StreamEnd count = %d, want 1", counts[StreamEnd])
	}
	if counts[ItemReceived] != 3 {
		t.Errorf("ItemReceived count = %d, want 3", counts[ItemReceived])
	}
	if counts[ValueReceived] != 3 {
		t.Errorf("ValueReceived count = %d, want 3", counts[ValueReceived])
	}
	if counts[ItemEmitted] != 3 {
		t.Errorf("ItemEmitted count = %d, want 3", counts[ItemEmitted])
	}
}

// TestIntercept_PassthroughWithoutRegistry verifies Intercept works without registry
func TestIntercept_PassthroughWithoutRegistry(t *testing.T) {
	ctx := context.Background() // No registry

	input := testStreamFromSlice([]int{1, 2, 3})
	output := Intercept[int]().Apply(ctx, input)

	values, err := Slice(ctx, output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []int{1, 2, 3}
	if len(values) != len(expected) {
		t.Errorf("got %d values, want %d", len(values), len(expected))
	}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("values[%d] = %d, want %d", i, v, expected[i])
		}
	}
}
