package is

import (
	"fmt"
	"strings"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Thats should be boolean predicates (or return string ptrs, and nil
// on pass?) to let fatalness happen on the assertion.
type That func() *string

func (t That) Run() *string { return t() }

func (t That) Append(next That) That {
	return func() *string {
		a := t()
		if a == nil {
			return next()
		}
		b := next()
		if b == nil {
			return a
		}

		return ft.Ptr(strings.Join([]string{ft.Ref(a), ft.Ref(b)}, ";\n"))
	}
}

func (t That) Join(ops ...That) That {
	return t.Append(func() *string {
		out := []string{}

		for _, op := range ops {
			val := op()
			if ft.Ref(val) != "" {
				out = append(out, *val)
			}
		}
		if len(out) > 0 {
			return ft.Ptr(strings.Join(out, ";\n"))
		}
		return nil
	})
}

func when(cond bool, args ...any) That {
	return ft.OnceDo(func() *string {
		return ft.WhenDo(cond, func() *string { return ft.Ptr(fmt.Sprint(args...)) })
	})
}

func whenf(cond bool, tmpl string, args ...any) That {
	return ft.OnceDo(func() *string {
		return ft.WhenDo(cond, func() *string { return ft.Ptr(fmt.Sprintf(tmpl, args...)) })
	})
}

func typestr[T any]() string { var zero T; return fmt.Sprintf("%T", zero) }

func EqualTo[T comparable](a, b T) That    { return whenf(a != b, "%v != %v [%T]", a, b, b) }
func NotEqualTo[T comparable](a, b T) That { return whenf(a == b, "%v == %v [%T]", a, b, b) }
func True(c bool) That                     { return when(!c, "assertion failure (true)") }
func False(c bool) That                    { return when(c, "assertion failure (false)") }
func Erorr(e error) That                   { return when(e != nil, "expected error") }
func NotErorr(e error) That                { return whenf(e == nil, "got error: %s", e) }
func ErrorIs(er, is error) That            { return whenf(!ers.Is(er, is), "%v is not %v [%T]", er, is, is) }
func NotErrorIs(er, is error) That         { return whenf(ers.Is(er, is), "%v is not %v [%T]", er, is, is) }
func Type[T any](v any) That               { return whenf(ft.IsType[T](v), "%v [%T] is not %T", v, v, typestr[T]()) }
func NotType[T any](v any) That            { return whenf(ft.IsType[T](v), "%v [%T] is  %T", v, v, typestr[T]()) }
func Nil[T any](val *T) That               { return EqualTo(val, nil) }
func NotNil[T any](val *T) That            { return NotEqualTo(val, nil) }
func Zero[T comparable](val T) That        { return EqualTo(val, zeroOf[T]()) }
func NotZero[T comparable](val T) That     { return NotEqualTo(val, zeroOf[T]()) }

func zeroOf[T any]() (out T) { return out }

func Substring(s, substr string) That {
	return whenf(strings.Contains(s, substr), "%q is not a substring of %s", substr, s)
}

func Contained[T comparable](item T, list []T) That {
	return whenf(!ft.Contains(item, list), "list (len=%d) does not contain contains %v", len(list), item)
}

func Panic(op func()) That {
	return func() (out *string) {
		defer func() {
			p := recover()
			if p != nil {
				// expected panic
				return
			}

			out = ft.Ptr("saw unexpected non-nil panic")
		}()
		op()
		return
	}
}

func NotPanic(op func()) That {
	return func() (out *string) {
		defer func() {
			p := recover()
			if p == nil {
				// no panic
				return
			}

			out = ft.Ptr(fmt.Sprintf("saw unexpected panic, %v", p))
		}()
		op()
		return
	}
}
