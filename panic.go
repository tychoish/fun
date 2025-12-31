package fun

import (
	"fmt"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// Invariant provides a namespace for making runtime invariant
// assertions. These all raise panics, passing error objects from
// panic, which can be more easily handled. These helpers are
// syntactic sugar around Invariant.OK, and the invariant is violated
// the ErrInvariantViolation.
var Invariant = RuntimeInvariant{}

// RuntimeInvariant is a type defined to create a namespace, callable
// (typically) via the Invariant symbol. Access these functions as in:
//
//	fun.Invariant.Ok(len(slice) > 0, "slice must have elements", len(slice))
type RuntimeInvariant struct{}

// Ok panics if the condition is false, passing an error that is
// rooted in InvariantViolation. Otherwise the operation is a noop.
func (RuntimeInvariant) Ok(cond bool, args ...any) {
	if !cond {
		panic(erc.NewInvariantError(args...))
	}
}

// Must raises an invariant error if the error is not nil. The content
// of the panic is both--via wrapping--an ErrInvariantViolation and
// the error itself.
func (RuntimeInvariant) Must(err error, args ...any) {
	Invariant.Ok(err == nil, func() error { return ers.Wrap(err, fmt.Sprintln(args...)) })
}
