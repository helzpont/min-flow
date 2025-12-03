package observe

import (
	"context"
	"sync/atomic"

	"github.com/lguimbarda/min-flow/flow/core"
)

// DoOnNext creates a Transformer that calls a handler function for each successful value.
// The handler is called for side effects only; values pass through unchanged.
func DoOnNext[T any](handler func(T)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsValue() {
					handler(res.Value())
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// DoOnError creates a Transformer that calls a handler function for each error.
// The handler is called for side effects only; errors pass through unchanged.
func DoOnError[T any](handler func(error)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsError() {
					handler(res.Error())
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// DoOnComplete creates a Transformer that calls a handler function when the stream completes.
// The handler is called for side effects only; the stream passes through unchanged.
// The handler is called whether the stream completes normally, with errors, or via cancellation.
func DoOnComplete[T any](handler func()) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)
			defer handler()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// DoOnEach creates a Transformer that calls a handler function for each Result (value, error, or sentinel).
// The handler is called for side effects only; results pass through unchanged.
func DoOnEach[T any](handler func(core.Result[T])) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				handler(res)

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// Tap creates a Transformer that observes values, errors, and completion events.
// All handlers are optional and called for side effects only.
type TapHandlers[T any] struct {
	OnNext     func(T)
	OnError    func(error)
	OnComplete func()
}

// Tap creates a Transformer with the given handlers.
// Nil handlers are safely ignored.
func Tap[T any](handlers TapHandlers[T]) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			defer func() {
				if handlers.OnComplete != nil {
					handlers.OnComplete()
				}
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsValue() && handlers.OnNext != nil {
					handlers.OnNext(res.Value())
				} else if res.IsError() && handlers.OnError != nil {
					handlers.OnError(res.Error())
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// Finally creates a Transformer that ensures a cleanup function is called when the stream ends.
// The cleanup function is called regardless of how the stream terminates (normal completion,
// error, or context cancellation).
func Finally[T any](cleanup func()) core.Transformer[T, T] {
	return DoOnComplete[T](cleanup)
}

// DoOnSubscribe creates a Transformer that calls a handler when the stream starts processing.
// The handler is called once at the beginning of stream consumption.
func DoOnSubscribe[T any](handler func()) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			// Call handler once at stream start
			handler()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// DoOnCancel creates a Transformer that calls a handler when the stream is cancelled via context.
func DoOnCancel[T any](handler func()) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			for {
				select {
				case <-ctx.Done():
					handler()
					return
				case res, ok := <-in:
					if !ok {
						// Input closed - check if it was due to cancellation
						select {
						case <-ctx.Done():
							handler()
						default:
							// Normal stream completion, not cancellation
						}
						return
					}

					select {
					case <-ctx.Done():
						handler()
						return
					case out <- res:
					}
				}
			}
		}()

		return out
	})
}

// CountValues creates a Transformer that counts items passing through and provides the count on completion.
// Values pass through unchanged. The count callback is called when the stream completes.
func CountValues[T any](onCount func(int64)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var count int64

			defer func() {
				onCount(count)
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if res.IsValue() {
					atomic.AddInt64(&count, 1)
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// CountAll creates a Transformer that counts all results (including errors and sentinels).
// The count callback is called when the stream completes.
func CountAll[T any](onCount func(values, errors, sentinels int64)) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var values, errs, sentinels int64

			defer func() {
				onCount(values, errs, sentinels)
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				switch {
				case res.IsValue():
					atomic.AddInt64(&values, 1)
				case res.IsSentinel():
					atomic.AddInt64(&sentinels, 1)
				case res.IsError():
					atomic.AddInt64(&errs, 1)
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// Log creates a Transformer that logs each value using a custom formatter.
// The formatter converts each value to a string, and the logger handles the output.
func Log[T any](formatter func(T) string, logger func(string)) core.Transformer[T, T] {
	return DoOnNext(func(v T) {
		logger(formatter(v))
	})
}

// Trace creates a Transformer that provides tracing for stream events.
// It calls the tracer with event information for debugging and monitoring.
type TraceEvent int

const (
	TraceValue TraceEvent = iota
	TraceError
	TraceSentinel
	TraceComplete
	TraceCancel
)

// Trace creates a Transformer that traces stream events.
func Trace[T any](tracer func(event TraceEvent, result core.Result[T])) core.Transformer[T, T] {
	return core.Transmit(func(ctx context.Context, in <-chan core.Result[T]) <-chan core.Result[T] {
		out := make(chan core.Result[T])

		go func() {
			defer close(out)

			var zeroResult core.Result[T]
			defer func() {
				select {
				case <-ctx.Done():
					tracer(TraceCancel, zeroResult)
				default:
					tracer(TraceComplete, zeroResult)
				}
			}()

			for res := range in {
				select {
				case <-ctx.Done():
					return
				default:
				}

				switch {
				case res.IsValue():
					tracer(TraceValue, res)
				case res.IsSentinel():
					tracer(TraceSentinel, res)
				case res.IsError():
					tracer(TraceError, res)
				}

				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}()

		return out
	})
}

// Materialize creates a Transformer that wraps each Result in a Notification.
// This allows downstream operators to treat all events uniformly.
type Notification[T any] struct {
	Kind   NotificationKind
	Value  T
	Error  error
	Result core.Result[T]
}

type NotificationKind int

const (
	NotificationValue NotificationKind = iota
	NotificationError
	NotificationSentinel
	NotificationComplete
)

// MaterializeNotification converts a stream of T into a stream of Notification[T].
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
