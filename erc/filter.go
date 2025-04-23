package erc

import (
	"github.com/tychoish/fun/ers"
)

// Filter provides a way to process error messages, either to remove
// errors, reformulate,  or annotate errors.
type Filter func(error) error

// NewFilter produces a filter that always returns the original error.
func NewFilter() Filter { return func(err error) error { return err } }

// Run runs the filter on the provided error to provide the option of
// improving readability at callsites
func (f Filter) Then(after Filter) Filter { return f.Join(after) }

func (f Filter) Apply(err error) error {
	if err == nil {
		return nil
	}
	return f(err)
}

func (f Filter) Join(filters ...Filter) Filter {
	return func(err error) error {
		if err = f.Apply(err); ers.IsOk(err) {
			return nil
		}

		for _, filter := range filters {
			if filter == nil {
				continue
			}

			if err = filter(err); ers.IsOk(err) {
				return nil
			}
		}

		return err
	}
}

func (f Filter) ForceApply(err error) error { return f(err) }

func (f Filter) ForceJoin(filters ...Filter) Filter {
	return func(err error) error {
		err = f.ForceApply(err)
		for _, filter := range filters {
			err = filter(err)
		}
		return err
	}
}

// FilterExclude takes an error and returns nil if the error is nil,
// or if the error (or one of its wrapped errors,) is in the exclusion
// list.
func (f Filter) Without(exclusions ...error) Filter {
	if len(exclusions) == 0 {
		return f
	}

	return Filter(func(err error) error {
		if ers.Is(err, exclusions...) {
			return nil
		}

		return err
	}).Then(f)
}

func (f Filter) WithOnly(inclusions ...error) Filter {
	return Filter(func(err error) error {
		if ers.Is(err, inclusions...) {
			return err
		}

		return nil
	}).Then(f)
}

func (f Filter) WithoutContext() Filter {
	return Filter(func(err error) error {
		if ers.IsExpiredContext(err) {
			return nil
		}
		return err
	}).Then(f)
}

// WithoutTerminating removes all terminating errors (e.g. io.EOF,
// ErrCurrentOpAbort, ErrContainerClosed). Other errors are
// propagated.
func (f Filter) WithoutTerminating() Filter {
	return Filter(func(err error) error {
		if ers.IsTerminating(err) {
			return nil
		}
		return err
	}).Then(f)
}
