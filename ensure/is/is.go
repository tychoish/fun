package is

import (
	"fmt"
	"strings"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

func Plist() *dt.Pairs[string, any] { return &dt.Pairs[string, any]{} }

type That func() []string

func (t That) Run() []string { return t() }
func (t That) Once() That    { return ft.OnceDo(t) }
func (t That) Join(ops ...That) That {
	return func() []string {
		out := dt.Sliceify(make([]string, 0, len(ops)+1))
		out.Extend(ft.SafeDo(t))
		dt.Sliceify(ops).Observe(func(op That) { out.Extend(ft.SafeDo(op)) })

		if len(out) == 0 {
			return nil
		}

		return out.Filter(ft.NotZero[string])
	}
}

func resp(args []any) That         { return func() []string { return []string{fmt.Sprint(args...)} } }
func respf(t string, a []any) That { return func() []string { return []string{fmt.Sprintf(t, a...)} } }
func that(c bool, a []any) That    { return func() []string { return ft.WhenDo(!c, resp(a)) } }
func thatf(c bool, t string, a []any) That {
	return func() []string { return ft.WhenDo(!c, respf(t, a)) }
}

func zeroOf[T any]() (out T) { return out }
func typestr[T any]() string { var zero T; return fmt.Sprintf("%T", zero) }

func assert(cond bool, args ...any) That            { return that(cond, args).Once() }
func assertf(cond bool, tmpl string, a ...any) That { return thatf(cond, tmpl, a).Once() }

func EqualTo[T comparable](a, b T) That    { return assertf(a == b, "%v != %v [%T]", a, b, b) }
func NotEqualTo[T comparable](a, b T) That { return assertf(a != b, "%v == %v [%T]", a, b, b) }
func True(c bool) That                     { return assert(c, "assertion failure (true)") }
func False(c bool) That                    { return assert(!c, "assertion failure (false)") }
func Error(e error) That                   { return assert(e != nil, "expected error") }
func NotError(e error) That                { return assertf(e == nil, "got error: %s", e) }
func ErrorIs(er, is error) That            { return assertf(ers.Is(er, is), "%v is not %v [%T]", er, is, is) }
func NotErrorIs(er, is error) That         { return assertf(!ers.Is(er, is), "%v is not %v [%T]", er, is, is) }
func Nil[T any](val *T) That               { return EqualTo(val, nil) }
func NotNil[T any](val *T) That            { return NotEqualTo(val, nil) }
func Zero[T comparable](val T) That        { return EqualTo(val, zeroOf[T]()) }
func NotZero[T comparable](val T) That     { return NotEqualTo(val, zeroOf[T]()) }

func Type[T any](v any) That {
	return assertf(ft.IsType[T](v), "%v [%T] is not %T", v, v, typestr[T]())
}
func NotType[T any](v any) That {
	return assertf(!ft.IsType[T](v), "%v [%T] is  %T", v, v, typestr[T]())
}

func Substring(s, substr string) That {
	return assertf(strings.Contains(s, substr), "%q is not a substring of %s", substr, s)
}

func NotSubstring(s, substr string) That {
	return assert(!strings.Contains(s, substr), "%q is a substring of %s", substr, s)
}

func Contained[T comparable](item T, list []T) That {
	return assertf(ft.Contains(item, list), "list (len=%d) does not contain contains %v", len(list), item)
}

func NotContained[T comparable](item T, list []T) That {
	return assertf(!ft.Contains(item, list), "list (len=%d) does not contain contains %v", len(list), item)
}

func Panic(op func()) That {
	return func() (out []string) {
		defer func() {
			if recover() == nil {
				out = append(out, "saw unexpected non-nil panic")
			}
		}()
		op()
		return
	}
}

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
