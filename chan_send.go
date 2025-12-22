package fun

import (
	"context"
	"io"

	"github.com/tychoish/fun/ft"
)

// ChanSend provides access to channel send operations, and is
// contstructed by the ChanSend() method on the channel operation. The
// primary method is Write(), with other methods provided for clarity.
type ChanSend[T any] struct {
	mode blockingMode
	ch   chan<- T
}

// Check performs a send and returns true when the send was successful
// and false otherwise.
func (sm ChanSend[T]) Check(ctx context.Context, it T) bool { return sm.Write(ctx, it) == nil }

// Ignore performs a send and omits the error.
func (sm ChanSend[T]) Ignore(ctx context.Context, it T) { ft.Ignore(sm.Write(ctx, it)) }

// Zero sends the zero value of T through the channel.
func (sm ChanSend[T]) Zero(ctx context.Context) error { var v T; return sm.Write(ctx, v) }

// Signal attempts to sends the Zero value of T through the channel
// and returns when: the send succeeds, the channel is full and this
// is a non-blocking send, the context is canceled, or the channel is
// closed.
func (sm ChanSend[T]) Signal(ctx context.Context) { var v T; sm.Ignore(ctx, v) }

// Write sends the item into the channel captured by
// Blocking/NonBlocking returning the appropriate error.
//
// The returned error is nil if the send was successful, and an io.EOF
// if the channel is closed (or nil) rather than a panic (as with the
// equivalent direct operation.) The error value is a context
// cancelation error when the context is canceled, and for
// non-blocking sends, if the channel did not accept the write,
// ErrSkippedNonBlockingChannelOperation is returned.
func (sm ChanSend[T]) Write(ctx context.Context, it T) (err error) {
	defer func() {
		if recover() != nil {
			err = io.EOF
		}
	}()

	switch sm.mode {
	case modeBlocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sm.ch <- it:
			return nil
		}
	case modeNonBlocking:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sm.ch <- it:
			return nil
		default:
			return ErrNonBlockingChannelOperationSkipped
		}
	default:
		// it should be impossible to provoke an EOF error
		// outside of this project, because you'd need to
		// construct an invalid Send object.
		return io.EOF
	}
}
