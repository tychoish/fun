package ft

import (
	"sync"

	"github.com/tychoish/fun/ers"
)

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

func NotZero[T comparable](in T) bool { return !IsZero(in) }

// IsType checks if the type of the argument matches the type
// specifier.
func IsType[T any](in any) bool { _, ok := in.(T); return ok }

// Cast is the same as doing `v, ok := in.(t)`, but with more clarity
// at the call site,
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
func Ref[T any](in *T) T { return IgnoreSecond(RefOK(in)) }

// RefOK takes a pointer to an value and returns the concrete type for
// that pointer. If the pointer is nil, RefOK returns the zero value
// for that type. The boolean value indicates if the zero value
// returned is because the reference.
func RefOK[T any](in *T) (value T, ok bool) {
	if in == nil {
		return value, false
	}
	return *in, true
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

// IsOK returns only the second argument passed to it, given a
// function that returns two values where the second value is a
// boolean, you can use IsOK to discard the first value.
func IsOK[T any](_ T, ok bool) bool { return ok }

// WhenCall runs a function when condition is true, and is a noop
// otherwise.
func WhenCall(cond bool, op func()) {
	if !cond {
		return
	}
	op()
}

// WhenDo calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type.
func WhenDo[T any](cond bool, op func() T) (out T) {
	if !cond {
		return out
	}
	return op()
}

// WhenHandle passes the argument "in" to the operation IF the
// condition function (which also takes "in") returns true.
func WhenHandle[T any](cond func(T) bool, op func(T), in T) {
	if cond(in) {
		op(in)
	}
}

// DoTimes runs the specified option n times.
func DoTimes(n int, op func()) {
	for i := 0; i < n; i++ {
		op()
	}
}

// SafeCall only calls the operation when it's non-nil.
func SafeCall(op func()) { WhenCall(op != nil, op) }

// SafeDo calls the function when the operation is non-nil, and
// returns either the output of the function or
func SafeDo[T any](op func() T) T { return WhenDo(op != nil, op) }

// SafeWrap wraps an operation with SafeCall so that the resulting
// operation is never nil, and will never panic if the input operation
// is nil.
func SafeWrap(op func()) func() { return func() { SafeCall(op) } }

// Flip takes two arguments and returns them in the opposite
// order. Intended to wrap other functions to reduce the friction when
// briding APIs
func Flip[A any, B any](first A, second B) (B, A) { return second, first }

func IgnoreFirst[A any, B any](first A, second B) B  { return second }
func IgnoreSecond[A any, B any](first A, second B) A { return first }

// Once uses a sync.Once to wrap to provide an output function that
// will execute at most one time, while eliding/managing the sync.Once
// object.
func Once(f func()) func() { o := &sync.Once{}; f = SafeWrap(f); return func() { o.Do(f) } }

// OnceDo runs a function, exactly once, to produce a value which is
// then cached, and returned on any successive calls to the function.
func OnceDo[T any](op func() T) func() T {
	var cache T
	opw := Once(func() { cache = op() })
	return func() T { opw(); return cache }
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

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T {
	WhenCall(err != nil, func() { panic(ers.Join(err, ers.ErrInvariantViolation)) })
	return arg
}

// MustBeOk raises an invariant violation if the ok value is false,
// and returns the first value if the second value is ok. Useful as
// in:
//
//	out := ft.MustBeOk(func() (string ok) { return "hello world", true })
func MustBeOk[T any](out T, ok bool) T {
	WhenCall(!ok, func() { panic(ers.Join(ers.New("ok check failed"), ers.ErrInvariantViolation)) })
	return out
}
