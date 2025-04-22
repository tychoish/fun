package ft

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"
	"time"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/internal"
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

// NotZero returns true when the item does not have the zero value for
// the type T.
func NotZero[T comparable](in T) bool { return Not(IsZero(in)) }

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

// IsNil uses reflection to determine if an object is nil. ()
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

// IsOk returns only the second argument passed to it, given a
// function that returns two values where the second value is a
// boolean, you can use IsOk to discard the first value.
func IsOk[T any](_ T, ok bool) bool { return ok }

// Not inverts a boolean.
func Not(p bool) bool { return !p }

// IfCall is, effectively the if-form from (many) lisps: when the
// condition is true, the first function is called, and otherwise the
// second. If the appropriate function is nil, IfCall is a noop. The
// "other" function call is never called.
func IfCall(cond bool, doIf func(), doElse func()) {
	if cond {
		SafeCall(doIf)
		return
	}
	SafeCall(doElse)
}

// IfDo returns the output of the first function when the condition is
// false, and the value of the second function otherwise. If the
// appropriate function is nil, IfDo returns the zero value for the
// type. The "other" function call is never called.
func IfDo[T any](cond bool, doIf func() T, doElse func() T) T {
	if cond {
		return SafeDo(doIf)
	}
	return SafeDo(doElse)
}

// IfValue provides a ternary-like operation as a complement to IfDo
// and IfCall for values.
func IfValue[T any](cond bool, ifVal T, elseVal T) T {
	if cond {
		return ifVal
	}
	return elseVal
}

// Flip takes two arguments and returns them in the opposite
// order. Intended to wrap other functions to reduce the friction when
// briding APIs.
func Flip[A any, B any](first A, second B) (B, A) { return second, first }

// Ignore is a noop, but can be used to annotate operations rather
// than assigning to the empty identifier:
//
//	_ = operation()
//	ft.Ignore(operation())
func Ignore[T any](_ T) { return } //nolint:staticcheck

// IgnoreFirst takes two arguments and returns only the second, for
// use in wrapping functions that return two values.
func IgnoreFirst[A any, B any](_ A, b B) B { return b }

// IgnoreSecond takes two arguments and returns only the first, for
// use when wrapping functions that return two values.
func IgnoreSecond[A any, B any](a A, _ B) A { return a }

// Once uses a sync.Once to wrap to provide an output function that
// will execute at most one time, while eliding/managing the sync.Once
// object.
//
// Deprecated: Use sync.OnceFunc() from the standard library. Be aware
// that sync.OnceFunc has slightly different semantics around panic
// handling.
func Once(f func()) func() { o := &sync.Once{}; f = SafeWrap(f); return func() { o.Do(f) } }

// OnceDo returns a function, that will run exactly once. The value
// returned by the inner function is cached transparently, so
// subsequent calls to the function returned by OnceDo will return the
// original value.
//
// Deprecated: Use sync.OnceValue() from the standard library. Be aware
// that sync.OnceValue has slightly different semantics around panic
// handling.
func OnceDo[T any](op func() T) func() T {
	var cache T
	opw := Once(func() { cache = op() })
	return func() T { opw(); return cache }
}

// Contains returns true if an element of the slice is equal to the
// item.
//
// Deprecated: use slices.Contains from the standard library.
func Contains[T comparable](item T, slice []T) bool { return slices.Contains(slice, item) }

// WhenCall runs a function when condition is true, and is a noop
// otherwise. Panics if the function is nil.
func WhenCall(cond bool, op func()) { IfCall(cond, op, nil) }

// WhenDo calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type. Panics if the function is nil.
func WhenDo[T any](cond bool, op func() T) (out T) { return IfDo(cond, op, nil) }

// UnlessCall is inverse form of WhenCall, calling the provided
// function only when the conditional is false. Panics if the function
// is nil.
func UnlessCall(cond bool, op func()) { IfCall(cond, nil, op) }

// UnlessDo is the inverse form of WhenDo, calling the function only
// when the condition is false. Panics if the function is nil.
func UnlessDo[T any](cond bool, op func() T) T { return IfDo(cond, nil, op) }

// WhenApply runs the function with the supplied argument only when
// the condition is true. Panics if the function is nil.
func WhenApply[T any](cond bool, op func(T), arg T) {
	if cond {
		op(arg)
	}
}

// WhenApplyFuture resolves the future and calls the operation
// function only when the conditional is true.
func WhenApplyFuture[T any](cond bool, op func(T), arg func() T) {
	if cond {
		op(arg())
	}
}

// WhenHandle passes the argument "in" to the operation IF the
// condition function (which also takes "in") returns true. Panics if
// the function is nil.
func WhenHandle[T any](cond func(T) bool, op func(T), in T) { WhenApply(cond(in), op, in) }

// DoTimes runs the specified option n times.
func DoTimes(n int, op func()) {
	for i := 0; i < n; i++ {
		op()
	}
}

// SafeCall only calls the operation when it's non-nil.
func SafeCall(op func()) {
	if op != nil {
		op()
	}
}

// SafeDo calls the function when the operation is non-nil, and
// returns either the output of the function or the zero value of T.
func SafeDo[T any](op func() T) (out T) {
	if op != nil {
		return op()
	}
	return
}

// Convert takes a sequence of A and converts it, lazily into a
// sequence of B, using the mapper function.
func Convert[A any, B any](mapper func(A) B, values iter.Seq[A]) iter.Seq[B] {
	return func(yield func(B) bool) {
		for input := range values {
			if !yield(mapper(input)) {
				return
			}
		}
	}
}

// SafeWrap wraps an operation with SafeCall so that the resulting
// operation is never nil, and will never panic if the input operation
// is nil.
func SafeWrap(op func()) func() { return func() { SafeCall(op) } }

// Wrap produces a function that always returns the value
// provided. Useful for bridging interface paradigms, and for storing
// interface-typed objects in atomics.
func Wrap[T any](in T) func() T { return func() T { return in } }

// SafeApply calls the function, fn, on the value, arg, only if the
// function is not nil.
func SafeApply[T any](fn func(T), arg T) { WhenApply(fn != nil, fn, arg) }

// Join creates a function that iterates over all of the input
// functions and calls all non-nil functions sequentially. Nil
// functions are ignored.
func Join(fns []func()) func() { return func() { CallMany(fns) } }

// Call executes the provided function.
func Call(op func()) { op() }

// Do executes the provided function and returns its result.
func Do[T any](op func() T) T { return op() }

// ApplyMany calls the function on each of the items in the provided
// slice. ApplyMany is a noop if the function is nil.
func ApplyMany[T any](fn func(T), args []T) {
	for idx := 0; idx < len(args) && fn != nil; idx++ {
		// don't need to use the safe variant because we check
		// if it's nil in the loop
		Apply(fn, args[idx])
	}
}

// CallMany calls each of the provided function, skipping any nil functions.
func CallMany(ops []func()) {
	for idx := range ops {
		SafeCall(ops[idx])
	}
}

// DoMany calls each of the provided functions and returns an iterator
// of their results.
func DoMany[T any](ops []func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for idx := 0; idx < len(ops); idx++ {
			if op := ops[idx]; op == nil {
				continue
			} else if !yield(op()) {
				return
			}
		}
	}
}

// DoMany2 calls each of the provided functions and returns an iterator
// of their results.
func DoMany2[A any, B any](ops []func() (A, B)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for idx := 0; idx < len(ops); idx++ {
			if op := ops[idx]; op == nil {
				continue
			} else if !yield(op()) {
				return
			}
		}
	}
}

// Slice returns a slice for the variadic arguments. Useful for
// adapting functions that take slice arguments where it's easier to
// pass values variadicly.
func Slice[T any](items ...T) []T { return items }

// Apply calls the input function with the provided argument.
func Apply[T any](fn func(T), arg T) { fn(arg) }

// ApplyFuture returns a function object that, when called calls the
// input function with the provided argument.
func ApplyFuture[T any](fn func(T), arg T) func() { return func() { Apply(fn, arg) } }

// Must wraps a function that returns a value and an error, and
// converts the error to a panic.
func Must[T any](arg T, err error) T {
	WhenCall(err != nil, func() { panic(errors.Join(err, ers.ErrInvariantViolation)) })
	return arg
}

// Check can wrap a the callsite of a function that returns a value
// and an error, and returns (zero, false) if the error is non, nil,
// and (value, true) if the error is nil.
func Check[T any](value T, err error) (zero T, _ bool) {
	if err != nil {
		return zero, false
	}
	return value, true
}

// IgnoreError discards an error.
func IgnoreError(_ error) { return } //nolint

// WithTimeout runs the function, which is the same type as
// fun.Operation, with a new context that expires after the specified duration.
func WithTimeout(dur time.Duration, op func(context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()
	op(ctx)
}

// WithContext runs the function, which is the same type as a
// fun.Operation, with a new context that is canceled after the
// function exits.
func WithContext(op func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	op(ctx)
}

// MustBeOk raises an invariant violation if the ok value is false,
// and returns the first value if the second value is ok. Useful as
// in:
//
//	out := ft.MustBeOk(func() (string, bool) { return "hello world", true })
func MustBeOk[T any](out T, ok bool) T {
	WhenCall(!ok, func() { panic(fmt.Errorf("check failed: %w", ers.ErrInvariantViolation)) })
	return out
}
