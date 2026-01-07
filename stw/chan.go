package stw

import (
	"context"
	"errors"
	"io"
	"iter"

	"github.com/tychoish/fun/ers"
)

// blockingMode provides named constants for blocking/non-blocking
// operations. They are fully internal, and only used indirectly.
type blockingMode int8

const (
	modeBlocking    blockingMode = 1
	modeNonBlocking blockingMode = 2
)

// ChanOp is a wrapper around a channel, to make it easier to write
// clear code that uses and handles basic operations with single
// channels. From a high level an operation might look like:
//
//	ch := make(chan string)
//	err := fun.Blocking().Send("hello world")
//
// Methods on ChanOp and related structures are not pointer receivers,
// ensure that the output values are recorded as needed. Typically
// it's reasonable to avoid creating ChanOp objects in a loop as well.
type ChanOp[T any] struct {
	mode blockingMode
	ch   chan T
}

// ChanBlocking produces a blocking Send instance. All Send/Check/Ignore
// operations will block until the context is canceled, the channel is
// canceled, or the send succeeds.
func ChanBlocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: modeBlocking, ch: ch} }

// ChanNonBlocking produces a send instance that performs a non-blocking
// send.
//
// The Send() method, for non-blocking sends, will return
// ErrSkipedNonBlockingSend if the channel was full and the object was
// not sent.
func ChanNonBlocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: modeNonBlocking, ch: ch} }

// Close closes the underlying channel.
//
// This swallows any panic encountered when calling close() on the
// underlying channel, which makes it safe to call on nil or
// already-closed channels: the result in all cases (that the channel
// is closed when Close() returns, is the same in all cases.)
func (op ChanOp[T]) Close() { defer func() { _ = recover() }(); close(op.ch) }

// Len returns the current length of the channel.
func (op ChanOp[T]) Len() int { return len(op.ch) }

// Cap returns the current capacity of the channel.
func (op ChanOp[T]) Cap() int { return cap(op.ch) }

// Send returns a ChanSend object that acts on the same underlying
// channel.
func (op ChanOp[T]) Send() ChanSend[T] {
	switch op.mode {
	case modeBlocking:
		return BlockingSend(op.ch)
	case modeNonBlocking:
		return NonBlockingSend(op.ch)
	default:
		panic(errors.Join(ers.ErrInvariantViolation, ers.Error("unreachable")))
	}
}

// Receive returns a ChanReceive object that acts on the same
// underlying sender.
func (op ChanOp[T]) Receive() ChanReceive[T] {
	switch op.mode {
	case modeBlocking:
		return ChanBlockingReceive(op.ch)
	case modeNonBlocking:
		return ChanNonBlockingReceive(op.ch)
	default:
		panic(errors.Join(ers.ErrInvariantViolation, ers.Error("unreachable")))
	}
}

////////////////////////////////////////////////////////////////////////////////
//
// RECEIVE

// ChanReceive wraps a channel fore <-chan T operations. It is the type
// returned by the ChanReceive() method on ChannelOp. The primary method
// is Read(), with other methods provided as "self-documenting"
// helpers.
type ChanReceive[T any] struct {
	mode blockingMode
	ch   <-chan T
}

// ChanBlockingReceive returns a Chan wrapper for <-chan T operation, that
// will block while waiting for new items to be sent through the
// channel.
func ChanBlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: modeBlocking, ch: ch}
}

// ChanNonBlockingReceive returns a non-blocking wrapper for <-chan T
// objects, for which read operations will return early when reading
// from an empty channel.
func ChanNonBlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: modeNonBlocking, ch: ch}
}

// Iterator provides access to the contents of the channel as a
// new-style standard library iterator. For ChanRecieve objects in
// non-blocking mode, iteration ends when there are no items in the
// channel. In blocking mode, iteration ends when the context is
// canceled or the channel is closed.
func (ro ChanReceive[T]) Iterator(ctx context.Context) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			value, err := ro.Read(ctx)
			switch {
			case err != nil && errors.Is(err, ers.ErrCurrentOpSkip):
				continue
			case err != nil:
				return
			case !yield(value):
				return
			}
		}
	}
}

// Drop performs a read operation and drops the response. If an item
// was dropped (e.g. Read would return an error), Drop() returns
// false, and true when the Drop was successful.
func (ro ChanReceive[T]) Drop(ctx context.Context) bool { _, ok := ro.Check(ctx); return ok }

// Ignore reads one item from the channel and discards it.
func (ro ChanReceive[T]) Ignore(ctx context.Context) { _, _ = ro.Read(ctx) }

// Force ignores the error returning only the value from Read. This is
// either the value sent through the channel, or the zero value for
// T. Because zero values can be sent through channels, Force does not
// provide a way to distinguish between "channel-closed" and "received
// a zero value".
func (ro ChanReceive[T]) Force(ctx context.Context) (out T) { out, _ = ro.Read(ctx); return }

// Check performs the read operation and converts the error into an
// "ok" value, returning true if receive was successful and false
// otherwise.
func (ro ChanReceive[T]) Check(ctx context.Context) (T, bool) {
	out, err := ro.Read(ctx)
	return out, err == nil
}

// Ok attempts to read from a channel returns false when the channel has been closed and true
// otherwise.
func (ro ChanReceive[T]) Ok() bool {
	switch ro.mode {
	case modeBlocking:
		_, ok := <-ro.ch
		return ok
	case modeNonBlocking:
		select {
		case _, ok := <-ro.ch:
			return ok
		default:
			return true
		}
	default:
		panic(errors.Join(ers.ErrInvariantViolation, ers.Error("unreachable")))
	}
}

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
func (ro ChanReceive[T]) Read(ctx context.Context) (T, error) {
	var zero T
	switch ro.mode {
	case modeBlocking:
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case obj, ok := <-ro.ch:
			if !ok {
				return zero, io.EOF
			}

			return obj, nil
		}
	case modeNonBlocking:
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case obj, ok := <-ro.ch:
			if !ok {
				return zero, io.EOF
			}

			return obj, nil
		default:
			return zero, ers.ErrCurrentOpSkip
		}
	default:
		panic(errors.Join(ers.ErrInvariantViolation, ers.Error("unreachable")))
	}
}

////////////////////////////////////////////////////////////////////////////////
//
// SEND

// ChanSend provides access to channel send operations, and is
// contstructed by the ChanSend() method on the channel operation. The
// primary method is Write(), with other methods provided for clarity.
type ChanSend[T any] struct {
	mode blockingMode
	ch   chan<- T
}

// BlockingSend returns a Chan wrapper for chan<- T operation, that
// will block when sending into a channel that is full or that doesn't
// have a listener.
func BlockingSend[T any](ch chan<- T) ChanSend[T] {
	return ChanSend[T]{mode: modeBlocking, ch: ch}
}

// NonBlockingSend returns a non-blocking wrapper for <-chan T
// objects, for which read operations will return early when reading
// from an empty channel.
func NonBlockingSend[T any](ch chan<- T) ChanSend[T] {
	return ChanSend[T]{mode: modeNonBlocking, ch: ch}
}

// Check performs a send and returns true when the send was successful
// and false otherwise.
func (sm ChanSend[T]) Check(ctx context.Context, it T) bool { return sm.Write(ctx, it) == nil }

// Ignore performs a send and omits the error.
func (sm ChanSend[T]) Ignore(ctx context.Context, it T) { _ = sm.Check(ctx, it) }

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
			return ers.ErrCurrentOpSkip
		}
	default:
		panic(errors.Join(ers.ErrInvariantViolation, ers.Error("unreachable")))
	}
}
