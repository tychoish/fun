package ers

import (
	"errors"
	"fmt"
)

// Filter provides a way to process error messages, either to remove
// errors, reformulate,  or annotate errors.
type Filter func(error) error

// Run runs the filter on the provided error to provide the option of
// improving readability at callsites
func (f Filter) Run(err error) error { return f(err) }

// FilterExclude takes an error and returns nil if the error is nil,
// or if the error (or one of its wrapped errors,) is in the exclusion
// list.
func FilterExclude(exclusions ...error) Filter {
	if len(exclusions) == 0 {
		return FilterNoop()
	}
	return FilterCheck(func(err error) bool { return Ok(err) || Is(err, exclusions...) })
}

// FilterNoop produces a filter that always returns the original error.
func FilterNoop() Filter { return func(err error) error { return err } }

// FilterCheck is an error filter that returns nil when the check is
// true, and false otherwise.
func FilterCheck(ep func(error) bool) Filter {
	return func(err error) error {
		if ep(err) {
			return nil
		}
		return err
	}
}

// FilterConvert returns the provided "output" error for all non-nil
// errors, and returns nil otherwise.
func FilterConvert(output error) Filter {
	return func(err error) error {
		if Ok(err) {
			return nil
		}
		return output
	}
}

// Extract iterates through a list of untyped objects and removes the
// errors from the list, returning both the errors and the remaining
// items.
func ExtractErrors(in []any) (rest []any, errs []error) {
	for idx := range in {
		switch val := in[idx].(type) {
		case nil:
			continue
		case error:
			errs = append(errs, val)
		case func() error:
			if e := val(); e != nil {
				errs = append(errs, e)
			}
		case string:
			if val == "" {
				continue
			}
			rest = append(rest, val)
		default:
			rest = append(rest, val)
		}
	}
	return
}

func extractAndJoin(in []any, withErrs ...error) error {
	args, errs := ExtractErrors(in)
	out := append(make([]error, 0, len(errs)+1+len(withErrs)), withErrs...)
	if len(args) > 0 {
		out = append(out, errors.New(fmt.Sprintln(args...)))
	}
	return Join(append(out, errs...)...)
}

// RemoveOk removes all nil errors from a slice of errors, returning
// the consolidated slice.
func RemoveOk(errs []error) []error {
	out := make([]error, 0, len(errs))
	for idx := range errs {
		if IsError(errs[idx]) {
			out = append(out, errs[idx])
		}
	}
	return out
}

// FilterToRoot produces a filter which always returns only the root/MOST
// wrapped error present in an error object.
func FilterToRoot() Filter { return findRoot }

func findRoot(err error) error {
	errs := Unwind(err)
	if len(errs) == 0 {
		return nil
	}

	return errs[len(errs)-1]
}
