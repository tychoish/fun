package erc

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
)

// When is a helper function, typically useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func When(ec *Collector, cond bool, val any) { ec.Add(ers.When(cond, val)) }

// Whenf conditionally creates and adds an error to the collector, as
// When, and with a similar use case, but permits Sprintf/Errorf
// formating.
func Whenf(ec *Collector, cond bool, val string, args ...any) { ec.Add(ers.Whenf(cond, val, args...)) }

// WithRecoverDo runs the provided function, and catches a panic,
// if necessary and adds that panic to the collector. If there is no
// panic, The return value is the return value of the provided
// function.
func WithRecoverDo[T any](ec *Collector, fn fn.Future[T]) T { defer Recover(ec); return fn() }

// WithRecoverCall calls the provided function and returns its output
// to the caller.
func WithRecoverCall(ec *Collector, fn func()) { defer Recover(ec); fn() }

// Recover calls the builtin recover() function and converts it to an
// error that is populated in the collector. Run RecoverHook in defer
// statements.
func Recover(ec *Collector) { ec.Add(ers.ParsePanic(recover())) }

// RecoverHook runs adds the output of recover() to the error
// collector, and runs the specified hook if. If there was no panic,
// this function is a noop. Run RecoverHook in defer statements.
func RecoverHook(ec *Collector, hook func()) { ec.AddWithHook(ers.ParsePanic(recover()), hook) }

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func Check(ec *Collector, fut fn.Future[error]) { ec.Add(fut.Resolve()) }

// PopulateFromChannel collects all errors from an error channel, and returns the
// aggregated error. PopulateFromChannel blocks until the context expires (but
// does not add a context cancellation error) or the error channel is
// closed.
func PopulateFromChannel(ctx context.Context, ec *Collector, errCh <-chan error) {
	fun.ChannelStream(errCh).Observe(ec.Handler()).Operation(ec.Add).Run(ctx)
}

// Collect produces a function that will collect the error from a
// function and add it to the collector returning the result. Use
// this, like risky.Force to delay handling an error while also avoiding
// declaring an extra error variable, without dropping the error
// entirely.
//
// For example:
//
//	func actor(conf Configuration) (int, error) { return 42, nil}
//
//	func main() {
//	    ec := &erc.Collector{}
//	    resolveSize := erc.Collect[int](ec)
//	    size := resolveSize(actor(Configuration{}))
//	}
func Collect[T any](ec *Collector) func(T, error) T {
	return func(out T, err error) T { ec.Add(err); return out }
}

// StreamHook adds errors to the Stream's error collector from the
// provided Collector when the stream closes.
//
//	ec := &Collector{}
//	stream := fun.Stream[int]
//	stream.WithHook(erc.StreamHook(ec))
func StreamHook[T any](ec *Collector) fn.Handler[*fun.Stream[T]] {
	return func(it *fun.Stream[T]) { it.AddError(ec.Resolve()) }
}
