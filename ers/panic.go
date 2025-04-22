package ers

import (
	"errors"
	"fmt"
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
			return Join(err, ErrRecoveredPanic)
		case string:
			return Join(New(err), ErrRecoveredPanic)
		case []error:
			out := make([]error, len(err), len(err)+1)
			copy(out, err)
			return Join(append(out, ErrRecoveredPanic)...)
		default:
			return fmt.Errorf("[%T]: %v: %w", err, err, ErrRecoveredPanic)
		}
	}
	return nil
}

// NewInvariantViolation creates a new error object, which always
// includes
func NewInvariantViolation(args ...any) error {
	switch len(args) {
	case 0:
		return ErrInvariantViolation
	case 1:
		switch ei := args[0].(type) {
		case error:
			return Join(ei, ErrInvariantViolation)
		case string:
			return Join(New(ei), ErrInvariantViolation)
		case func() error:
			return Join(ei(), ErrInvariantViolation)
		default:
			return fmt.Errorf("%v: %w", args[0], ErrInvariantViolation)
		}
	default:
		args, errs := ExtractErrors(args)
		out := append(make([]error, 0, len(errs)+2), ErrInvariantViolation)
		if len(args) > 0 {
			out = append(out, errors.New(fmt.Sprintln(args...)))
		}
		return Join(append(out, errs...)...)
	}
}

// IsInvariantViolation returns true if the argument is or resolves to
// ErrInvariantViolation.
func IsInvariantViolation(r any) bool {
	err, _ := r.(error)
	if r == nil || IsOk(err) {
		return false
	}

	return errors.Is(err, ErrInvariantViolation)
}
