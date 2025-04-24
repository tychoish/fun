package erc

import (
	"github.com/tychoish/fun/ers"
)

// Filter provides a way to process error messages, either to remove
// errors, reformulate,  or annotate errors.
type Filter func(error) error

// NewFilter produces a filter that always returns the original error.
func NewFilter() Filter { return func(err error) error { return err } }

// Then functions like Join, but for a single function. The "after"
// function is called on the results of the first function are
// non-nil. The first function is only called if the input error is
// non-nil. Nil filters are ignored.
func (f Filter) Then(after Filter) Filter { return f.Join(after) }

// Next calls the "next" filter on the result of the first
// filter. Both Filters are called all errors, including nil errors.
func (f Filter) Next(next Filter) Filter { return func(err error) error { return next(f(err)) } }

// Apply runs the Filter on all non-nil errors. nil Filters become
// noops. This means for all non-Force filters, nil you can use
// uninitialized filters as the "base" of a chain, as in:
//
//	var erf Filter
//	err = erf.Join(f1, f1, f3).Apply(err)
func (f Filter) Apply(err error) error {
	switch {
	case err == nil:
		return nil
	case f == nil:
		return err
	default:
		return f(err)
	}
}

// Join returns a filter that applies itself, and then applies all of
// the filters passed to it sequentially. If an filter returns a
// nil error immediately, execution stops as this is a short circut operation.
func (f Filter) Join(filters ...Filter) Filter {
	return func(err error) error {
		if err = f.Apply(err); ers.IsOk(err) {
			return nil
		}

		for _, filter := range filters {
			if err = filter.Apply(err); ers.IsOk(err) {
				return nil
			}
		}

		return err
	}
}

func (f Filter) Remove(check func(error) bool) Filter {
	return f.Then(func(err error) error {
		if check(err) {
			return nil
		}
		return err
	}).Then(f)
}

// FilterExclude takes an error and returns nil if the error is nil,
// or if the error (or one of its wrapped errors,) is in the exclusion
// list.
func (f Filter) Without(exclusions ...error) Filter {
	if len(exclusions) == 0 {
		return f
	}

	return f.Remove(func(err error) bool { return ers.Is(err, exclusions...) })
}

func (f Filter) Only(inclusions ...error) Filter {
	if len(inclusions) == 0 {
		return f
	}

	return f.Remove(func(err error) bool { return !ers.Is(err, inclusions...) })
}

func (f Filter) WithoutContext() Filter { return f.Remove(ers.IsExpiredContext) }

// WithoutTerminating removes all terminating errors (e.g. io.EOF,
// ErrCurrentOpAbort, ErrContainerClosed). Other errors are
// propagated.
func (f Filter) WithoutTerminating() Filter { return f.Remove(ers.IsTerminating) }
