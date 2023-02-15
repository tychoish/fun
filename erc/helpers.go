package erc

import (
	"context"
	"errors"
	"fmt"
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
func Safe[T any](ec *Collector, fn func() T) T {
	defer Recover(ec)
	return fn()
}

// Recover calls the builtin recover() function and converts it to an
// error that is populated in the collector. Run RecoverHook in defer
// statements.
func Recover(ec *Collector) {
	if r := recover(); r != nil {
		ec.Add(fmt.Errorf("panic: %v", r))
	}
}

// RecoverHook runs adds the output of recover() to the error
// collector, and runs the specified hook if. If there was no panic,
// this function is a noop. Run RecoverHook in defer statements.
func RecoverHook(ec *Collector, hook func()) {
	if r := recover(); r != nil {
		ec.Add(fmt.Errorf("panic: %v", r))
		if hook != nil {
			hook()
		}
	}
}

// CheckCtx executes a simple function that takes a context and if it
// returns an error, adds it to the collector, primarily for use in
// defer statements.
func CheckCtx(ctx context.Context, ec *Collector, fn func(context.Context) error) {
	Check(ec, func() error { return fn(ctx) })
}

// Check executes a simple function and if it returns an error, adds
// it to the collector, primarily for use in defer statements.
func Check(ec *Collector, fn func() error) { ec.Add(fn()) }

var internalIterContext = context.Background()

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
	if es, ok := err.(*Stack); ok {
		iter := es.Iterator()
		for iter.Next(internalIterContext) {
			out = append(out, iter.Value())
		}
		return out
	}

	out = append(out, err)
	for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
		out = append(out, e)
	}

	return out
}
