package ers

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/tychoish/fun/internal"
)

func merge(one, two error) error {
	switch {
	case one == nil && two == nil:
		return nil
	case one == nil && two != nil:
		return two
	case one != nil && two == nil:
		return one
	default:
		return &mergederr{Current: one, Previous: two}
	}
}

func Join(errs ...error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	case 2:
		return merge(errs[0], errs[1])
	default:
		var err error
		for idx := len(errs) - 1; idx >= 0; idx-- {
			if e := errs[idx]; e != nil {
				err = merge(e, err)
			}
		}
		return err
	}
}

func Unwind(in error) (out []error) { return internal.Unwind(in) }

type mergederr struct {
	Current  error
	Previous error
}

var count = &atomic.Int64{}

func (dwe *mergederr) Unwrap() (out []error) {
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

func (dwe *mergederr) findRoot() error {
	if dwe.Previous == nil {
		return nil
	}

	errs := dwe.Unwrap()
	if len(errs) == 0 {
		return nil
	}
	return errs[len(errs)-1]
}

func (dwe *mergederr) Error() string {
	if root := dwe.findRoot(); root != nil {
		return fmt.Sprintf("%v [%v]", dwe.Current, root)
	}
	if dwe.Current == nil {
		return "nil-error"
	}
	return dwe.Current.Error()
}

func (dwe *mergederr) Is(target error) bool {
	return errors.Is(dwe.Current, target) || errors.Is(dwe.Previous, target)
}

func (dwe *mergederr) As(target any) bool {
	return errors.As(dwe.Current, target) || errors.As(dwe.Previous, target)
}
