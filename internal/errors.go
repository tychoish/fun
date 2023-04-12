package internal

import (
	"errors"
	"fmt"
)

type MergedError struct {
	Current error
	Wrapped error
}

func MergeErrors(err1, err2 error) error {
	switch {
	case err1 == nil && err2 == nil:
		return nil
	case err1 == nil && err2 != nil:
		return err2
	case err1 != nil && err2 == nil:
		return err1
	default:
		return &MergedError{Current: err1, Wrapped: err2}
	}
}

func (dwe *MergedError) Unwrap() error { return dwe.Wrapped }
func (dwe *MergedError) Error() string { return fmt.Sprintf("%v: %v", dwe.Current, dwe.Wrapped) }

func (dwe *MergedError) Is(target error) bool {
	return errors.Is(dwe.Current, target) || errors.Is(dwe.Wrapped, target)
}

func (dwe *MergedError) As(target any) bool {
	return errors.As(dwe.Current, target) || errors.As(dwe.Wrapped, target)
}
