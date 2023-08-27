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
const ErrNonBlockingChannelOperationSkipped ers.Error = ers.ErrCurrentOpSkip

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

// Chan constructs a channel op, like "make(chan T)", with the
// optionally specified length. The underlying channel is blocking by
// default. Uses the Blocking() and NonBlocking methods on the channel
// operation to access versions
func Chan[T any](args ...int) ChanOp[T] {
	switch len(args) {
	case 0:
		return Blocking(make(chan T))
	case 1:
		return Blocking(make(chan T, args[0]))
	default:
		panic(ers.Wrap(ers.ErrInvariantViolation, "cannot specify >2 arguments to make() for a slice"))
	}
}

// DefaultChan takes a channel value and if it is non-nil, returns it;
// otherwise it constructs a new ChanOp of the specified type with the
// optionally provided length and returns it.
func DefaultChan[T any](input chan T, args ...int) ChanOp[T] {
	if input != nil {
		return ChanOp[T]{ch: input}
	}

	return Chan[T](args...)
}

// Blocking produces a blocking Send instance. All Send/Check/Ignore
// operations will block until the context is canceled, the channel is
// canceled, or the send succeeds.
func Blocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: modeBlocking, ch: ch} }

// NonBlocking produces a send instance that performs a non-blocking
// send.
//
// The Send() method, for non-blocking sends, will return
// ErrSkipedNonBlockingSend if the channel was full and the object was
// not sent.
func NonBlocking[T any](ch chan T) ChanOp[T] { return ChanOp[T]{mode: modeNonBlocking, ch: ch} }

// Close closes the underlying channel.
func (op ChanOp[T]) Close() { close(op.ch) }

// Blocking returns a version of the ChanOp in blocking mode.
func (op ChanOp[T]) Blocking() ChanOp[T] { op.mode = modeBlocking; return op }

// NonBlocking returns a version of the ChanOp in non-blocking mode.
func (op ChanOp[T]) NonBlocking() ChanOp[T] { op.mode = modeNonBlocking; return op }

// Channel returns the underlying channel.
func (op ChanOp[T]) Channel() chan T { return op.ch }

// Send returns a ChanSend object that acts on the same underlying
// channel
func (op ChanOp[T]) Send() ChanSend[T] { return ChanSend[T]{mode: op.mode, ch: op.ch} }

// Receive returns a ChanReceive object that acts on the same
// underlying sender.
func (op ChanOp[T]) Receive() ChanReceive[T] { return ChanReceive[T]{mode: op.mode, ch: op.ch} }

// Iterator returns the "receive" aspect of the channel as an
// iterator. This is equivalent to fun.ChannelIterator(), but may be
// more accessible in some contexts.
func (op ChanOp[T]) Iterator() *Iterator[T] { return op.Receive().Producer().Iterator() }

// Processor exposes the "send" aspect of the channel as a Processor function.
func (op ChanOp[T]) Processor() Processor[T] { return op.Send().Processor() }

// Producer expoess the "receive" aspect of the channel as a Producer function.
func (op ChanOp[T]) Producer() Producer[T] { return op.Receive().Producer() }

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

// Filter returns a channel that consumes the output of a channel and
// returns a NEW channel that only contains elements that have
// elements that the filter function returns true for.
func (ro ChanReceive[T]) Filter(ctx context.Context, eh Handler[error], filter func(T) bool) ChanReceive[T] {
	out := ChanOp[T]{ch: make(chan T), mode: ro.mode}

	ro.Producer().Filter(filter).SendAll(out.Processor()).
		PreHook(Operation(func(ctx context.Context) { <-ctx.Done(); out.Close() }).Once().Launch(ctx)).
		Background(ctx, eh)

	return out.Receive()
}

// BlockingReceive is the equivalent of Blocking(ch).Receive(), except
// that it accepts a receive-only channel.
func BlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: modeBlocking, ch: ch}
}

// NonBlockingReceive is the equivalent of NonBlocking(ch).Receive(),
// except that it accepts a receive-only channel.
func NonBlockingReceive[T any](ch <-chan T) ChanReceive[T] {
	return ChanReceive[T]{mode: modeNonBlocking, ch: ch}
}

// Drop performs a read operation and drops the response. If an item
// was dropped (e.g. Read would return an error), Drop() returns
// false, and true when the Drop was successful.
func (ro ChanReceive[T]) Drop(ctx context.Context) bool { return ft.IsOK(ro.Producer().Check(ctx)) }

// Ignore reads one item from the channel and discards it.
func (ro ChanReceive[T]) Ignore(ctx context.Context) { ro.Producer().Ignore(ctx).Resolve() }

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

// OK attempts to read from a channel returns true either when the
// channel is blocked or an item is read from the channel and false
// when the channel has been closed.
func (ro ChanReceive[T]) OK() bool {
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
func (ro ChanReceive[T]) Producer() Producer[T] { return ro.Read }

// Iterator expoeses aspects to the contents of the channel as an
// iterator.
func (ro ChanReceive[T]) Iterator() *Iterator[T] { return ro.Producer().Iterator() }

// Consume returns a Worker function that processes the output of data
// from the channel with the Processor function. If the processor
// function returns ErrIteratorSkip, the processing will continue. All
// other Processor errors (and problems reading from the channel,)
// abort iterator. io.EOF errors are not propagated to the caller.
func (ro ChanReceive[T]) Consume(op Processor[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(err, ers.ParsePanic(recover())) }()

		var value T
	LOOP:
		for {
			value, err = ro.Read(ctx)
			if err != nil {
				goto HANDLE_ERROR
			}
			if err = op(ctx, value); err != nil {
				goto HANDLE_ERROR
			}

		HANDLE_ERROR:
			switch {
			case err == nil:
				continue LOOP
			case errors.Is(err, io.EOF):
				return nil
			case errors.Is(err, ErrIteratorSkip):
				continue LOOP
			default:
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
func BlockingSend[T any](ch chan<- T) ChanSend[T] { return ChanSend[T]{mode: modeBlocking, ch: ch} }

// NonBlockingSend is equivalent to NonBlocking(ch).Send() except that
// it accepts a send-only channel.
func NonBlockingSend[T any](ch chan<- T) ChanSend[T] {
	return ChanSend[T]{mode: modeNonBlocking, ch: ch}
}

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

// Consume returns a worker that, when executed, pushes the content
// from the iterator into the channel.
func (sm ChanSend[T]) Consume(iter *Iterator[T]) Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = ers.Join(iter.Close(), err, ers.ParsePanic(recover())) }()
		return iter.Process(sm.Processor()).Run(ctx)
	}
}
