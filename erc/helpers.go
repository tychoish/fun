package erc

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// ContextExpired checks an error to see if it, or any of it's parent
// contexts signal that a context has expired. This covers both
// canceled contexts and ones which have exceeded their deadlines.
func ContextExpired(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// When is a helper function, typcially useful for improving the
// readability of validation code. If the condition is true, then When
// creates an error with the string value and adds it to the Collector.
func When(ec *Collector, cond bool, val string) {
	if !cond {
		return
	}

	ec.Add(errors.New(val))
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
func Recover(ec *Collector) {
	if r := recover(); r != nil {
		switch err := r.(type) {
		case error:
			ec.Add(fmt.Errorf("panic: %w", err))
		default:
			ec.Add(fmt.Errorf("panic: %v", err))
		}
	}
}

// RecoverHook runs adds the output of recover() to the error
// collector, and runs the specified hook if. If there was no panic,
// this function is a noop. Run RecoverHook in defer statements.
func RecoverHook(ec *Collector, hook func()) {
	if r := recover(); r != nil {
		switch err := r.(type) {
		case error:
			ec.Add(fmt.Errorf("panic: %w", err))
		default:
			ec.Add(fmt.Errorf("panic: %v", err))
		}

		if hook != nil {
			hook()
		}
	}
}

// CheckCtx executes a simple function that takes a context and if it
// returns an error, adds it to the collector, primarily for use in
// defer statements.
func CheckCtx(ctx context.Context, ec *Collector, fn func(context.Context) error) {
	CheckWait(ec, fn)(ctx)
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func Check(ec *Collector, fn func() error) { ec.Add(fn()) }

// CheckWait returns a fun.WaitFunc for a function that returns an
// error, with the error consumed by the collector.
func CheckWait(ec *Collector, fn func(context.Context) error) fun.WaitFunc {
	return func(ctx context.Context) { ec.Add(fn(ctx)) }
}

// Unwind converts an error into a slice of errors in two cases:
// First, if an error is an *erc.Stack, Unwind will return a slice
// with all constituent errors. Second, if the error is wrapped,
// Unwind will unwrap the error object adding every intermediate
// error.
func Unwind(err error) []error {
	if err == nil {
		return nil
	}

	out := []error{}
	switch e := err.(type) {
	case *Stack:
		iter := e.Iterator()
		fun.Observe(internal.BackgroundContext, iter, func(err error) {
			out = append(out, err)
		})
	default:
		out = append(out, err)
		for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
			// the first error might not be unwrappable
			if e != nil {
				out = append(out, e)
			}
		}
	}

	return out
}

// Merge produces a single error from two input errors. The output
// error behaves correctly for errors.Is and errors.As and Unwrap()
// calls, for both errors, checked in order.
//
// If both errors are of the same root type and you investigate the
// output error with errors.As, the first error's value will be used.
func Merge(err1, err2 error) error {
	switch {
	case err1 == nil && err2 == nil:
		return nil
	case err1 == nil && err2 != nil:
		return err2
	case err1 != nil && err2 == nil:
		return err1
	default:
		return &internal.MergedError{Current: err1, Wrapped: err2}
	}
}

// Collapse takes a slice of errors and converts it into an *erc.Stack
// typed error.
func Collapse(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	ec := &Collector{}

	CollapseFrom(ec, errs)

	return ec.Resolve()
}

// CollapseInto is a helper that has the same semantics and calling
// pattern as Collapse, but collapses those errors into the provided
// Collector.
func CollapseInto(ec *Collector, errs ...error) { CollapseFrom(ec, errs) }

// CollapseFrom is a helper that adds a slice of errors to the
// provided Collector.
func CollapseFrom(ec *Collector, errs []error) {
	for idx := range errs {
		ec.Add(errs[idx])
	}
}

// Stream collects all errors from an error channel, and returns the
// aggregated error. Stream blocks until the context expires (but
// does not add a context cancellation error) or the error channel is
// closed.
func Stream(ctx context.Context, errCh <-chan error) error {
	return Consume(ctx, &internal.ChannelIterImpl[error]{Pipe: errCh})
}

// StreamAll collects all errors from an error channel and adds them
// to the provided collector. StreamAll returns a fun.WaitFunc that
// blocks the error channel is closed or its context is canceled.
func StreamAll(ec *Collector, errCh <-chan error) fun.WaitFunc {
	return fun.WaitObserveAll(ec.Add, errCh)
}

// StreamOne returns a fun.WaitFunc that blocks until a single
// error is set to the error channel and adds that to the
// collector.
func StreamOne(ec *Collector, errCh <-chan error) fun.WaitFunc {
	return fun.WaitObserve(ec.Add, errCh)
}

// StreamProcess is a non-blocking helper that starts a background
// goroutine to process the contents of an error channel. The function
// returned will block until the channel is closed or the context is
// canceled and can be used to wait for the background operation to be
// complete.
func StreamProcess(ctx context.Context, ec *Collector, errCh <-chan error) fun.WaitFunc {
	return ConsumeProcess(ctx, ec, &internal.ChannelIterImpl[error]{Pipe: errCh})
}

// Consume iterates through all errors in the fun.Iterator and
// returning the aggregated (*erc.Stack) error for these errors.
func Consume(ctx context.Context, iter fun.Iterator[error]) error {
	ec := &Collector{}
	ConsumeAll(ec, iter)(ctx)
	return ec.Resolve()
}

// ConsumeAll adds all errors in the input iterator and returns a wait
// function that blocks until the iterator is exhausted. ConsumeAll
// does not begin processing the iterator until the wait function is called.
func ConsumeAll(ec *Collector, iter fun.Iterator[error]) fun.WaitFunc {
	return func(ctx context.Context) { fun.Observe(ctx, iter, ec.Add) }
}

// ConsumeProcess adds all errors in the iterator to the
// provided collector and returns a wait function that blocks until
// the iterator has been exhausted. This error processing work happens
// in a different go routine, and the fun.WaitFunc blocks until this
// goroutine has returned.
func ConsumeProcess(ctx context.Context, ec *Collector, iter fun.Iterator[error]) fun.WaitFunc {
	sig := make(chan struct{})

	go func() {
		defer close(sig)
		defer Recover(ec)
		ConsumeAll(ec, iter)(ctx)
	}()

	return fun.WaitChannel(sig)
}
