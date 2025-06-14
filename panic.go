package fun

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tychoish/fun/erc"
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
var Invariant = RuntimeInvariant{}

// RuntimeInvariant is a type defined to create a namespace, callable
// (typically) via the Invariant symbol. Access these functions as in:
//
//	fun.Invariant.IsTrue(len(slice) > 0, "slice must have elements", len(slice))
type RuntimeInvariant struct{}

// New creates an error that is rooted in ers.ErrInvariantViolation,
// aggregating errors and annotating the error.
func (RuntimeInvariant) New(args ...any) error {
	switch len(args) {
	case 0:
		return ers.ErrInvariantViolation
	case 1:
		switch ei := args[0].(type) {
		case error:
			return erc.Join(ei, ers.ErrInvariantViolation)
		case string:
			return erc.Join(ers.New(ei), ers.ErrInvariantViolation)
		case func() error:
			return erc.Join(ei(), ers.ErrInvariantViolation)
		default:
			return fmt.Errorf("%v: %w", args[0], ers.ErrInvariantViolation)
		}
	default:
		ec := &erc.Collector{}
		ec.Push(ers.ErrInvariantViolation)
		extractErrors(ec, args)
		return ec.Resolve()
	}
}

// Ok panics if the condition is false, passing an error that is
// rooted in InvariantViolation. Otherwise the operation is a noop.
func (RuntimeInvariant) Ok(cond bool, args ...any) {
	if !cond {
		panic(Invariant.New(args...))
	}
}

// Must raises an invariant error if the error is not nil. The content
// of the panic is both--via wrapping--an ErrInvariantViolation and
// the error itself.
func (RuntimeInvariant) Must(err error, args ...any) {
	Invariant.Ok(err == nil, func() error { return ers.Wrap(err, MAKE.StrJoin(args).Resolve()) })
}

// IsTrue provides a runtime assertion that the condition is true, and
// annotates panic object, which is an error rooted in the
// ErrInvariantViolation. In all other cases the operation is a noop.
func (RuntimeInvariant) IsTrue(cond bool, args ...any) { Invariant.Ok(cond, args...) }

// IsFalse provides a runtime assertion that the condition is false,
// and annotates panic object, which is an error rooted in the
// ErrInvariantViolation. In all other cases the operation is a noop.
func (RuntimeInvariant) IsFalse(cond bool, args ...any) { Invariant.Ok(!cond, args...) }

// Failure unconditionally raises an invariant failure error and
// processes the arguments as with the other invariant failures:
// extracting errors and aggregating constituent errors.
func (RuntimeInvariant) Failure(args ...any) { Invariant.Ok(false, args...) }

// extractErrors iterates through a list of untyped objects and removes the
// errors from the list, returning both the errors and the remaining
// items.
func extractErrors(ec *erc.Collector, in []any) {
	args := []any{}

	for idx := range in {
		switch val := in[idx].(type) {
		case nil:
			continue
		case error:
			ec.Push(val)
		case func() error:
			ec.Push(val())
		case string:
			val = strings.TrimSpace(val)
			if val != "" {
				args = append(args, val)
			}
		default:
			args = append(args, val)
		}
	}

	if len(args) > 0 {
		ec.Push(errors.New(strings.TrimSpace(fmt.Sprintln(args...))))
	}
}
