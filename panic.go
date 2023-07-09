package fun

import (
	"errors"
	"fmt"

	"github.com/tychoish/fun/ers"
)

// ErrInvariantViolation is the root error of the error object that is
// the content of all panics produced by the Invariant helper.
const ErrInvariantViolation ers.Error = ers.ErrInvariantViolation

// ErrRecoveredPanic is at the root of any error returned by a
// function in the fun package that recovers from a panic.
const ErrRecoveredPanic ers.Error = ers.ErrRecoveredPanic

// Invariant provides a namespace for making runtime invariant
// assertions. These all raise panics, passing error objects from
// panic, which can be more easily handled. These helpers are
// syntactic sugar around Invariant.OK, and the invariant is violated
// the ErrInvariantViolation.
var Invariant RuntimeInvariant = RuntimeInvariant{}

// RuntimeInvariant is a type defined to create a namespace, callable
// (typically) via the Invariant symbol. Access these functions as in:
//
//	fun.Invariant.IsTrue(len(slice) > 0, "slice must have elements", len(slice))
type RuntimeInvariant struct{}

// IsTrue provides a runtime assertion that the condition is true, and
// annotates panic object, which is an error rooted in the
// ErrInvariantViolation. In all other cases the operation is a noop.
func (RuntimeInvariant) IsTrue(cond bool, args ...any) { Invariant.OK(cond, args...) }

// IsTrue provides a runtime assertion that the condition is false,
// and annotates panic object, which is an error rooted in the
// ErrInvariantViolation. In all other cases the operation is a noop.
func (RuntimeInvariant) IsFalse(cond bool, args ...any) { Invariant.OK(!cond, args...) }

// Failure unconditionally raises an invariant failure error and
// processes the arguments as with the other invariant failures:
// extracting errors and aggregating constituent errors.
func (RuntimeInvariant) Failure(args ...any) { Invariant.OK(false, args...) }

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
			case func() error:
				panic(ers.Join(ei(), ErrInvariantViolation))
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

// Must raises an invariant error if the error is not nil. The content
// of the panic is both--via wrapping--an ErrInvariantViolation and
// the error itself.
func (RuntimeInvariant) Must(err error, args ...any) {
	Invariant.OK(err == nil, func() error { return ers.Wrap(err, args...) })
}
