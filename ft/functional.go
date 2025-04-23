package ft

import "iter"

// Call executes the provided function.
func Call(op func()) { op() }

// Do executes the provided function and returns its result.
func Do[T any](op func() T) T { return op() }

// Apply calls the input function with the provided argument.
func Apply[T any](fn func(T), arg T) { fn(arg) }

// SafeDo calls the function when the operation is non-nil, and
// returns either the output of the function or the zero value of T.
func SafeDo[T any](op func() T) (out T) {
	if op != nil {
		return op()
	}
	return
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

// ApplyFuture returns a function object that, when called calls the
// input function with the provided argument.
func ApplyFuture[T any](fn func(T), arg T) func() { return func() { Apply(fn, arg) } }

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
