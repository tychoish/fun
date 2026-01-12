package stw

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

// Default takes two values. if the first value is the zero value for
// the type T, then Default returns the second (default)
// value. Otherwise it returns the first input type.
func Default[T comparable](input T, defaultValue T) T {
	if IsZero(input) {
		return defaultValue
	}
	return input
}

// Ptr returns a pointer for the object. Useful for setting the value
// in structs where you cannot easily create a reference (e.g. the
// output of functions, and for constant literals.). If you pass a
// value that is a pointer (e.x. *string), then Ptr returns
// **string. If the input object is a nil pointer, then Ptr returns a
// non-nil pointer to a nil pointer.
func Ptr[T any](in T) *T { return &in }

// DerefOk takes a pointer to an value and returns the concrete type for
// that pointer. If the pointer is nil, DerefOk returns the zero value
// for that type. The boolean value indicates if the zero value
// returned is because the reference.
func DerefOk[T any](in *T) (value T, ok bool) {
	if in == nil {
		return value, false
	}
	return *in, true
}

// DerefZ de-references a pointer value, and if the pointer is nil, returns the zero value for that
// type. This cannot panic, but does not distinguish between "nil pointer" and "pointer to the zero
// value for the type" input.
func DerefZ[T any](in *T) (value T) { value, _ = DerefOk(in); return }

// Deref de-references a pointer, panicing if the pointer is nil.
func Deref[T any](in *T) T { return *in }
