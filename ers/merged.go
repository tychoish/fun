package ers

import (
	"errors"
	"fmt"
	"sync/atomic"
)

func Merge(one, two error) error {
	switch {
	case one == nil && two == nil:
		return nil
	case one == nil && two != nil:
		return two
	case one != nil && two == nil:
		return one
	default:
		return &Combined{Current: one, Previous: two}
	}
}

func Splice(errs ...error) (err error) {
	for idx := len(errs) - 1; idx >= 0; idx-- {
		if e := errs[idx]; e != nil {
			err = Merge(e, err)
		}
	}
	return err
}

type Combined struct {
	Current  error
	Previous error
}

var count = &atomic.Int64{}

func (dwe *Combined) Unwrap() (out []error) {
	for _, err := range []error{dwe.Current, dwe.Previous} {
		switch e := err.(type) {
		case interface{ Unwrap() []error }:
			out = append(out, e.Unwrap()...)
		case nil:
			continue
		default:
			out = append(out, e)
		}
	}
	return
}

func (dwe *Combined) findRoot() error {
	if dwe.Previous == nil {
		return nil
	}

	errs := dwe.Unwrap()
	if len(errs) == 0 {
		return nil
	}
	return errs[len(errs)-1]
}

func (dwe *Combined) Error() string {
	if root := dwe.findRoot(); root != nil {
		return fmt.Sprintf("%v [%v]", dwe.Current, root)
	}
	if dwe.Current == nil {
		return "nil-error"
	}
	return dwe.Current.Error()
}

func (dwe *Combined) Is(target error) bool {
	return errors.Is(dwe.Current, target) || errors.Is(dwe.Previous, target)
}

func (dwe *Combined) As(target any) bool {
	return errors.As(dwe.Current, target) || errors.As(dwe.Previous, target)
}
