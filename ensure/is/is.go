// Package is contains a simple assertion library for the fun/ensure
// testing framework. The package name is simple and chosen mostly to
// namespace a collection of operations for readability effects.
package is

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Plist is a simple constructor for making metadata pairs for the
// ensure.Assertion.Metadata() method. The return value is chainable,
// as in:
//
//	ensure.That(is.True(os.IsNotExists(err))).Metadata(
//		  is.Plist().Pair("name", name).
//			     Pair("path", path)).Run(t)
func Plist() *dt.Pairs[string, any] { return &dt.Pairs[string, any]{} }

// That is the root type for the assertion helpers in this
// package. Implementations return nil for no-errors, and one or more
// error messages for failing assertions..
type That fn.Future[[]string]

// And combines a collection of That expression. All constituent
// expressions are run in order until one fails, at which point the
// operation aborts.
func And(ops ...That) That {
	return func() []string {
		out := dt.NewSlice(make([]string, 0, len(ops)+1))
		for _, that := range ops {
			if that == nil {
				out.Add("encountered nil is.That operation")
				break
			}
			if results := that(); len(results) > 0 {
				out.Extend(results)
				break
			}
		}

		return ft.WhenDo(len(out) > 0, out.FilterFuture(ft.NotZero[string]))
	}
}

// All combines a collection of that expressions. All expressions are
// run in order, and the error are aggregated. Nil is.That expressions
// are reported as errors
func All(ops ...That) That {
	return func() []string {
		out := dt.NewSlice(make([]string, 0, len(ops)+1))
		dt.NewSlice(ops).ReadAll(func(op That) {
			out.AppendWhen(op == nil, "encountered nil is.That operation")
			out.Extend(ft.SafeDo(op))
		})
		return ft.WhenDo(len(out) > 0, out.FilterFuture(ft.NotZero[string]))
	}
}

func assert(cond bool, args ...any) That {
	return That(fn.MakeFuture(fun.MAKE.Str(args).Slice()).Not(cond).Once())
}

func assertf(cond bool, t string, a ...any) That {
	return That(fn.MakeFuture(fun.MAKE.Strf(t, a).Slice()).Not(cond).Once())
}

// EqualTo asserts that two comparable values are equal to eachother.
func EqualTo[T comparable](a, b T) That { return assertf(a == b, "%v != %v [%T]", a, b, b) }

// NotEqualTo asserts that two comparable values are not equal to eachother.
func NotEqualTo[T comparable](a, b T) That { return assertf(a != b, "%v == %v [%T]", a, b, b) }

// True asserts that the value is true.
func True(c bool) That { return assert(c, "assertion failure (true)") }

// False asserts that the value is false.
func False(c bool) That { return assert(!c, "assertion failure (false)") }

// Error asserts that the error object is not nil.
func Error(e error) That { return assert(e != nil, "expected error") }

// NotError asserts that the error object is nil.
func NotError(e error) That { return assertf(e == nil, "got error: %s", e) }

// ErrorIs asserts that the error (er) is, or unwraps to, the target
// (tr) error.
func ErrorIs(er, tr error) That { return assertf(ers.Is(er, tr), "%v is not %v [%T]", er, tr, er) }

// NotErrorIs asserts that the error (er) is not, and does not unwrap
// to, the target (tr) error.
func NotErrorIs(er, tr error) That { return assertf(!ers.Is(er, tr), "%v is not %v [%T]", er, tr, er) }

// NilPtr asserts that the pointer is nil.
func NilPtr[T any](val *T) That { return EqualTo(val, nil) }

// NotNilPtr asserts that the pointer is not nil.
func NotNilPtr[T any](val *T) That { return NotEqualTo(val, nil) }

// Nil asserts that the value is nil. Uses reflection unlike the other
// assertions.
func Nil(val any) That {
	return All(
		assertf(!ft.IsType[error](val), "use is.Error() rather than is.Nil() for errors"),
		assertf(internal.IsNil(val), "value (type=%T), was nil", val),
	)
}

// NotNil asserts that the value is not nil.
func NotNil(val any) That {
	return All(
		assertf(!ft.IsType[error](val), "use is.Error() rather than is.Nil() for errors"),
		assertf(!internal.IsNil(val), "value (type=%T), was nil", val),
	)
}

// Zero asserts that the comparable value is equal to the zero value
// for the type T.
func Zero[T comparable](val T) That { return EqualTo(val, zeroOf[T]()) }

// NotZero asserts that the comparable value is not equal to the zero
// value for the type T.
func NotZero[T comparable](val T) That { return NotEqualTo(val, zeroOf[T]()) }

func zeroOf[T any]() (out T) { return out }
func typestr[T any]() string { var zero T; return fmt.Sprintf("%T", zero) }

// Type asserts that value v is of type T.
func Type[T any](v any) That {
	return assertf(ft.IsType[T](v), "%v [%T] is not %s", v, v, typestr[T]())
}

// NotType asserts that the value v is not of type T.
func NotType[T any](v any) That {
	return assertf(!ft.IsType[T](v), "%v [%T] is  %s", v, v, typestr[T]())
}

// Substring asserts that the string (s) contains the substring (substr).
func Substring(s, substr string) That {
	return assertf(strings.Contains(s, substr), "%q is not a substring of %s", substr, s)
}

// NotSubstring asserts that the string (s) does not contain the
// substring (substr.)
func NotSubstring(s, substr string) That {
	return assert(!strings.Contains(s, substr), "%q is a substring of %s", substr, s)
}

// Contained asserts that the slice (sl) has at least one element
// equal to the item.
func Contained[T comparable](item T, sl []T) That {
	return assertf(slices.Contains(sl, item), "list (len=%d) does not contain contains %v", len(sl), item)
}

// NotContained asserts that the slice (sl) has no elements that are
// equal to the item.
func NotContained[T comparable](item T, sl []T) That {
	return assertf(!slices.Contains(sl, item), "list (len=%d) does not contain contains %v", len(sl), item)
}

// Panic asserts that the function (op) panics when executed.
func Panic(op func()) That {
	return func() (out []string) {
		defer func() {
			if recover() == nil {
				out = append(out, "saw unexpected nil panic")
			}
		}()
		op()
		return
	}
}

// NotPanic asserts that the function (op) does not panic when executed.
func NotPanic(op func()) That {
	return func() (out []string) {
		defer func() {
			if p := recover(); p != nil {
				out = append(out, fmt.Sprintf("saw unexpected panic, %v", p))
			}
		}()
		op()
		return
	}
}
