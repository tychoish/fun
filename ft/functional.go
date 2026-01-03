package ft

// Join creates a function that iterates over all of the input
// functions and calls all non-nil functions sequentially. Nil
// functions are ignored.
func Join(fns ...func()) func() { return func() { CallMany(fns) } }

// Call executes the provided function.
func Call(op func()) { op() }

// CallMany calls each of the provided function, skipping any nil functions.
func CallMany(ops []func()) {
	for _, item := range ops {
		CallSafe(item)
	}
}

// CallTimes runs the specified operation n times.
func CallTimes(n int, op func()) {
	for range n {
		CallSafe(op)
	}
}

// Do executes the provided function and returns its result.
func Do[T any](op func() T) T { return op() }

// Do2 executes the provided function and returns its results.
func Do2[T any, V any](op func() (T, V)) (T, V) { return op() }

// Apply calls the input function with the provided argument.
func Apply[T any](fn func(T), arg T) { fn(arg) }

// Filter calls the input function with the provided argument and returns the result.
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
