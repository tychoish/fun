package fun

import (
	"context"
	"iter"

	"github.com/tychoish/fun/ers"
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
// optionally specified length. Operations (like read from and write
// to a channel) on the channel are blocking by default, but the
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
//
// This swallows any panic encountered when calling close() on the
// underlying channel, which makes it safe to call on nil or
// already-closed channels: the result in all cases (that the channel
// is closed when Close() returns, is the same in all cases.)
func (op ChanOp[T]) Close() {
	defer func() { _ = recover() }()
	close(op.ch)
}

// Len returns the current length of the channel.
func (op ChanOp[T]) Len() int { return len(op.ch) }

// Cap returns the current capacity of the channel.
func (op ChanOp[T]) Cap() int { return cap(op.ch) }

// Blocking returns a version of the ChanOp in blocking mode. This is
// not an atomic operation.
func (op ChanOp[T]) Blocking() ChanOp[T] { op.mode = modeBlocking; return op }

// NonBlocking returns a version of the ChanOp in non-blocking mode.
// This is not an atomic operation.
func (op ChanOp[T]) NonBlocking() ChanOp[T] { op.mode = modeNonBlocking; return op }

// Channel returns the underlying channel.
func (op ChanOp[T]) Channel() chan T { return op.ch }

// Send returns a ChanSend object that acts on the same underlying
// channel
func (op ChanOp[T]) Send() ChanSend[T] { return ChanSend[T]{mode: op.mode, ch: op.ch} }

// Receive returns a ChanReceive object that acts on the same
// underlying sender.
func (op ChanOp[T]) Receive() ChanReceive[T] { return ChanReceive[T]{mode: op.mode, ch: op.ch} }

// Stream returns the "receive" aspect of the channel as an
// stream. This is equivalent to fun.ChannelStream(), but may be
// more accessible in some contexts.
func (op ChanOp[T]) Stream() *Stream[T] { return op.Receive().Stream() }

func (op ChanOp[T]) Iterator(ctx context.Context) iter.Seq[T] { return op.Receive().Iterator(ctx) }
func (op ChanOp[T]) Seq2(ctx context.Context) iter.Seq2[int, T] {
	return op.Receive().IteratorIndexed(ctx)
}

// Handler exposes the "send" aspect of the channel as a Handler function.
func (op ChanOp[T]) Handler() Handler[T] { return op.Send().Handler() }

// Generator expoess the "receive" aspect of the channel as a Generator function.
func (op ChanOp[T]) Generator() Generator[T] { return op.Receive().Generator() }

// Pipe creates a linked pair of functions for transmitting data via
// these interfaces.
func (op ChanOp[T]) Pipe() (Handler[T], Generator[T]) { return op.Handler(), op.Generator() }
