package erc

import "github.com/tychoish/fun/ers"

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
	return FilterCheck(func(err error) bool { return ers.IsOk(err) || ers.Is(err, exclusions...) })
}

// FilterNoop produces a filter that always returns the original error.
func FilterNoop() Filter { return func(err error) error { return err } }

// FilterContext returns nil for all nil and context cancellation
// errors. Other errors are propagated.
func FilterContext() Filter {
	return func(err error) error {
		if ers.IsOk(err) || ers.IsExpiredContext(err) {
			return nil
		}
		return err
	}
}

// FilterTerminating returns nil for all nil and terminating errors
// (e.g. io.EOF, ErrCurrentOpAbort, ErrContainerClosed). Other errors
// are propagated.
func FilterTerminating() Filter {
	return func(err error) error {
		if ers.IsOk(err) || ers.IsTerminating(err) {
			return nil
		}
		return err
	}
}

// FilterCheck is an error filter that returns nil when the check is
// true, and false otherwise.
func FilterCheck(ep func(error) bool) Filter {
	return func(err error) error {
		if err == nil || ep(err) {
			return nil
		}
		return err
	}
}

// FilterConvert returns the provided "output" error for all non-nil
// errors, and returns nil otherwise.
func FilterConvert(output error) Filter {
	return func(err error) error {
		if ers.IsOk(err) {
			return nil
		}
		return output
	}
}

// FilterJoin combines a group of Filters into a single Filter. The
// joined filter skips any nil input filters AND has short circut
// logic: if the input error is nil, any of the filters return a nil
// error, then the joined filter returns immediately, otherwise all
// filters will be processed.
func FilterJoin(filters ...Filter) Filter {
	return func(err error) error {
		if err == nil || len(filters) == 0 {
			return nil
		}

		for _, filter := range filters {
			if filter == nil {
				continue
			}

			err = filter(err)
			if err == nil {
				return nil
			}
		}

		return err
	}
}
