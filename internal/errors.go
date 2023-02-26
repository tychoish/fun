package internal

import (
	"errors"
	"fmt"
)

type MergedError struct {
	Current error
	Wrapped error
}

var _ error = &MergedError{}

func (dwe *MergedError) Error() string        { return fmt.Sprintf("%v: %v", dwe.Current, dwe.Wrapped) }
func (dwe *MergedError) Is(target error) bool { return errors.Is(dwe.Current, target) }
func (dwe *MergedError) As(target any) bool   { return errors.As(dwe.Current, target) }
func (dwe *MergedError) Unwrap() error        { return dwe.Wrapped }
