package erc

import (
	"fmt"

	"github.com/tychoish/fun/ers"
)

// ParsePanic converts a panic to an error, if it is not, and attaching
// the ErrRecoveredPanic error to that error. If no panic is
// detected, ParsePanic returns nil.
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

// NewInvariantError returns an error that is either
// ers.ErrInvariantViolation, or an error wrapped
// with ers.ErrInvariantViolation.
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
