package erc

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// ConstErr is a type alias for building/declaring sentinel errors
// as constants.
type ConstErr string

// Error implements the error interface for ConstError.
func (e ConstErr) Error() string { return string(e) }

// ContextExpired checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func ContextExpired(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Wrap produces a wrapped error if the err is non-nil, wrapping the
// error with the provided annotation. When the error is nil, Wrap
// returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrap" but
// taking advantage of newer standard library error wrapping.
func Wrap(err error, annotation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", annotation, err)
}

// Wrapf produces a wrapped error, if the error is non-nil, with a
// formated wrap annotation. When the error is nil, Wrapf does not
// build an error and returns nil.
//
// This, roughly mirrors the usage "github/pkg/errors.Wrapf" but
// taking advantage of newer standard library error wrapping.
func Wrapf(err error, tmpl string, args ...any) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", fmt.Sprintf(tmpl, args...), err)
}

// When is a helper function, typcially useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func When(ec *Collector, cond bool, val any) {
	if !cond {
		return
	}
	switch e := val.(type) {
	case error:
		ec.Add(e)
	case string:
		ec.Add(errors.New(e))
	default:
		ec.Add(fmt.Errorf("error=%T: %v", val, e))
	}
}

// Whenf conditionally creates and adds an error to the collector, as
// When, and with a similar use case, but permits Sprintf/Errorf
// formating.
func Whenf(ec *Collector, cond bool, val string, args ...any) {
	if !cond {
		return
	}

	ec.Add(fmt.Errorf(val, args...))
}

// Safe provides similar semantics fun.Safe, but creates an error from
// the panic value and adds it to the error collector.
func Safe[T any](ec *Collector, fn func() T) T { defer Recover(ec); return fn() }

// Recover calls the builtin recover() function and converts it to an
// error that is populated in the collector. Run RecoverHook in defer
// statements.
func Recover(ec *Collector) { ec.Add(fun.ParsePanic(recover())) }

// Recovery catches a panic, turns it into an error and passes it to
// the provided observer function.
func Recovery(ob fun.Observer[error]) { ob(fun.ParsePanic(recover())) }

// RecoverHook runs adds the output of recover() to the error
// collector, and runs the specified hook if. If there was no panic,
// this function is a noop. Run RecoverHook in defer statements.
func RecoverHook(ec *Collector, hook func()) {
	if r := recover(); r != nil {
		switch err := r.(type) {
		case error:
			ec.Add(err)
		default:
			ec.Add(fmt.Errorf("%v", err))
		}

		ec.Add(fun.ErrRecoveredPanic)

		if hook != nil {
			hook()
		}
	}
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func Check(ec *Collector, fn func() error) { ec.Add(fn()) }

// Checkf executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func Checkf(ec *Collector, fn func() error, tmpl string, args ...any) {
	ec.Add(Wrapf(fn(), tmpl, args...))
}

// CheckWhen executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func CheckWhen(ec *Collector, cond bool, fn func() error) {
	if cond {
		ec.Add(fn())
	}
}

// Unwind converts an error into a slice of errors in two cases:
// First, if an error is an *erc.Stack, Unwind will return a slice
// with all constituent errors. Second, if the error is wrapped or
// merged, Unwind will unwrap the error object adding every
// intermediate error.
func Unwind(err error) []error {
	if err == nil {
		return nil
	}

	var out []error
	switch e := err.(type) {
	case *Stack:
		// the only way this can error is if the observer
		// function panics, which it can't:
		fun.InvariantMust(e.Iterator().Observe(internal.BackgroundContext, func(err error) { out = append(out, err) }))
	default:
		out = fun.Unwind(err)
	}

	return out
}

// Merge produces a single error from two input errors. The output
// error behaves correctly for errors.Is and errors.As and Unwrap()
// calls, for both errors, checked in order.
//
// If both errors are of the same root type and you investigate the
// output error with errors.As, the first error's value will be used.
func Merge(err1, err2 error) error { return internal.MergeErrors(err1, err2) }

// Collapse takes a slice of errors and converts it into an *erc.Stack
// typed error.
func Collapse(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	ec := &Collector{}

	for idx := range errs {
		ec.Add(errs[idx])
	}

	return ec.Resolve()
}

// Stream collects all errors from an error channel, and returns the
// aggregated error. Stream blocks until the context expires (but
// does not add a context cancellation error) or the error channel is
// closed.
//
// Because Stream() is a fun.ProcessFunc you can convert this into
// fun.Worker and fun.Operation objects as needed.
func Stream(ctx context.Context, errCh <-chan error) error {
	return Consume(ctx, fun.ChannelIterator(errCh))
}

// Consume iterates through all errors in the fun.Iterator and
// returning the aggregated (*erc.Stack) error for these errors.
//
// Because Consume() is a fun.ProcessFunc you can convert this into
// fun.Worker and fun.Operation objects as needed.
func Consume(ctx context.Context, iter *fun.Iterator[error]) error {
	ec := &Collector{}
	ec.Add(iter.Observe(ctx, ec.Observer()))
	return ec.Resolve()
}

// Collect produces a function that will collect the error from a
// function and add it to the collector returning the result. Use
// this, like fun.Must to delay handling an error while also avoiding
// declaring an extra error variable, without dropping the error
// entirely.
//
// For example:
//
//	func actor(conf Configuration) (int, error) { return 42, nil}
//
//	func main() {
//	    ec := &erc.Collector{}
//	    size := erc.Collect[int](ec)(actor(Configuration{}))
//	}
func Collect[T any](ec *Collector) func(T, error) T {
	return func(out T, err error) T { ec.Add(err); return out }
}

func IteratorHook[T any](ec *Collector) fun.Observer[*fun.Iterator[T]] {
	return func(it *fun.Iterator[T]) { it.AddError(ec.Resolve()) }
}
