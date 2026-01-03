package ft

// Check can wrap a the callsite of a function that returns a value
// and an error, and returns (zero, false) if the error is non, nil,
// and (value, true) if the error is nil.
func Check[T any](value T, err error) (zero T, _ bool) {
	if err != nil {
		return zero, false
	}
	return value, true
}

// WrapCheck returns a function that when called returns the result of Check on the provided value and error.
func WrapCheck[T any](value T, err error) func() (T, bool) {
	return func() (T, bool) { return Check(value, err) }
}

// Ignore is a noop, but can be used to annotate operations rather
// than assigning to the empty identifier:
//
//	_ = operation()
//	ft.Ignore(operation())
func Ignore[T any](T) { return }

// IgnoreFirst takes two arguments and returns only the second, for
// use in wrapping functions that return two values.
func IgnoreFirst[A any, B any](_ A, b B) B { return b }

// IgnoreSecond takes two arguments and returns only the first, for
// use when wrapping functions that return two values.
func IgnoreSecond[A any, B any](a A, _ B) A { return a }
