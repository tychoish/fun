package fun

import (
	"errors"
	"fmt"

	"github.com/tychoish/fun/ers"
)

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
var ErrInvariantViolation = errors.New("invariant violation")

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
var ErrRecoveredPanic error = ers.ErrRecoveredPanic

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
				panic(ers.Join(ei, ErrInvariantViolation))
			case string:
				panic(ers.Join(errors.New(ei), ErrInvariantViolation))
			default:
				panic(fmt.Errorf("[%v]: %w", args[0], ErrInvariantViolation))
			}
		default:
			if err, ok := args[0].(error); ok {
				panic(ers.Join(fmt.Errorf("[%s]", args[1:]), ers.Join(err, ErrInvariantViolation)))
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
	if len(args) == 0 {
		panic(ers.Join(err, ErrInvariantViolation))
	}

	panic(ers.Join(fmt.Errorf("%s: %w", fmt.Sprint(args...), err), ErrInvariantViolation))
}

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
