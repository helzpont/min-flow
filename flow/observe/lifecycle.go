package observe

import (
	"context"

	"github.com/lguimbarda/min-flow/flow/core"
)

// Notification represents a materialized stream event.
// This allows downstream operators to treat all events uniformly.
type Notification[T any] struct {
	Kind   NotificationKind
	Value  T
	Error  error
	Result core.Result[T]
}

// NotificationKind indicates the type of notification.
type NotificationKind int

const (
	NotificationValue NotificationKind = iota
	NotificationError
	NotificationSentinel
	NotificationComplete
)

// MaterializeNotification converts a stream of T into a stream of Notification[T].
// This is useful for testing, serialization, or when you need to treat all
// stream events uniformly.
func MaterializeNotification[T any]() core.Transformer[T, Notification[T]] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[Notification[T]] {
		out := make(chan core.Result[Notification[T]])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var notification Notification[T]

				switch {
				case res.IsValue():
					notification = Notification[T]{
						Kind:   NotificationValue,
						Value:  res.Value(),
						Result: res,
					}
				case res.IsSentinel():
					notification = Notification[T]{
						Kind:   NotificationSentinel,
						Error:  res.Error(),
						Result: res,
					}
				case res.IsError():
					notification = Notification[T]{
						Kind:   NotificationError,
						Error:  res.Error(),
						Result: res,
					}
				}

				select {
				case <-ctx.Done():
					return
				case out <- core.Ok(notification):
				}
			}

			// Send completion notification
			select {
			case <-ctx.Done():
				return
			case out <- core.Ok(Notification[T]{Kind: NotificationComplete}):
			}
		}()

		return out
	})
}

// DematerializeNotification converts a stream of Notification[T] back into a stream of T.
func DematerializeNotification[T any]() core.Transformer[Notification[T], T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[Notification[T]]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Skip errors in the notification stream
				if !res.IsValue() {
					continue
				}

				notification := res.Value()

				var result core.Result[T]
				switch notification.Kind {
				case NotificationValue:
					result = core.Ok(notification.Value)
				case NotificationError:
					result = core.Err[T](notification.Error)
				case NotificationSentinel:
					result = core.Sentinel[T](notification.Error)
				case NotificationComplete:
					// Stream completion - just return
					return
				}

				select {
				case <-ctx.Done():
					return
				case out <- result:
				}
			}
		}()

		return out
	})
}
