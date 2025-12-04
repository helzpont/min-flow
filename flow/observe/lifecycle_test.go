package observe_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lguimbarda/min-flow/flow/core"
	"github.com/lguimbarda/min-flow/flow/observe"
)

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
