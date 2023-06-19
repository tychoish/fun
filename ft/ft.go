package ft

// IsZero returns true if the input value compares "true" to the zero
// value for the type of the argument. If the type implements an
// IsZero() method (e.g. time.Time), then IsZero returns that value,
// otherwise, IsZero constructs a zero valued object of type T and
// compares the input value to the zero value.
func IsZero[T comparable](in T) bool {
	switch val := any(in).(type) {
	case interface{ IsZero() bool }:
		return val.IsZero()
	default:
		var comp T
		return in == comp
	}
}

func IsType[T any](in any) bool         { _, ok := in.(T); return ok }
func Cast[T any](in any) (v T, ok bool) { v, ok = in.(T); return }

// Ptr returns a pointer for the object. Useful for setting the value
// in structs where you cannot easily create a reference (e.g. the
// output of functions, and for constant literals.). If you pass a
// value that is a pointer (e.x. *string), then Ptr returns
// **string. If the input object is a nil pointer, then Ptr returns a
// non-nil pointer to a nil pointer.
func Ptr[T any](in T) *T { return &in }

// Default takes two values. if the first value is the zero value for
// the type T, then Default returns the second (default)
// value. Otherwise it returns the first input type.
func Default[T comparable](input T, defaultValue T) T {
	if IsZero(input) {
		return defaultValue
	}
	return input
}

// IsOk returns only the second argument passed to it, given a
// function that returns two values where the second value is a
// boolean, you can use IsOk to discard the first value.
func IsOk[T any](_ T, ok bool) bool { return ok }

// WhenCall runs a function when condition is true, and is a noop
// otherwise.
func WhenCall(cond bool, op func()) {
	if !cond {
		return
	}
	op()
}

// DoTimes runs the specified option n times.
func DoTimes(n int, op func()) {
	for i := 0; i < n; i++ {
		op()
	}
}

// SafeCall only calls the operation when it's non-nil.
func SafeCall(op func()) { WhenCall(op != nil, op) }

// WhenDo calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type.
func WhenDo[T any](cond bool, op func() T) (out T) {
	if !cond {
		return out
	}
	return op()
}

// Contain returns true if an element of the slice is equal to the
// item.
func Contains[T comparable](item T, slice []T) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

// Wrapper produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrapper[T any](in T) func() T { return func() T { return in } }
