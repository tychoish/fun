package ft

import "github.com/tychoish/fun/internal"

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

// NotZero returns true when the item does not have the zero value for
// the type T.
func NotZero[T comparable](in T) bool { return Not(IsZero(in)) }

// IsType checks if the type of the argument matches the type
// specifier.
func IsType[T any](in any) bool { _, ok := in.(T); return ok }

// Cast is the same as doing `v, ok := in.(t)`, but with more clarity
// at the call site,.
func Cast[T any](in any) (v T, ok bool) { v, ok = in.(T); return }

// SafeCast casts the input object to the specified type, and returns
// either, the result of the cast, or a zero value for the specified
// type.
func SafeCast[T any](in any) T { return IgnoreSecond(Cast[T](in)) }

// Ptr returns a pointer for the object. Useful for setting the value
// in structs where you cannot easily create a reference (e.g. the
// output of functions, and for constant literals.). If you pass a
// value that is a pointer (e.x. *string), then Ptr returns
// **string. If the input object is a nil pointer, then Ptr returns a
// non-nil pointer to a nil pointer.
func Ptr[T any](in T) *T { return &in }

// Ref takes a pointer to an value and dereferences it, If the input
// value is nil, the output value is the zero value of the type.
func Ref[T any](in *T) T { return IgnoreSecond(RefOk(in)) }

// RefOk takes a pointer to an value and returns the concrete type for
// that pointer. If the pointer is nil, RefOk returns the zero value
// for that type. The boolean value indicates if the zero value
// returned is because the reference.
func RefOk[T any](in *T) (value T, ok bool) {
	if in == nil {
		return value, false
	}
	return *in, true
}

// IsPtr uses reflection to determine if an object is a pointer.
func IsPtr(in any) bool { return internal.IsPtr(in) }

// IsNil uses reflection to determine if an object is nil.
func IsNil(in any) bool { return internal.IsNil(in) }

// Default takes two values. if the first value is the zero value for
// the type T, then Default returns the second (default)
// value. Otherwise it returns the first input type.
func Default[T comparable](input T, defaultValue T) T {
	if IsZero(input) {
		return defaultValue
	}
	return input
}

// DefaultApply returns the input if it's not zero, otherwise applies the function to the argument and returns the result.
func DefaultApply[T comparable, A any](input T, fn func(A) T, arg A) T {
	if IsZero(input) {
		return input
	}
	return fn(arg)
}

// DefaultNew checks a pointer to a value, and when it is nil,
// constructs a new value of the same type.
func DefaultNew[T any](input *T) *T {
	if input != nil {
		return input
	}
	return new(T)
}

// DefaultFuture combines Default() and WhenDo: if the input value is
// NOT the zero value for the comparable type T it is returned directly;
// otherwise DefaultFuture calls the provided function and returns its
// output.
func DefaultFuture[T comparable](input T, fn func() T) T {
	if IsZero(input) {
		return fn()
	}
	return input
}
