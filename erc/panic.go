package erc

import (
	"fmt"

	"github.com/tychoish/fun/ers"
)

// ParsePanic converts a panic to an error, if it is not, and attaching the
// ErrRecoveredPanic error to that error. If no panic is detected, ParsePanic
// returns nil.
func ParsePanic(r any) error {
	if r != nil {
		switch err := r.(type) {
		case error:
			return Join(err, ers.ErrRecoveredPanic)
		case string:
			return Join(ers.New(err), ers.ErrRecoveredPanic)
		case []error:
			out := make([]error, len(err), len(err)+1)
			copy(out, err)
			return Join(append(out, ers.ErrRecoveredPanic)...)
		}
		return fmt.Errorf("[%T]: %v: %w", r, r, ers.ErrRecoveredPanic)
	}
	return nil
}

// NewInvariantError returns an error that is either ers.ErrInvariantViolation,
// or an error wrapped with ers.ErrInvariantViolation.
func NewInvariantError(args ...any) error {
	switch len(args) {
	case 0:
		return ers.ErrInvariantViolation
	case 1:
		switch ei := args[0].(type) {
		case error:
			return Join(ei, ers.ErrInvariantViolation)
		case string:
			return Join(ers.New(ei), ers.ErrInvariantViolation)
		case func() error:
			return Join(ei(), ers.ErrInvariantViolation)
		default:
			return fmt.Errorf("%v: %w", args[0], ers.ErrInvariantViolation)
		}
	default:
		ec := &Collector{}
		ec.Push(ers.ErrInvariantViolation)
		ec.extractErrors(args)
		return ec.Resolve()
	}
}

// Invariant raises an invariant error if the error is not nil. The content of
// the panic contains both the ers.ErrInvariantViolation, and optional
// additional annotations.
func Invariant(err error, args ...any) {
	if ers.IsError(err) {
		ec := &Collector{}
		ec.Push(err)
		ec.Push(ers.ErrInvariantViolation)
		ec.extractErrors(args)
		panic(ec.Resolve())
	}
}

// InvariantOk raises an invariant error when the condition is false. Optional
// args annotate the error that is passed to panic. The panic is always rooted
// in ers.ErrInvariantViolation.
func InvariantOk(condition bool, args ...any) {
	if !condition {
		panic(NewInvariantError(args...))
	}
}

// Must wraps a function that returns a value and an error, and converts the
// error to a panic.
func Must[T any](arg T, err error) T { Invariant(err); return arg }

// MustOk raises an invariant violation if the ok value is false, and returns
// the first value if the second value is ok. Useful as in:
//
//	out := erc.MustOk(func() (string, bool) { return "hello world", true })
func MustOk[T any](out T, ok bool) T { InvariantOk(ok); return out }
