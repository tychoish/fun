package ers

import (
	"errors"
	"fmt"

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
		return &mergederr{current: one, previous: two}
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
	current  error
	previous error
}

func (dwe *mergederr) Unwrap() (out []error) {
	for _, err := range []error{dwe.current, dwe.previous} {
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

func (dwe *mergederr) rootSize() (size int, err error) {
	if dwe.previous == nil {
		if !OK(dwe.current) {
			return 1, nil
		}
		return 0, nil
	}

	errs := dwe.Unwrap()
	l := len(errs)

	if l == 0 {
		return 0, nil
	}

	return l, errs[l-1]
}

func (dwe *mergederr) Error() string {
	if size, root := dwe.rootSize(); root != nil {
		return fmt.Sprintf("%v [%v] <%d>", dwe.current, root, size)
	}
	if dwe.current == nil {
		return "<nil>"
	}
	return dwe.current.Error()
}

func (dwe *mergederr) Is(target error) bool {
	return errors.Is(dwe.current, target) || errors.Is(dwe.previous, target)
}

func (dwe *mergederr) As(target any) bool {
	return errors.As(dwe.current, target) || errors.As(dwe.previous, target)
}
