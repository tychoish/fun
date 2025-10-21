package fun

import (
	"context"
	"errors"
	"io"
	"iter"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
)

// ChanReceive wraps a channel fore <-chan T operations. It is the type
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
func (ro ChanReceive[T]) Filter(ctx context.Context, filter func(T) bool) ChanReceive[T] {
	out := ChanOp[T]{ch: make(chan T), mode: ro.mode}

	MakeStream(ro.Read).
		WithHook(func(*Stream[T]) { out.Close() }).
		Filter(filter).
		Parallel(out.Send().Write,
			WorkerGroupConfNumWorkers(1),
			WorkerGroupConfIncludeContextErrors(),
		).
		Ignore().
		Background(ctx)

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
func (ro ChanReceive[T]) Drop(ctx context.Context) bool { return ft.IsOk(ro.Check(ctx)) }

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
func (ro ChanReceive[T]) Check(ctx context.Context) (T, bool) { return ft.Check(ro.Read(ctx)) }

// Ok attempts to read from a channel returns true either when the
// channel is blocked or an item is read from the channel and false
// when the channel has been closed.
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
		// should be impossible outside of the package,
		panic(ers.ErrInvariantViolation)
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

// Stream provides access to the contents of the channel as a
// fun-style stream. For ChanRecieve objects in
// non-blocking mode, iteration ends when there are no items in the
// channel. In blocking mode, iteration ends when the context is
// canceled or the channel is closed.
func (ro ChanReceive[T]) Stream() *Stream[T] { return MakeStream(ro.Read) }

// Iterator provides access to the contents of the channel as a
// new-style standard library stream. For ChanRecieve objects in
// non-blocking mode, iteration ends when there are no items in the
// channel. In blocking mode, iteration ends when the context is
// canceled or the channel is closed.
func (ro ChanReceive[T]) Iterator(ctx context.Context) iter.Seq[T] { return ro.Stream().Iterator(ctx) }

// ReadAll returns a Worker function that processes the output of data
// from the channel with the Handler function. If the processor
// function returns ers.ErrCurrentOpSkip, the processing will continue. All
// other Handler errors (and problems reading from the channel,)
// abort stream. io.EOF errors are not propagated to the caller.
func (ro ChanReceive[T]) ReadAll(op func(context.Context, T) error) fnx.Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()

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
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue LOOP
			default:
				return err
			}
		}
	}
}
