package fun

import (
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
func Must[T any](arg T, err error) T {
	if err != nil {
		panic(err)
	}

	return arg
}

// Ignore runs a function that takes an arbitrary argument and ignores
// the error and swallows any panic. This is a risky move: usually
// functions panic for a reason, but for certain invariants this may
// be useful.
//
// Be aware, that while Ignore will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func Ignore[T any](fn func(T) error, arg T) {
	defer func() { _ = recover() }()
	_ = fn(arg)
}

// IgnoreMust runs a function that takes an arbitrary argument and
// ignores the error and swallows any panic, returning the output of
// the function, likely a Zero value, in the case of an error.  This
// is a risky move: usually functions panic for a reason, but for
// certain invariants this may be useful.
//
// Be aware, that while Ignore will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func IgnoreMust[T any, O any](fn func(T) (O, error), arg T) O {
	defer func() { _ = recover() }()
	val, _ := fn(arg)
	return val
}

// Check, like Safe, runs a function without arguments that does not
// produce an error, and, if the function panics, converts it into an
// error.
func Check(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = buildRecoverError(r)
		}
	}()

	fn()
	return
}

// Safe runs a function with a panic handler that converts the panic
// to an error.
func Safe[T any](fn func() T) (out T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = buildRecoverError(r)
		}
	}()
	out = fn()
	return
}

func buildRecoverError(r any) error {
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
