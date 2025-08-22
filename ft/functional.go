package ft

import "iter"

// Join creates a function that iterates over all of the input
// functions and calls all non-nil functions sequentially. Nil
// functions are ignored.
func Join(fns []func()) func() { return func() { CallMany(fns) } }

// Call executes the provided function.
func Call(op func()) { op() }

// Do executes the provided function and returns its result.
func Do[T any](op func() T) T { return op() }

// Do2 executes the provided function and returns its results.
func Do2[T any, V any](op func() (T, V)) (T, V) { return op() }

// Apply calls the input function with the provided argument.
func Apply[T any](fn func(T), arg T) { fn(arg) }

func Filter[T any](fn func(T) T, arg T) T { return fn(arg) }

// CallSafe only calls the operation when it's non-nil.
func CallSafe(op func()) {
	if op != nil {
		op()
	}
}

// DoSafe calls the function when the operation is non-nil, and
// returns either the output of the function or the zero value of T.
func DoSafe[T any](op func() T) (out T) {
	if op != nil {
		return op()
	}
	return
}

// ApplySafe calls the function, fn, on the value, arg, only if the
// function is not nil.
func ApplySafe[T any](fn func(T), arg T) {
	if fn != nil {
		fn(arg)
	}
}

// FilterSafe passes the arg value through the filter function. If the
// filter function is nil, then FilterSafe returns the input argument.
func FilterSafe[T any](fn func(T) T, arg T) T {
	if fn != nil {
		return fn(arg)
	}
	return arg
}

// CallMany calls each of the provided function, skipping any nil functions.
func CallMany(ops []func()) {
	for idx := range ops {
		CallSafe(ops[idx])
	}
}

// DoMany calls each of the provided functions and returns an iterator
// of their results.
func DoMany[T any](ops []func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for idx := range ops {
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
		for idx := range ops {
			if op := ops[idx]; op == nil {
				continue
			} else if !yield(op()) {
				return
			}
		}
	}
}

// ApplyMany calls the function on each of the items in the provided
// slice. ApplyMany is a noop if the function is nil.
func ApplyMany[T any](fn func(T), args []T) {
	for idx := 0; idx < len(args) && fn != nil; idx++ {
		// don't need to use the safe variant because we check
		// if it's nil in the loop
		Apply(fn, args[idx])
	}
}

// CallTimes runs the specified option n times.
func CallTimes(n int, op func()) {
	for range n {
		op()
	}
}

// ApplyTimes runs the provided function and argument pair, the
// specfied number of times.
func ApplyTimes[T any](n int, fn func(T), arg T) {
	for range n {
		fn(arg)
	}
}

// DoMany calls the provided functions n times and returns an iterator
// of their results.
func DoTimes[T any](n int, op func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for range n {
			if op == nil || !yield(op()) {
				return
			}
		}
	}
}
