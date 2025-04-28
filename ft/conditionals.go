package ft

// IfElse provides a ternary-like operation as a complement to IfDo
// and IfCall for values.
func IfElse[T any](cond bool, ifVal T, elseVal T) T {
	if Is(cond) {
		return ifVal
	}
	return elseVal
}

// CallIfElse is, effectively the if-form from (many) lisps: when the
// condition is true, the first function is called, and otherwise the
// second. If the appropriate function is nil, CallIfElse is a noop. The
// "other" function call is never called.
func CallIfElse(cond bool, doIf func(), doElse func()) {
	if Is(cond) {
		CallSafe(doIf)
		return
	}
	CallSafe(doElse)
}

// DoIfElse returns the output of the first function when the condition is
// false, and the value of the second function otherwise. If the
// appropriate function is nil, DoIfElse returns the zero value for the
// type. The "other" function call is never called.
func DoIfElse[T any](cond bool, doIf func() T, doElse func() T) T {
	if Is(cond) {
		return DoSafe(doIf)
	}
	return DoSafe(doElse)
}

// IsOk returns only the second argument passed to it, given a
// function that returns two values where the second value is a
// boolean, you can use IsOk to discard the first value.
func IsOk[T any](_ T, ok bool) bool { return ok }

// Not inverts a boolean.
func Not(p bool) bool { return !p }

// Is is a verbose boolean.
func Is(p bool) bool { return p }

// CallWhen runs a function when condition is true, and is a noop
// otherwise. Panics if the function is nil.
func CallWhen(cond bool, op func()) { CallIfElse(cond, op, nil) }

// DoWhen calls the function when the condition is true, and returns
// the result, or if the condition is false, the operation is a noop,
// and returns zero-value for the type. Panics if the function is nil.
func DoWhen[T any](cond bool, op func() T) (out T) { return DoIfElse(cond, op, nil) }

// ApplyWhen runs the function with the supplied argument only when
// the condition is true. Panics if the function is nil.
func ApplyWhen[T any](cond bool, op func(T), arg T) {
	if Is(cond) {
		op(arg)
	}
}

// CallUnless is inverse form of CallWhen, calling the provided
// function only when the conditional is false. Panics if the function
// is nil.
func CallUnless(cond bool, op func()) { CallIfElse(cond, nil, op) }

// DoUnless is the inverse form of DoWhen, calling the function only
// when the condition is false. Panics if the function is nil.
func DoUnless[T any](cond bool, op func() T) T { return DoIfElse(cond, nil, op) }

// ApplyUnless runs the function with the supplied argument only when
// the condition is true. Panics if the function is nil.
func ApplyUnless[T any](cond bool, op func(T), arg T) {
	if !cond {
		op(arg)
	}
}
