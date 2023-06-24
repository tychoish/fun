package fun

import (
	"context"
	"errors"
	"io"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// ErrNonBlockingChannelOperationSkipped is returned when sending into
// a channel, in a non-blocking context, when the channel was full and
// the send or receive was therefore skipped.
const ErrNonBlockingChannelOperationSkipped ers.Error = ers.Error("non-blocking channel operation skipped")

// blockingMode provides named constants for blocking/non-blocking
// operations. They are fully internal, and only used indirectly.
type blockingMode int8

const (
	blocking     blockingMode = 1
	non_blocking blockingMode = 2
)

// ChanOp is a wrapper around a channel, to make it easier to write
// clear code that uses and handles basic operations with single
// channels. From a high level an operation might look like:
//
//	ch := make(chan string)
//	err := fun.Blocking().Send()
type ChanOp[T any] struct {
	mode blockingMode
	ch   chan T
}

// Blocking produces a blocking Send instance. All Send/Check/Ignore
// operations will block until the context is canceled, the channel is
// canceled, or the send succeeds.
func Blocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: blocking, ch: ch} }

// NonBlocking produces a send instance that performs a non-blocking
// send.
//
// The Send() method, for non-blocking sends, will return
// ErrSkipedNonBlockingSend if the channel was full and the object was
// not sent.
func NonBlocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: non_blocking, ch: ch} }

func (op ChanOp[T]) Close()                  { close(op.ch) }
func (op ChanOp[T]) Channel() chan T         { return op.ch }
func (op ChanOp[T]) Send() ChanSend[T]       { return ChanSend[T]{mode: op.mode, ch: op.ch} }
func (op ChanOp[T]) Receive() ChanReceive[T] { return ChanReceive[T]{mode: op.mode, ch: op.ch} }
func (op ChanOp[T]) Iterator() *Iterator[T]  { return op.Receive().Producer().Iterator() }

func (op ChanOp[T]) Processor() Processor[T] { return op.Send().Processor() }
func (op ChanOp[T]) Producer() Producer[T]   { return op.Receive().Producer() }

// Pipe creates a linked pair of functions for transmitting data via
// these interfaces.
func (op ChanOp[T]) Pipe() (Processor[T], Producer[T]) {
	return op.Processor(), op.Producer()
}

// ChanReceive, wraps a channel fore <-chan T operations. It is the type
// returned by the ChanReceive() method on ChannelOp. The primary method
// is Read(), with other methods provided as "self-documenting"
// helpers.
type ChanReceive[T any] struct {
	mode blockingMode
	ch   <-chan T
}

// BlockingReceive is the equivalent of Blocking(ch).Receive(), except
// that it accepts a receive-only channel.
func BlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: blocking, ch: ch}
}

// NonBlockingReceive is the equivalent of NonBlocking(ch).Receive(),
// except that it accepts a receive-only channel.
func NonBlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: non_blocking, ch: ch}
}

// Drop performs a read operation and drops the response. If an item
// was dropped (e.g. Read would return an error), Drop() returns
// false, and true when the Drop was successful.
func (ro ChanReceive[T]) Drop(ctx context.Context) bool { return ft.IsOK(ro.Producer().Check(ctx)) }

// Ignore reads one item from the channel and discards it.
func (ro ChanReceive[T]) Ignore(ctx context.Context) { ro.Producer().Ignore(ctx) }

// Force ignores the error returning only the value from Read. This is
// either the value sent through the channel, or the zero value for
// T. Because zero values can be sent through channels, Force does not
// provide a way to distinguish between "channel-closed" and "received
// a zero value".
func (ro ChanReceive[T]) Force(ctx context.Context) (out T) { out, _ = ro.Read(ctx); return }

// Check performs the read operation and converts the error into an
// "ok" value, returning true if receive was successful and false
// otherwise.
func (ro ChanReceive[T]) Check(ctx context.Context) (T, bool) { return ro.Producer().Check(ctx) }

// Ok attempts to read from a channel returns true either when the
// channel is blocked or an item is read from the channel and false
// when the channel has been closed.
func (ro ChanReceive[T]) Ok() bool {
	switch ro.mode {
	case blocking:
		_, ok := <-ro.ch
		return ok
	case non_blocking:
		select {
		case _, ok := <-ro.ch:
			return ok
		default:
			return true
		}
	default:
		// should be impossible outside of the package,
		panic(ErrInvariantViolation)
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
	case blocking:
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case obj, ok := <-ro.ch:
			if !ok {
				return zero, io.EOF
			}

			return obj, nil
		}
	case non_blocking:
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case obj, ok := <-ro.ch:
			if !ok {
				return zero, io.EOF
			}

			return obj, nil
		default:
			return zero, ErrNonBlockingChannelOperationSkipped
		}
	default:
		// this is impossible without an invalid blockingMode
		// value
		return zero, io.EOF
	}
}

// Producer returns the Read method as a producer for integration into
// existing tools.
func (ro ChanReceive[T]) Producer() Producer[T]  { return ro.Read }
func (ro ChanReceive[T]) Iterator() *Iterator[T] { return ro.Producer().Iterator() }

func (ro ChanReceive[T]) Consume(op Processor[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()

		var value T
		for {
			value, err = ro.Read(ctx)
			if err != nil {
				return err
			}
			if err = op(ctx, value); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}
}

// ChanSend provides access to channel send operations, and is
// contstructed by the ChanSend() method on the channel operation. The
// primary method is Write(), with other methods provided for clarity.
type ChanSend[T any] struct {
	mode blockingMode
	ch   chan<- T
}

// BlockingSend is equivalent to Blocking(ch).Send() except that
// it accepts a send-only channel.
func BlockingSend[T any](ch chan<- T) ChanSend[T] { return ChanSend[T]{mode: blocking, ch: ch} }

// NonBlockingSend is equivalent to NonBlocking(ch).Send() except that
// it accepts a send-only channel.
func NonBlockingSend[T any](ch chan<- T) ChanSend[T] { return ChanSend[T]{mode: non_blocking, ch: ch} }

// Check performs a send and returns true when the send was successful
// and false otherwise.
func (sm ChanSend[T]) Check(ctx context.Context, it T) bool { return sm.Processor().Check(ctx, it) }

// Ignore performs a send and omits the error.
func (sm ChanSend[T]) Ignore(ctx context.Context, it T) { sm.Processor().Ignore(ctx, it) }

// Processor returns the Write method as a processor for integration
// into existing tools
func (sm ChanSend[T]) Processor() Processor[T] { return sm.Write }

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
			return ErrNonBlockingChannelOperationSkipped
		}
	default:
		// it should be impossible to provoke an EOF error
		// outside of this project, because you'd need to
		// construct an invalid Send object.
		return io.EOF
	}
}

func (sm ChanSend[T]) Consume(iter *Iterator[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(iter.Close(), err, ers.ParsePanic(recover())) }()
		return iter.Process(ctx, sm.Processor())
	}
}
