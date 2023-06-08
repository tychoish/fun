package ers

import (
	"errors"
	"fmt"
)

func Merge(err1, err2 error) error {
	switch {
	case err1 == nil && err2 == nil:
		return nil
	case err1 == nil && err2 != nil:
		return err2
	case err1 != nil && err2 == nil:
		return err1
	default:
		return &Combined{Current: err1, Previous: err2}
	}
}

type Combined struct {
	Current  error
	Previous error
}

func (dwe *Combined) Unwrap() error { return dwe.Previous }
func (dwe *Combined) Error() string { return fmt.Sprintf("%v: %v", dwe.Current, dwe.Previous) }

func (dwe *Combined) Is(target error) bool {
	return errors.Is(dwe.Current, target) || errors.Is(dwe.Previous, target)
}

func (dwe *Combined) As(target any) bool {
	return errors.As(dwe.Current, target) || errors.As(dwe.Previous, target)
}
