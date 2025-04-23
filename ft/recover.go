package ft

import (
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// Recover catches a panic, turns it into an error and passes it to
// the provided observer function.
func Recover(ob func(error)) { ob(erc.ParsePanic(recover())) }

// WithRecoverCall runs a function without arguments that does not
// produce an error and, if the function panics, converts it into an
// error.
func WithRecoverCall(fn func()) (err error) {
	defer func() { err = erc.ParsePanic(recover()) }()
	fn()
	return
}

// WrapRecoverCall wraps a function without arguments that does not
// produce an error with one that does produce an error. When called,
// the new function will never panic but returns an error if the input
// function panics.
func WrapRecoverCall(fn func()) func() error { return func() error { return WithRecoverCall(fn) } }

// WithRecoverDo runs a function with a panic handler that converts
// the panic to an error.
func WithRecoverDo[T any](fn func() T) (_ T, err error) {
	defer func() { err = erc.ParsePanic(recover()) }()
	return fn(), nil
}

// WithRecoverApply runs a function with a panic handler that converts
// the panic to an error.
func WithRecoverApply[T any](op func(T), in T) (err error) {
	defer func() { err = erc.ParsePanic(recover()) }()
	op(in)
	return
}

// WithRecoverOk runs a function and returns true if there are no errors and
// no panics the bool output value is true, otherwise it is false.
func WithRecoverOk[T any](fn func() (T, error)) (zero T, ok bool) {
	defer func() { ok = ers.IsOk(erc.ParsePanic(recover())) && ok }()
	if value, err := fn(); ers.IsOk(err) {
		return value, ers.IsOk(err)
	}
	return zero, false
}

// WrapRecoverDo wraps a function that returns a single value, with
// one that returns that argument and an error if the underlying
// function panics.
func WrapRecoverDo[T any](fn func() T) func() (T, error) {
	return func() (T, error) { return WithRecoverDo(fn) }
}

// WrapRecoverOk takes a function that returns an error and a value,
// and returns a wrapped function that also catches any panics in the
// input function, and returns the value and a boolean that is true if
// the input function does not return an error or panic.
func WrapRecoverOk[T any](fn func() (T, error)) func() (T, bool) {
	return func() (T, bool) { return WithRecoverOk(fn) }
}
