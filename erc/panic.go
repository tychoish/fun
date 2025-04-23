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
		default:
			return fmt.Errorf("[%T]: %v: %w", err, err, ers.ErrRecoveredPanic)
		}
	}
	return nil
}
