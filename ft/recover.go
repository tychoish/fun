package ft

import (
	"github.com/tychoish/fun/erc"
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

// WrapRecoverDo wraps a function that returns a single value, with
// one that returns that argument and an error if the underlying
// function panics.
func WrapRecoverDo[T any](fn func() T) func() (T, error) {
	return func() (T, error) { return WithRecoverDo(fn) }
}

// WithRecoverApply runs a function with a panic handler that converts
// the panic to an error.
func WithRecoverApply[T any](op func(T), in T) (err error) {
	defer func() { err = erc.ParsePanic(recover()) }()
	op(in)
	return
}

func WrapRecoverApply[T any](op func(T), in T) func() error {
	return func() error { return WithRecoverApply(op, in) }
}

func WithRecoverFilter[T any](op func(T) T, in T) (_ T, err error) {
	defer func() { err = erc.ParsePanic(recover()) }()
	return op(in), nil
}

func WrapRecoverFilter[T any](op func(T) T, in T) func(T) (T, error) {
	return func(v T) (T, error) { return WithRecoverFilter(op, v) }
}
