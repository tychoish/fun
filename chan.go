package fun

import (
	"context"
	"io"

	"github.com/tychoish/fun/internal"
)

// ErrSkippedNonBlockingChannelOperation is returned when sending into
// a channel, in a non-blocking context, when the channel was full and
// the send or receive was therefore skipped.
var ErrSkippedNonBlockingChannelOperation = internal.ErrSkippedNonBlockingChannelOperation

// blockingMode provides named constants for blocking/non-blocking
// operations. They are fully internal, and only used indirectly.
type blockingMode int8

const (
	blocking     blockingMode = 1
	non_blocking blockingMode = 2
)

// ChannelOp is a wrapper around a channel, to make it easier to write
// clear code that uses and handles basic operations with single
// channels. From a high level an operation might look like:
//
//	ch := make(chan string)
//	err := fun.Blocking().Send()
type ChannelOp[T any] struct {
	mode blockingMode
	ch   chan T
}

// Blocking produces a blocking Send instance. All Send/Check/Ignore
// operations will block until the context is canceled, the channel is
// canceled, or the send succeeds.
func Blocking[T any](ch chan T) ChannelOp[T] { return ChannelOp[T]{mode: blocking, ch: ch} }

// NonBlocking produces a send instance that performs a non-blocking
// send.
//
// The Send() method, for non-blocking sends, will return
// ErrSkipedNonBlockingSend if the channel was full and the object was
// not sent.
func NonBlocking[T any](ch chan T) ChannelOp[T] { return ChannelOp[T]{mode: non_blocking, ch: ch} }

func (op ChannelOp[T]) Send() Send[T]       { return Send[T]{mode: op.mode, ch: op.ch} }
func (op ChannelOp[T]) Recieve() Receive[T] { return Receive[T]{mode: op.mode, ch: op.ch} }

// Receive, wraps a channel fore <-chan T operations. It is the type
// returned by the Receive() method on ChannelOp. The primary method
// is Read(), with other methods provided as "self-documenting"
// helpers.
type Receive[T any] struct {
	mode blockingMode
	ch   <-chan T
}

// Drop performs a read operation and drops the response. If an item
// was dropped (e.g. Read would return an error), Drop() returns
// false, and true when the Drop was successful.
func (ro Receive[T]) Drop(ctx context.Context) bool { _, err := ro.Read(ctx); return err == nil }

// Force ignores the error returning only the value from Read. This is
// either the value sent through the channel, or the zero value for
// T. Because zero values can be sent through channels, Force does not
// provide a way to distinguish between "channel-closed" and "received
// a zero value".
func (ro Receive[T]) Force(ctx context.Context) (out T) { out, _ = ro.Read(ctx); return }

// Check performs the read operation and converts the error into an
// "ok" value, returning true if receive was successful and false
// otherwise.
func (ro Receive[T]) Check(ctx context.Context) (T, bool) { o, e := ro.Read(ctx); return o, e == nil }

// Read performs the read operation according to the
// blocking/non-blocking semantics of the receive operation.
//
// In general errors are either: io.EOF if channel is closed; a
// context cancellation error if the context passed to Read() is
// canceled, or ErrSkippedNonBlockingChannelOperation in the
// non-blocking case if the channel was empty.
//
// In all cases when Read() returns an error, the return value is the
// zero value for T.
func (ro Receive[T]) Read(ctx context.Context) (T, error) {
	switch ro.mode {
	case blocking:
		return internal.ReadOne(ctx, ro.ch)
	case non_blocking:
		return internal.NonBlockingReadOne(ctx, ro.ch)
	default:
		// this is impossible without an invalid blockingMode
		// value
		return ZeroOf[T](), io.EOF
	}
}

// Send provides access to channel send operations, and is
// contstructed by the Send() method on the channel operation. The
// primary method is Write(), with other methods provided for clarity.
type Send[T any] struct {
	mode blockingMode
	ch   chan<- T
}

// Check performs a send and returns true when the send was successful
// and false otherwise.
func (sm Send[T]) Check(ctx context.Context, it T) bool { return sm.Write(ctx, it) == nil }

// Ignore performs a send and omits the error.
func (sm Send[T]) Ignore(ctx context.Context, it T) { _ = sm.Write(ctx, it) }

// Write sends the item into the channel captured by
// Blocking/NonBlocking returning the appropriate error.
//
// The returned error is nil if the send was successful, and an io.EOF
// if the channel is closed rather than a panic (as with the
// equivalent direct operation.) The error value is a context
// cancelation error when the context is canceled, and for
// non-blocking sends, if the channel did not accept the write,
// ErrSkippedNonBlockingChannelOperation is returned.
func (sm Send[T]) Write(ctx context.Context, it T) (err error) {
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
			return ErrSkippedNonBlockingChannelOperation
		}
	default:
		// it should be impossible to provoke an EOF error
		// outside of this project, because you'd need to
		// construct an invalid Send object.
		return io.EOF
	}
}