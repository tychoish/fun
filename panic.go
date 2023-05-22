package fun

import (
	"context"
	"errors"
	"fmt"

	"github.com/tychoish/fun/internal"
)

var (
	// ErrInvariantViolation is the root error of the error object that is
	// the content of all panics produced by the Invariant helper.
	ErrInvariantViolation = errors.New("invariant violation")
	// ErrRecoveredPanic is at the root of any error returned by a
	// function in the fun package that recovers from a panic.
	ErrRecoveredPanic = errors.New("recovered panic")
)

// Invariant panics if the condition is false Invariant panics,
// passing an error that is rooted by ErrInvariantViolation.
func Invariant(cond bool, args ...any) {
	if !cond {
		switch len(args) {
		case 0:
			panic(ErrInvariantViolation)
		case 1:
			switch ei := args[0].(type) {
			case error:
				panic(&internal.MergedError{Current: ei, Wrapped: ErrInvariantViolation})
			case string:
				panic(&internal.MergedError{Current: errors.New(ei), Wrapped: ErrInvariantViolation})
			default:
				panic(fmt.Errorf("[%v]: %w", args[0], ErrInvariantViolation))
			}
		default:
			if err, ok := args[0].(error); ok {
				panic(&internal.MergedError{
					Current: fmt.Errorf("[%s]", args[1:]),
					Wrapped: &internal.MergedError{Current: err, Wrapped: ErrInvariantViolation},
				})
			}
			panic(fmt.Errorf("[%s]: %w", fmt.Sprintln(args...), ErrInvariantViolation))
		}
	}
}

// InvariantMust raises an invariant error if the error is not
// nil. The content of the panic is both--via wrapping--an
// ErrInvariantViolation and the error itself.
func InvariantMust(err error, args ...any) {
	if err == nil {
		return
	}

	panic(&internal.MergedError{
		Current: fmt.Errorf("%s: %w", fmt.Sprint(args...), err),
		Wrapped: ErrInvariantViolation,
	})
}

// InvariantCheck calls the function and if it returns an error panics
// with an ErrInvariantViolation error, wrapped with the error of the
// function, and any annotation arguments.
func InvariantCheck(fn func() error, args ...any) { InvariantMust(fn(), args...) }

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, ok := r.(error)
	if r == nil || !ok {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T { InvariantMust(err); return arg }

// MustBeOk raises an invariant violation if the ok value is false,
// and returns the first value if the second value is ok. Useful as
// in:
//
//	out := fun.MustBeOk(func() (string ok) { return "hello world", true })
func MustBeOk[T any](out T, ok bool) T { Invariant(ok, "ok check failed"); return out }

// Check, like Safe, runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func Check(fn func()) (err error) {
	defer func() { err = buildRecoverError(recover()) }()
	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() { err = buildRecoverError(recover()) }()
	out = fn()
	return
}

// Protect wraps a function with a panic handler, that will parse and
// attach the content of the pantic to the error output (while
// maintaining the functions orginial error.) All handled panics will
// be annotated with fun.ErrRecoveredPanic.
func Protect[I any, O any](fn func(I) (O, error)) func(I) (O, error) {
	return func(in I) (out O, err error) {
		defer func() { err = mergeWithRecover(err, recover()) }()
		return fn(in)
	}
}

// Observer describes a function that operates on a single object, but
// returns no output, and is used primarly for side effects,
// particularly around handling errors or collecting metrics. The
// Observer implementation here makes it possible to provide
// panic-safety for these kinds of functions or easily convert to
// other related types.
type Observer[T any] func(T)

// Safe handles any panic encountered during the observer's execution
// and converts it to an error.
func (of Observer[T]) Safe(in T) (err error) {
	defer func() { err = buildRecoverError(recover()) }()
	of(in)
	return
}

// Worker captures a variable and returns a worker function which
// will, when executed, observe the input value. These worker
// functions, use the Safe-mode of execution.
func (of Observer[T]) Worker(in T) WorkerFunc {
	return func(context.Context) (err error) { return of.Safe(in) }
}

// Wait captures a variable and converts an Observer into a wait
// function that observes the value when the WaitFunc runs.
func (of Observer[T]) Wait(in T) WaitFunc { return func(context.Context) { of(in) } }

func buildRecoverError(r any) error {
	if r == nil {
		return nil
	}

	switch in := r.(type) {
	case error:
		return &internal.MergedError{
			Current: in,
			Wrapped: ErrRecoveredPanic,
		}
	default:
		return &internal.MergedError{
			Current: fmt.Errorf("panic: %v", in),
			Wrapped: ErrRecoveredPanic,
		}
	}
}

func mergeWithRecover(err error, r any) error {
	return internal.MergeErrors(err, buildRecoverError(r))
}
