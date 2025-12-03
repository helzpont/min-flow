package observe_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lguimbarda/min-flow/flow"
	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/observe"
)

func TestDoOnNext(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3, 4, 5})

	var captured []int
	handler := func(v int) {
		captured = append(captured, v)
	}

	result := observe.DoOnNext(handler).Apply(ctx, stream)
	got, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Values should pass through unchanged
	want := []int{1, 2, 3, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}

	// Handler should have captured all values
	if len(captured) != len(want) {
		t.Fatalf("captured %v, want %v", captured, want)
	}
	for i := range captured {
		if captured[i] != want[i] {
			t.Errorf("captured[%d] = %d, want %d", i, captured[i], want[i])
		}
	}
}

func TestDoOnError(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error2"))
		}()
		return out
	})
	stream := emitter

	var capturedErrors []string
	handler := func(err error) {
		capturedErrors = append(capturedErrors, err.Error())
	}

	result := observe.DoOnError[int](handler).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	// Handler should have captured all errors
	if len(capturedErrors) != 2 {
		t.Fatalf("got %d captured errors, want 2", len(capturedErrors))
	}
	if capturedErrors[0] != "error1" || capturedErrors[1] != "error2" {
		t.Errorf("captured errors = %v, want [error1, error2]", capturedErrors)
	}
}

func TestDoOnComplete(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	var completed bool
	handler := func() {
		completed = true
	}

	result := observe.DoOnComplete[int](handler).Apply(ctx, stream)
	_, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !completed {
		t.Error("completion handler was not called")
	}
}

func TestDoOnCompleteWithCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(i):
				}
			}
		}()
		return out
	})
	stream := emitter

	completedCh := make(chan struct{})
	handler := func() {
		close(completedCh)
	}

	result := observe.DoOnComplete[int](handler).Apply(ctx, stream)

	// Consume a few items then cancel
	count := 0
	for range result.Emit(ctx) {
		count++
		if count >= 5 {
			cancel()
			break
		}
	}

	// Wait for completion handler with timeout
	select {
	case <-completedCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("completion handler was not called on cancellation")
	}
}

func TestDoOnEach(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error"))
			out <- core.Sentinel[int](errors.New("sentinel"))
		}()
		return out
	})
	stream := emitter

	var valueCount, errorCount, sentinelCount int
	handler := func(r core.Result[int]) {
		switch {
		case r.IsValue():
			valueCount++
		case r.IsSentinel():
			sentinelCount++
		case r.IsError():
			errorCount++
		}
	}

	result := observe.DoOnEach(handler).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	if valueCount != 1 {
		t.Errorf("got %d values, want 1", valueCount)
	}
	if errorCount != 1 {
		t.Errorf("got %d errors, want 1", errorCount)
	}
	if sentinelCount != 1 {
		t.Errorf("got %d sentinels, want 1", sentinelCount)
	}
}

func TestTap(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error"))
			out <- core.Ok(3)
		}()
		return out
	})
	stream := emitter

	var values []int
	var errors []string
	var completed bool

	handlers := observe.TapHandlers[int]{
		OnNext: func(v int) {
			values = append(values, v)
		},
		OnError: func(err error) {
			errors = append(errors, err.Error())
		},
		OnComplete: func() {
			completed = true
		},
	}

	result := observe.Tap(handlers).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	if len(values) != 3 {
		t.Errorf("got %d values, want 3", len(values))
	}
	if len(errors) != 1 {
		t.Errorf("got %d errors, want 1", len(errors))
	}
	if !completed {
		t.Error("completion handler was not called")
	}
}

func TestTapWithNilHandlers(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	// Should not panic with nil handlers
	handlers := observe.TapHandlers[int]{
		OnNext:     nil,
		OnError:    nil,
		OnComplete: nil,
	}

	result := observe.Tap(handlers).Apply(ctx, stream)
	got, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("got %d values, want 3", len(got))
	}
}

func TestFinally(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	var cleaned bool
	cleanup := func() {
		cleaned = true
	}

	result := observe.Finally[int](cleanup).Apply(ctx, stream)
	_, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cleaned {
		t.Error("cleanup was not called")
	}
}

func TestDoOnSubscribe(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	var subscribeCalled bool
	handler := func() {
		subscribeCalled = true
	}

	result := observe.DoOnSubscribe[int](handler).Apply(ctx, stream)

	// Before consuming, handler should not be called
	if subscribeCalled {
		t.Error("subscribe handler called before consumption")
	}

	_, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !subscribeCalled {
		t.Error("subscribe handler was not called")
	}
}

func TestDoOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			for i := 0; ; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(i):
				}
			}
		}()
		return out
	})
	stream := emitter

	cancelCalledCh := make(chan struct{})
	handler := func() {
		close(cancelCalledCh)
	}

	result := observe.DoOnCancel[int](handler).Apply(ctx, stream)

	// Consume items - wait until we get at least 5
	resultChan := result.Emit(ctx)
	count := 0
	for range resultChan {
		count++
		if count >= 5 {
			cancel()
			break
		}
	}

	// Drain remaining items
	go func() {
		for range resultChan {
		}
	}()

	// Wait for the cancel handler with timeout
	select {
	case <-cancelCalledCh:
		// Success
	case <-time.After(time.Second):
		t.Error("cancel handler was not called")
	}
}

func TestCountValues(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error"))
			out <- core.Ok(2)
			out <- core.Ok(3)
		}()
		return out
	})
	stream := emitter

	var count int64
	onCount := func(c int64) {
		count = c
	}

	result := observe.CountValues[int](onCount).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	// Should count only values, not errors
	if count != 3 {
		t.Errorf("got count = %d, want 3", count)
	}
}

func TestCountAll(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Ok(2)
			out <- core.Err[int](errors.New("error1"))
			out <- core.Err[int](errors.New("error2"))
			out <- core.Sentinel[int](errors.New("sentinel"))
		}()
		return out
	})
	stream := emitter

	var values, errs, sentinels int64
	onCount := func(v, e, s int64) {
		values = v
		errs = e
		sentinels = s
	}

	result := observe.CountAll[int](onCount).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	if values != 2 {
		t.Errorf("got values = %d, want 2", values)
	}
	if errs != 2 {
		t.Errorf("got errors = %d, want 2", errs)
	}
	if sentinels != 1 {
		t.Errorf("got sentinels = %d, want 1", sentinels)
	}
}

func TestLog(t *testing.T) {
	ctx := context.Background()
	stream := flow.FromSlice([]int{1, 2, 3})

	var logs []string
	formatter := func(v int) string {
		return string(rune('0' + v))
	}
	logger := func(s string) {
		logs = append(logs, s)
	}

	result := observe.Log(formatter, logger).Apply(ctx, stream)
	_, err := flow.Slice[int](ctx, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"1", "2", "3"}
	if len(logs) != len(want) {
		t.Fatalf("got %v, want %v", logs, want)
	}
	for i := range logs {
		if logs[i] != want[i] {
			t.Errorf("logs[%d] = %q, want %q", i, logs[i], want[i])
		}
	}
}

func TestTrace(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error"))
			out <- core.Sentinel[int](errors.New("sentinel"))
		}()
		return out
	})
	stream := emitter

	var events []observe.TraceEvent
	tracer := func(event observe.TraceEvent, _ core.Result[int]) {
		events = append(events, event)
	}

	result := observe.Trace(tracer).Apply(ctx, stream)

	// Consume the stream
	for range result.Emit(ctx) {
	}

	// Should have value, error, sentinel, and complete events
	expected := []observe.TraceEvent{
		observe.TraceValue,
		observe.TraceError,
		observe.TraceSentinel,
		observe.TraceComplete,
	}
	if len(events) != len(expected) {
		t.Fatalf("got %d events, want %d", len(events), len(expected))
	}
	for i := range events {
		if events[i] != expected[i] {
			t.Errorf("events[%d] = %d, want %d", i, events[i], expected[i])
		}
	}
}

func TestTraceWithCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(i):
				}
			}
		}()
		return out
	})
	stream := emitter

	cancelEventCh := make(chan struct{})
	tracer := func(event observe.TraceEvent, _ core.Result[int]) {
		if event == observe.TraceCancel {
			close(cancelEventCh)
		}
	}

	result := observe.Trace(tracer).Apply(ctx, stream)

	// Consume a few items then cancel
	count := 0
	for range result.Emit(ctx) {
		count++
		if count >= 5 {
			cancel()
			break
		}
	}

	// Wait for cancel event with timeout
	select {
	case <-cancelEventCh:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("trace cancel event was not emitted")
	}
}

func TestMaterializeNotification(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			out <- core.Ok(1)
			out <- core.Err[int](errors.New("error"))
			out <- core.Ok(2)
		}()
		return out
	})
	stream := emitter

	result := observe.MaterializeNotification[int]().Apply(ctx, stream)

	var notifications []observe.Notification[int]
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			notifications = append(notifications, r.Value())
		}
	}

	// Should have 3 notifications + 1 completion notification
	if len(notifications) != 4 {
		t.Fatalf("got %d notifications, want 4", len(notifications))
	}

	// Check kinds
	expected := []observe.NotificationKind{
		observe.NotificationValue,
		observe.NotificationError,
		observe.NotificationValue,
		observe.NotificationComplete,
	}
	for i := range expected {
		if notifications[i].Kind != expected[i] {
			t.Errorf("notifications[%d].Kind = %d, want %d", i, notifications[i].Kind, expected[i])
		}
	}
}

func TestDematerializeNotification(t *testing.T) {
	ctx := context.Background()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[observe.Notification[int]] {
		out := make(chan core.Result[observe.Notification[int]])
		go func() {
			defer close(out)
			out <- core.Ok(observe.Notification[int]{Kind: observe.NotificationValue, Value: 1})
			out <- core.Ok(observe.Notification[int]{Kind: observe.NotificationError, Error: errors.New("error")})
			out <- core.Ok(observe.Notification[int]{Kind: observe.NotificationValue, Value: 2})
			out <- core.Ok(observe.Notification[int]{Kind: observe.NotificationComplete})
		}()
		return out
	})
	stream := emitter

	result := observe.DematerializeNotification[int]().Apply(ctx, stream)

	var values []int
	var errCount int
	for r := range result.Emit(ctx) {
		if r.IsValue() {
			values = append(values, r.Value())
		} else if r.IsError() {
			errCount++
		}
	}

	if len(values) != 2 {
		t.Errorf("got %d values, want 2", len(values))
	}
	if errCount != 1 {
		t.Errorf("got %d errors, want 1", errCount)
	}
}

func TestLifecycleContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emitter := core.Emit(func(ctx context.Context) <-chan core.Result[int] {
		out := make(chan core.Result[int])
		go func() {
			defer close(out)
			for i := 0; i < 100; i++ {
				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(i):
				}
			}
		}()
		return out
	})
	stream := emitter

	var handlerCalls int32
	handler := func(_ int) {
		atomic.AddInt32(&handlerCalls, 1)
	}

	result := observe.DoOnNext(handler).Apply(ctx, stream)

	// Consume a few items then cancel
	count := 0
	for range result.Emit(ctx) {
		count++
		if count >= 5 {
			cancel()
			break
		}
	}

	// Should have received around 5 handler calls
	calls := atomic.LoadInt32(&handlerCalls)
	if calls < 3 || calls > 10 {
		t.Errorf("got %d handler calls, expected around 5", calls)
	}
}
