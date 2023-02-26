package internal

import (
	"errors"
	"fmt"
)

type DoubleWrappedError struct {
	Current error
	Wrapped error
}

var _ error = &DoubleWrappedError{}

func (dwe *DoubleWrappedError) Error() string        { return fmt.Sprintf("%v: %v", dwe.Current, dwe.Wrapped) }
func (dwe *DoubleWrappedError) Is(target error) bool { return errors.Is(dwe.Current, target) }
func (dwe *DoubleWrappedError) As(target any) bool   { return errors.As(dwe.Current, target) }
func (dwe *DoubleWrappedError) Unwrap() error        { return dwe.Wrapped }
