package fun

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun/internal"
)

// ReadOnce reads one item from the channel, and returns it. ReadOne
// returns early if the context is canceled (ctx.Err()) or the channel
// is closed (io.EOF).
func ReadOne[T any](ctx context.Context, ch <-chan T) (T, error) {
	return internal.ReadOne(ctx, ch)
}

// WhenCall runs a function when condition is true, and is a noop
// otherwise.
func WhenCall(cond bool, op func()) {
	if !cond {
		return
	}
	op()
}

// WhenDo calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type.
func WhenDo[T any](cond bool, op func() T) T {
	if !cond {
		return ZeroOf[T]()
	}
	return op()
}

// Contain returns true if an element of the slice is equal to the
// item.
func Contains[T comparable](item T, slice []T) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

// Send provides a functional and descriptive interface for sending
// into channels. The only meaningful use of Send objects is via the
// Blocking() and NonBlocking() constructors. Invocations resemble:
//
//	ch := make(chan int)
//	err := fun.Blocking(ch).Send(ctx, 42)
//	// handle error, which is always a context cancellation error
//
// There are three kinds of sends: Check, which returns a boolean if
// the send was successful; Send(), which returns the error useful if
// you need to distinguish between timeouts and cancellations; and
// Ignore which has no response.
//
// Send operations against closed channels return io.EOF (or false, in
// the case of Check) rather than panicing.
type Send[T any] struct {
	mode blockingMode
	ch   chan<- T
}

type blockingMode int8

const (
	blocking     blockingMode = 1
	non_blocking blockingMode = 2
)

// ErrSkippedBlockingSend is returned when sending into a channel, in
// a non-blocking context, when the channel was blocking and the send
// was therefore skipped.
var ErrSkippedBlockingSend = errors.New("skipped blocking send")

// Blocking produces a blocking Send instance. All Send/Check/Ignore
// operations will block until the context is canceled, the channel is
// canceled, or the send succeeds.
func Blocking[T any](ch chan<- T) Send[T] { return Send[T]{mode: blocking, ch: ch} }

// NonBlocking produces a send instance that performs a non-blocking send.
func NonBlocking[T any](ch chan<- T) Send[T] { return Send[T]{mode: non_blocking, ch: ch} }

// Check performs a send and returns true when the send was successful.
func (sm Send[T]) Check(ctx context.Context, it T) bool { return sm.Send(ctx, it) == nil }

// Ignore performs a send and omits the error.
func (sm Send[T]) Ignore(ctx context.Context, it T) { _ = sm.Send(ctx, it) }

// Send sends the item into the channel captured by
// Blocking/NonBlocking returning the appropriate error.
func (sm Send[T]) Send(ctx context.Context, it T) (err error) {
	defer func() {
		if recover() != nil {
			err = io.EOF
		}
	}()

	switch sm.mode {
	case blocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sm.ch <- it:
			return nil
		}
	case non_blocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sm.ch <- it:
			return nil
		default:
			return ErrSkippedBlockingSend
		}
	default:
		// it should be impossible to provoke an EOF error
		// outside of this project, because you'd need to
		// construct an invalid Send object.
		return io.EOF
	}
}
