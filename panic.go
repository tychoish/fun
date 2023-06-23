package fun

import (
	"errors"
	"fmt"

	"github.com/tychoish/fun/ers"
)

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation = ers.ErrInvariantViolation

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
const ErrRecoveredPanic = ers.ErrRecoveredPanic

var Invariant RuntimeInvariant = RuntimeInvariant{}

type RuntimeInvariant struct{}

func (RuntimeInvariant) IsTrue(cond bool, args ...any)  { Invariant.OK(cond, args...) }
func (RuntimeInvariant) IsFalse(cond bool, args ...any) { Invariant.OK(!cond, args...) }

// OK panics if the condition is false, passing an error that is
// rooted in InvariantViolation. Otherwise the operation is a noop.
func (RuntimeInvariant) OK(cond bool, args ...any) {
	if !cond {
		switch len(args) {
		case 0:
			panic(ErrInvariantViolation)
		case 1:
			switch ei := args[0].(type) {
			case error:
				panic(ers.Join(ei, ErrInvariantViolation))
			case string:
				panic(ers.Join(ers.New(ei), ErrInvariantViolation))
			default:
				panic(fmt.Errorf("%v: %w", args[0], ErrInvariantViolation))
			}
		default:
			var errs []error
			args, errs = ers.ExtractErrors(args)
			panic(ers.Join(errors.New(fmt.Sprintln(args...)), ers.Join(append(errs, ErrInvariantViolation)...)))
		}
	}
}

// Must raises an invariant error if the error is not
// nil. The content of the panic is both--via wrapping--an
// ErrInvariantViolation and the error itself.
func (RuntimeInvariant) Must(err error, args ...any) {
	if err == nil {
		return
	}
	if len(args) == 0 {
		panic(ers.Join(err, ErrInvariantViolation))
	}

	panic(ers.Join(fmt.Errorf("%s: %w", fmt.Sprint(args...), err), ErrInvariantViolation))
}

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T { Invariant.Must(err); return arg }

// MustBeOk raises an invariant violation if the ok value is false,
// and returns the first value if the second value is ok. Useful as
// in:
//
//	out := fun.MustBeOk(func() (string ok) { return "hello world", true })
func MustBeOk[T any](out T, ok bool) T { Invariant.OK(ok, "ok check failed"); return out }
