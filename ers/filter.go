package ers

// Filter provides a way to process error messages, either to remove
// errors, reformulate,  or annotate errors.
type Filter func(error) error

// Filter takes an error and returns nil if the error is nil, or if
// the error (or one of its wrapped errors,) is in the exclusion list.
func FilterRemove(exclusions ...error) Filter {
	return FilterCheck(func(err error) bool { return Ok(err) || len(exclusions) == 0 || Is(err, exclusions...) })
}

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

func FilterConvert(output error) Filter {
	return func(err error) error {
		if Ok(err) {
			return nil
		}
		return output
	}
}

// FilterToRoot produces a filter which always returns only the root/MOST
// wrapped error present in an error object.
func FilterToRoot() Filter { return findRoot }

func findRoot(err error) error {
	for {
		switch wi := any(err).(type) {
		case nil:
			return nil
		case interface{ Unwrap() error }:
			err = wi.Unwrap()
		case interface{ Unwrap() []error }:
			sl := wi.Unwrap()
			if len(sl) == 0 {
				return err
			}
			return sl[len(sl)-1]
		default:
			return err
		}
	}
}
