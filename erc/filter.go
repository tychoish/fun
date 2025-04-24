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
func (erf Filter) Then(after Filter) Filter { return erf.Join(after) }

// Next calls the "next" filter on the result of the first
// filter. Both Filters are called all errors, including nil errors.
func (erf Filter) Next(next Filter) Filter { return func(err error) error { return next(erf(err)) } }

// Apply runs the Filter on all non-nil errors. nil Filters become
// noops. This means for all non-Force filters, nil you can use
// uninitialized filters as the "base" of a chain, as in:
//
//	var erf Filter
//	err = erf.Join(f1, f1, f3).Apply(err)
//
// While Apply will not attempt to execute a nil Filter, it's possible
// to call a nil Filter added with Next() or where the filter is
// called directly.
func (erf Filter) Apply(err error) error {
	switch {
	case err == nil:
		return nil
	case erf == nil:
		return err
	default:
		return erf(err)
	}
}

// Join returns a filter that applies itself, and then applies all of
// the filters passed to it sequentially. If an filter returns a
// nil error immediately, execution stops as this is a short circut operation.
func (erf Filter) Join(filters ...Filter) Filter {
	return func(err error) error {
		if err = erf.Apply(err); ers.IsOk(err) {
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

// Remove returns nil whenever the check function returns true. Use
// this to remove errors from the previous Filter (e.g. erf).
func (erf Filter) Remove(check func(error) bool) Filter {
	return erf.Then(func(err error) error {
		if check(err) {
			return nil
		}
		return err
	})
}

// Without produces a filter that only returns errors that do NOT
// appear in the exclusion list. All other errors are takes an error
// and returns nil if the error is nil, or if the error (or one of its
// wrapped errors,) is in the exclusion list.
func (erf Filter) Without(exclusions ...error) Filter {
	if len(exclusions) == 0 {
		return erf
	}

	return erf.Remove(func(err error) bool { return ers.Is(err, exclusions...) })
}

// Only takes filters errors, propagating only errors that are (in the
// sense of errors.Is) in the inclusion list. All other errors are
// returned as nil and are not propagated.
func (erf Filter) Only(inclusions ...error) Filter {
	if len(inclusions) == 0 {
		return erf
	}

	return erf.Remove(func(err error) bool { return !ers.Is(err, inclusions...) })
}

// WithoutContext removes only context cancellation and deadline
// exceeded/timeout errors. All other errors are propagated.
func (erf Filter) WithoutContext() Filter { return erf.Remove(ers.IsExpiredContext) }

// WithoutTerminating removes all terminating errors (e.g. io.EOF,
// ErrCurrentOpAbort, ErrContainerClosed). Other errors are
// propagated.
func (erf Filter) WithoutTerminating() Filter { return erf.Remove(ers.IsTerminating) }
