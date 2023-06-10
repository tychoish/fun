// Package risky contains a bunch of bad ideas for APIs and operations
// that will definitely lead to panics and deadlocks and incorrect
// behavior when used incorrectly.
//
// At the same time, these operations may be useful in the right
// situations, and the package name "risky" indicates (like "unsafe")
// that the operation should be handled with care.
package risky

import (
	"context"
)

// Check takes two values and returns the first value and a second
// "ok" value. The second value is true if the error is nil (isOK) and
// false otherwise. This is not too risky.
func Check[T any](out T, err error) (T, bool) { return out, err == nil }

// Force swallows an error, and returns the output, as a non-panic'ing
// form of risky.Force.
//
//	size += risky.Force(buffer.Write([]byte("hello world")))
func Force[T any](out T, _ error) T { return out }

// Block runs the function with a context that is never canceled. Use
// this in cases where you don't want to plumb a context through *and*
// the operation cannot block on the context.
func Block[T any](fn func(context.Context) T) T { return fn(context.Background()) }

// ForceOp, is like Force, except it takes the function and calls it
// itself so that it can ignore a possible panic and return an
// output. In the case of a panic or an error the output value is
// often the zero value for the type.
func ForceOp[T any](fn func() (T, error)) (out T) {
	defer Recover()
	out, _ = fn()
	return out
}

func Cast[T any](in any) T { return in.(T) }

// BlockForce combines Block with ForceOp to run a function with a
// context that is never canceled, ignoring any panics and returning
// the output value.
func BlockForceOp[T any](fn func(context.Context) (T, error)) T {
	defer Recover()
	out, _ := fn(context.Background())
	return out
}

// Ignore runs a function that takes an arbitrary argument and ignores
// the error and swallows any panic. This is a risky move: usually
// functions panic for a reason, but for certain invariants this may
// be useful.
//
// Be aware, that while Ignore will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func Ignore[T any](fn func(T) error, arg T) { defer Recover(); _ = fn(arg) }

// Recover catches a panic and discards its contents.
func Recover() { _ = recover() }

// Try runs a function with the provided input, and returns the
// input. If the operation panics or returns an error, Try returns the
// input argument.
func Try[T any](op func(T) (T, error), input T) (out T) {
	defer Recover()
	out = input
	if output, err := op(input); err == nil {
		out = output
	}
	return
}

// IgnoreMust runs a function that takes an arbitrary argument and
// ignores the error and swallows any panic, returning the output of
// the function, likely a Zero value, in the case of an error.  This
// is a risky move: usually functions panic for a reason, but for
// certain invariants this may be useful.
//
// Be aware, that while Ignore will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func IgnoreMust[T any, O any](fn func(T) (O, error), arg T) O {
	defer Recover()
	val, _ := fn(arg)
	return val
}
