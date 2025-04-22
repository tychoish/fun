package erc

import (
	"errors"
	"fmt"

	"github.com/tychoish/fun/ers"
)

// Recover catches a panic, turns it into an error and passes it to
// the provided observer function.
func Recover(ob func(error)) { ob(ParsePanic(recover())) }

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
		default:
			return fmt.Errorf("[%T]: %v: %w", err, err, ers.ErrRecoveredPanic)
		}
	}
	return nil
}

// NewInvariantViolation creates a new error object, which always
// includes
func NewInvariantViolation(args ...any) error {
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
		args, errs := extractErrors(args)
		out := append(make([]error, 0, len(errs)+2), ers.ErrInvariantViolation)
		if len(args) > 0 {
			out = append(out, errors.New(fmt.Sprintln(args...)))
		}
		return Join(append(out, errs...)...)
	}
}

// IsInvariantViolation returns true if the argument is or resolves to
// ers.ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, _ := r.(error)
	if r == nil || ers.IsOk(err) {
		return false
	}

	return errors.Is(err, ers.ErrInvariantViolation)
}

// extractErrors iterates through a list of untyped objects and removes the
// errors from the list, returning both the errors and the remaining
// items.
func extractErrors(in []any) (rest []any, errs []error) {
	for idx := range in {
		switch val := in[idx].(type) {
		case nil:
			continue
		case error:
			errs = append(errs, val)
		case func() error:
			if e := val(); e != nil {
				errs = append(errs, e)
			}
		case string:
			if val == "" {
				continue
			}
			rest = append(rest, val)
		default:
			rest = append(rest, val)
		}
	}
	return
}
