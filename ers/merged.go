package ers

import (
	"errors"
	"fmt"
	"strings"
)

// Join is analogous to errors.Join, with a few distinctions: Errors
// are stored as a stack; in string form, errors are joined with ": "
// rather than a new line; if there are more than 4 errors, the error
// takes the following form:
//
//	"{top-most error's Error() value} <n={size}> [{bottom-most error's Error() value}]"
//
// Use Unwind() to get the full content of the aggregated error
// message, which are unwinded recursively.
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

type mergederr struct {
	current  error
	previous error
}

func (dwe *mergederr) Unwrap() (out []error) {
	for _, err := range []error{dwe.current, dwe.previous} {
		switch e := err.(type) {
		case interface{ Unwrap() []error }:
			out = Append(out, e.Unwrap()...)
		case nil:
			continue
		default:
			out = Append(out, e)
		}
	}
	return
}

func (dwe *mergederr) Error() string {
	if dwe == nil || (dwe.current == nil && dwe.previous == nil) {
		return "<nil>"
	}

	errs := dwe.Unwrap()
	if len(errs) == 1 {
		return errs[0].Error()
	}

	if len(errs) <= 4 {
		return strings.Join(Strings(errs), ": ")
	}

	return fmt.Sprintf("%v <n=%d> %v", errs[0], len(errs), errs[len(errs)-1])
}

func (dwe *mergederr) Is(target error) bool {
	return errors.Is(dwe.current, target) || errors.Is(dwe.previous, target)
}

func (dwe *mergederr) As(target any) bool {
	return errors.As(dwe.current, target) || errors.As(dwe.previous, target)
}

func Strings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for idx := range errs {
		if !OK(errs[idx]) {
			out = append(out, errs[idx].Error())
		}
	}

	return out
}
