package core

import "context"

// CapacityMapper is an experimental Mapper variant that batches context checks
// based on output channel capacity. Instead of checking ctx.Done() on every
// item, it only checks when the channel might block.
//
// Hypothesis: For buffered channels, we can skip context checks when we know
// the send won't block, reducing overhead significantly.
//
// Trade-offs:
//   - Pro: Fewer context checks = less overhead
//   - Con: Delayed cancellation response (up to bufferSize items)
//   - Con: More complex code path
type CapacityMapper[IN, OUT any] struct {
	mapFunc func(Result[IN]) (Result[OUT], error)
}

// NewCapacityMapper creates a mapper that batches context checks.
func NewCapacityMapper[IN, OUT any](fn func(IN) (OUT, error)) CapacityMapper[IN, OUT] {
	return CapacityMapper[IN, OUT]{
		mapFunc: func(res Result[IN]) (Result[OUT], error) {
			if res.IsError() {
				return Err[OUT](res.Error()), nil
			}
			if res.IsSentinel() {
				return Sentinel[OUT](res.Sentinel()), nil
			}

			var resOut Result[OUT]
			func() {
				defer func() {
					if r := recover(); r != nil {
						resOut = Err[OUT](NewPanicError(r))
					}
				}()
				out, err := fn(res.Value())
				if err != nil {
					resOut = Err[OUT](err)
				} else {
					resOut = Ok(out)
				}
			}()
			return resOut, nil
		},
	}
}

// Apply transforms a stream with capacity-based context checking.
func (m CapacityMapper[IN, OUT]) Apply(ctx context.Context, s Stream[IN]) Stream[OUT] {
	return m.ApplyWith(ctx, s, DefaultBufferSize)
}

// ApplyWith transforms a stream with a custom buffer size.
func (m CapacityMapper[IN, OUT]) ApplyWith(ctx context.Context, s Stream[IN], bufferSize int) Stream[OUT] {
	return Emit(func(ctx context.Context) <-chan Result[OUT] {
		outChan := make(chan Result[OUT], bufferSize)
		go func() {
			defer close(outChan)

			// Track how many items we've sent without a context check
			itemsSinceCheck := 0

			for resIn := range s.Emit(ctx) {
				resOut, err := m.mapFunc(resIn)
				if err != nil {
					// Error from the mapper function itself (not from the value)
					select {
					case <-ctx.Done():
						return
					case outChan <- Err[OUT](err):
					}
					itemsSinceCheck = 0
					continue
				}

				// Capacity-based check: only check context when we might block
				// or when we've processed a batch
				if itemsSinceCheck >= bufferSize {
					select {
					case <-ctx.Done():
						return
					default:
					}
					itemsSinceCheck = 0
				}

				// Try non-blocking send first
				select {
				case outChan <- resOut:
					itemsSinceCheck++
				default:
					// Channel is full, must block - check context
					select {
					case <-ctx.Done():
						return
					case outChan <- resOut:
						itemsSinceCheck = 0
					}
				}
			}
		}()
		return outChan
	})
}

// CapacityTransmitter is a Transmitter that batches context checks.
func CapacityTransmit[T any](bufferSize int) Transmitter[T, T] {
	return Transmit(func(ctx context.Context, in <-chan Result[T]) <-chan Result[T] {
		out := make(chan Result[T], bufferSize)

		go func() {
			defer close(out)

			itemsSinceCheck := 0

			for res := range in {
				// Batch context checks based on buffer capacity
				if itemsSinceCheck >= bufferSize {
					select {
					case <-ctx.Done():
						return
					default:
					}
					itemsSinceCheck = 0
				}

				// Try non-blocking send first
				select {
				case out <- res:
					itemsSinceCheck++
				default:
					// Channel is full, must block - check context
					select {
					case <-ctx.Done():
						return
					case out <- res:
						itemsSinceCheck = 0
					}
				}
			}
		}()

		return out
	})
}

// OptimisticSend attempts to send without context check when possible.
// Returns true if sent successfully, false if context was cancelled.
//
// This is a utility function that can be used in custom transformers to
// implement capacity-based context checking.
func OptimisticSend[T any](ctx context.Context, ch chan<- T, value T, itemsSinceCheck *int, bufferSize int) bool {
	// Check context periodically
	if *itemsSinceCheck >= bufferSize {
		select {
		case <-ctx.Done():
			return false
		default:
		}
		*itemsSinceCheck = 0
	}

	// Try non-blocking send
	select {
	case ch <- value:
		(*itemsSinceCheck)++
		return true
	default:
		// Full buffer - must check context
		select {
		case <-ctx.Done():
			return false
		case ch <- value:
			*itemsSinceCheck = 0
			return true
		}
	}
}
