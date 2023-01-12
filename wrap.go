package fun

// Is a generic version of `errors.Is` that takes advantage of the
// Unwrap function, and is useful for checking if an object of an
// interface type is or wraps an implementation of the type
// parameter. Callers
func Is[T any](in any) bool {
	for {
		if _, ok := in.(T); ok {
			return true
		}
		if in = Unwrap(in); in == nil {
			return false
		}
	}
}

// Unwrap is a generic equivalent of the `errors.Unwrap()` function
// for any type that implements an `Unwrap() T` method. useful in
// combination
func Unwrap[T any](in T) T {
	u, ok := any(in).(interface{ Unwrap() T })
	if !ok {
		return *new(T)
	}
	return u.Unwrap()
}
