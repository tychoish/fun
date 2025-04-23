package ft

// IfValue provides a ternary-like operation as a complement to IfDo
// and IfCall for values.
func IfValue[T any](cond bool, ifVal T, elseVal T) T {
	if cond {
		return ifVal
	}
	return elseVal
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
