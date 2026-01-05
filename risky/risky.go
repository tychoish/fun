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

// Cast just provides as a wrapper around in.(T). Cast will panic.
func Cast[T any](in any) T { return in.(T) }

// Force swallows an error, and returns the output, as a non-panic'ing
// form of risky.Force.
//
//	size += risky.Force(buffer.Write([]byte("hello world")))
func Force[T any](out T, _ error) T { return out }

// ForceCall takes the function and calls it itself so that it can
// ignore a possible panic and return an output. In the case of a
// panic or an error the output value is often the zero value for the
// type.
func ForceCall[T any](fn func() (T, error)) T { defer Recover(); return Force(fn()) }

// Block runs a function with a background Context, like Block.
func Block[T any](fn func(context.Context) (T, error)) (T, error) { return fn(context.Background()) }

// BlockForce runs the function with a context that is never canceled. Use
// this in cases where you don't want to plumb a context through *and*
// the operation cannot block on the context.
func BlockForce[T any](fn func(context.Context) T) T { return fn(context.Background()) }

// BlockForceIgnore combines Block with ForceOp to run a function with a
// context that is never canceled, ignoring any panics and returning
// the output value.
func BlockForceIgnore[T any](fn func(context.Context) (T, error)) T {
	defer Recover()
	return ignoreSecond(Block(fn))
}

// WithRecover runs a function that takes an arbitrary argument and ignores
// the error and swallows any panic. This is a risky move: usually
// functions panic for a reason, but for certain invariants this may
// be useful.
//
// Be aware, that while WithRecover will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func WithRecover[T any](fn func(T) error, arg T) { defer Recover(); _ = fn(arg) }

func ignoreSecond[A, B any](a A, _ B) A { return a }

// Recover catches a panic and discards its value.
func Recover() { _ = recover() }

// ApplyDefault runs a function with the provided input, and returns the
// input. If the operation panics or returns an error, ApplyDefault returns
// the input argument.
func ApplyDefault[T any](op func(T) (T, error), input T) (out T) {
	defer Recover()
	out = input
	if output, err := op(input); err == nil {
		out = output
	}
	return
}

// Apply runs a function that takes an arbitrary argument and
// ignores the error and swallows any panic, returning the output of
// the function, likely a Zero value, in the case of an error.  This
// is a risky move: usually functions panic for a reason, but for
// certain invariants this may be useful.
//
// Be aware, that while Ignore will recover from any panics, defers
// within the ignored function will not run unless there is a call to
// recover *before* the defer.
func Apply[T any, O any](fn func(T) (O, error), arg T) O {
	defer Recover()
	val, _ := fn(arg)
	return val
}

// ApplyAll processes an input slice, with the provided function,
// returning a new slice that holds the results. Panics are ignored
// and do not abort the operation. The output is always the same
// length as the input.
func ApplyAll[T any, O any](fn func(T) O, in []T) (out []O) {
	defer Recover()

	out = make([]O, len(in))

	for idx := range in {
		out[idx] = fn(in[idx])
	}

	return out
}
