package ft

import (
	"context"
	"errors"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestWhen(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		out := DoWhen(true, func() int { return 100 })
		check.Equal(t, out, 100)

		out = DoWhen(false, func() int { return 100 })
		check.Equal(t, out, 0)
	})
	t.Run("Call", func(t *testing.T) {
		called := false
		CallWhen(true, func() { called = true })
		check.True(t, called)

		called = false
		CallWhen(false, func() { called = true })
		check.True(t, !called)
	})
	t.Run("Apply", func(t *testing.T) {
		called := false
		ApplyWhen(true, func(in int) { called = true; check.Equal(t, in, 42) }, 42)
		check.True(t, called)

		called = false
		ApplyWhen(false, func(in int) { called = true; check.Equal(t, in, 42) }, 40)
		check.True(t, !called)
	})
	t.Run("Not", func(t *testing.T) {
		check.True(t, !Not(true))
		check.True(t, Not(false))
	})
	t.Run("Ternary", func(t *testing.T) {
		check.Equal(t, 100, IfElse(true, 100, 900))
		check.Equal(t, 900, IfElse(false, 100, 900))
	})
}

func TestMust(t *testing.T) {
	assert.Panic(t, func() { Must("32", ers.Error("whoop")) })
	assert.Panic(t, func() { MustBeOk("32", false) })
	assert.NotPanic(t, func() { check.Equal(t, "32", Must("32", nil)) })
	assert.NotPanic(t, func() { check.Equal(t, "32", MustBeOk("32", true)) })
}

func TestPtr(t *testing.T) {
	out := Ptr(123)
	assert.True(t, out != nil)
	check.Equal(t, *out, 123)

	var nptr *string //nolint:staticcheck
	var err error

	check.True(t, IsNil(nptr))
	check.True(t, IsNil(err))
	check.True(t, Not(IsNil(4)))
	check.True(t, Not(IsNil(t)))

	var anyif any
	check.True(t, IsNil(anyif))
	check.True(t, anyif == nil)
	anyif = nptr

	// interfaces holding nil values are not nil
	check.True(t, anyif != nil) //nolint:staticcheck
	// however...
	check.True(t, IsNil(anyif))

	// this is gross, but we have a pointer (non-nil) to an object
	// that is a pointer, which is nil.
	var dptr *string
	st := Ptr(dptr)
	assert.True(t, st != nil)
	assert.True(t, *st == nil)
	assert.Type[**string](t, st)

	check.True(t, IsPtr(dptr))
	check.True(t, IsPtr(t))
	check.True(t, Not(IsPtr(Ref(dptr))))
	check.True(t, Not(IsPtr(3)))
}

func TestDefault(t *testing.T) {
	t.Run("Static", func(t *testing.T) {
		assert.Equal(t, Default(0, 42), 42)
		assert.Equal(t, Default(77, 42), 77)

		assert.Equal(t, Default("", "kip"), "kip")
		assert.Equal(t, Default("buddy", "kip"), "buddy")
	})
	t.Run("Dynamic", func(t *testing.T) {
		count := 0
		assert.Equal(t, DefaultFuture(0, func() int { count++; return 42 }), 42)
		check.Equal(t, count, 1)
		assert.Equal(t, DefaultFuture(77, func() int { count++; return 42 }), 77)
		check.Equal(t, count, 1)

		assert.Equal(t, DefaultFuture("", func() string { count++; return "kip" }), "kip")
		check.Equal(t, count, 2)

		assert.Equal(t, DefaultFuture("buddy", func() string { count++; return "kip" }), "buddy")
		check.Equal(t, count, 2)
	})
}

func TestUnless(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		count := 0
		check.Equal(t, 0, DoUnless(true, func() int { count++; return count }))
		check.Equal(t, 1, DoUnless(false, func() int { count++; return count }))
		check.Equal(t, 2, DoUnless(false, func() int { count++; return count }))
		check.Equal(t, 0, DoUnless(true, func() int { count++; return count }))
	})
	t.Run("Call", func(t *testing.T) {
		count := 0
		CallUnless(true, func() { count++ })
		check.Equal(t, 0, count)
		CallUnless(false, func() { count++ })
		check.Equal(t, 1, count)
		CallUnless(false, func() { count++ })
		check.Equal(t, 2, count)
		CallUnless(true, func() { count++ })
		check.Equal(t, 2, count)
	})
	t.Run("Apply", func(t *testing.T) {
		count := 0
		ApplyUnless(true, func(in int) { check.Equal(t, in, 42); count++ }, 42)
		check.Equal(t, 0, count)
		ApplyUnless(false, func(in int) { check.Equal(t, in, 42); count++ }, 42)
		check.Equal(t, 1, count)
		ApplyUnless(true, func(in int) { check.Equal(t, in, 42); count++ }, 42)
		check.Equal(t, 1, count)
	})
}

func TestWrap(t *testing.T) {
	t.Run("Wrapper", func(t *testing.T) {
		assert.NotError(t, Wrap[error](nil)())
		assert.Error(t, Wrap(errors.New("Hello"))())
		assert.Equal(t, Wrap(1)(), 1)
		assert.Equal(t, Wrap("hello")(), "hello")
	})
	t.Run("Cast", func(t *testing.T) {
		var out string
		var in any = "fooo"
		var ok bool
		// the real test is if this compiles
		out, ok = Cast[string](in)
		assert.True(t, ok)
		assert.Equal(t, "fooo", out)

		in = 1234
		out, ok = Cast[string](in)
		assert.True(t, !ok)
		assert.Equal(t, "", out)
	})
	t.Run("IsType", func(t *testing.T) {
		var in any = "fooo"
		assert.True(t, IsType[string](in))
		in = 1234
		assert.True(t, !IsType[string](in))
	})
	t.Run("IsZero", func(t *testing.T) {
		assert.True(t, !IsZero(100))
		assert.True(t, !IsZero(true))
		assert.True(t, !IsZero("hello world"))
		assert.True(t, !IsZero(time.Now()))
		assert.True(t, IsZero(0))
		assert.True(t, IsZero(false))
		assert.True(t, IsZero(""))
		assert.True(t, IsZero(time.Time{}))
	})
	t.Run("NotZero", func(t *testing.T) {
		assert.True(t, NotZero(100))
		assert.True(t, NotZero(true))
		assert.True(t, NotZero("hello world"))
		assert.True(t, NotZero(time.Now()))
		assert.True(t, !NotZero(0))
		assert.True(t, !NotZero(false))
		assert.True(t, !NotZero(""))
		assert.True(t, !NotZero(time.Time{}))
	})
	t.Run("IsOk", func(t *testing.T) {
		assert.True(t, IsOk(100, true))
		assert.True(t, !IsOk(100, false))
		assert.True(t, IsOk(func() (int, bool) { return 100, true }()))
	})
	t.Run("SafeCall", func(t *testing.T) {
		count := 0
		fn := func() { count++ }
		assert.NotPanic(t, func() { CallSafe(nil) })
		assert.NotPanic(t, func() { CallSafe(fn) })
		assert.NotPanic(t, func() { CallSafe(nil) })
		assert.NotPanic(t, func() { CallSafe(fn) })
		check.Equal(t, count, 2)
	})
	t.Run("CallTimes", func(t *testing.T) {
		count := 0
		CallTimes(42, func() { count++ })
		assert.Equal(t, count, 42)
	})
	t.Run("DoTimes", func(t *testing.T) {
		t.Run("Exhaust", func(t *testing.T) {
			count := 0

			for out := range DoTimes(42, func() int { count++; return 84 }) {
				assert.Equal(t, out, 84)
			}
			assert.Equal(t, count, 42)
		})
		t.Run("Abort", func(t *testing.T) {
			count := 0

			sum := 0
			for out := range DoTimes(32, func() int { count++; return 2 }) {
				assert.Equal(t, out, 2)
				sum += out
				if sum == 32 {
					break
				}
			}
			assert.Equal(t, count, 16)
		})
	})

	t.Run("Once", func(t *testing.T) {
		count := 0
		op := func() { count++ }
		CallTimes(128, sync.OnceFunc(op))
		assert.Equal(t, count, 1)
	})
	t.Run("Flip", func(t *testing.T) {
		op := func() (int, bool) { return 42, true }
		num, ok := op()
		check.True(t, ok)
		check.Equal(t, 42, num)
		ok, num = Flip(op()) //nolint
	})
	t.Run("SafeOK", func(t *testing.T) {
		assert.True(t, !DoSafe[bool](nil))
		assert.True(t, nil == DoSafe[*bool](nil))
		assert.True(t, nil == DoSafe[*testing.T](nil))
		assert.Equal(t, 1, DoSafe(func() int { return 1 }))
		assert.Equal(t, 412, DoSafe(func() int { return 412 }))
	})
	t.Run("SafeCast", func(t *testing.T) {
		assert.Zero(t, SafeCast[int](any(0)))
		assert.True(t, SafeCast[bool](any(true)))
		assert.True(t, !SafeCast[bool](any(false)))
		assert.Equal(t, "hello world", SafeCast[string](any("hello world")))
		assert.NotZero(t, SafeCast[time.Time](time.Now()))

		foo := "foo"
		var tt testing.TB
		assert.NotZero(t, SafeCast[*string](&foo))
		assert.Zero(t, SafeCast[*testing.T](tt))
	})
	t.Run("Ref", func(t *testing.T) {
		var strptr *string
		assert.True(t, strptr == nil)
		assert.Equal(t, "", Ref(strptr))
		assert.True(t, !IsOk(RefOk(strptr)))

		strptr = Ptr("")
		assert.True(t, strptr != nil)
		assert.Equal(t, "", Ref(strptr))
		assert.True(t, IsOk(RefOk(strptr)))

		strptr = Ptr("hello")
		assert.True(t, strptr != nil)
		assert.True(t, IsOk(RefOk(strptr)))
		assert.Equal(t, "hello", Ref(strptr))
	})
	t.Run("DefaultNew", func(t *testing.T) {
		t.Run("Passthrough", func(t *testing.T) {
			val := Ptr("string value")
			newVal := DefaultNew(val)
			check.Equal(t, val, newVal)
			check.Equal(t, *val, *newVal)
		})
		t.Run("Constructor", func(t *testing.T) {
			var ts *time.Time
			check.Panic(t, func() { _ = ts.IsZero() })
			ts = DefaultNew(ts)
			check.True(t, ts != nil)
			check.True(t, ts.IsZero())
		})
	})
	t.Run("Call", func(t *testing.T) {
		assert.Panic(t, func() { Call(nil) })
		assert.Panic(t, func() { Call(func() { panic("here") }) })

		var called bool
		Call(func() { called = true })
		assert.True(t, called)
	})
	t.Run("Do", func(t *testing.T) {
		assert.Panic(t, func() { Do[int](nil) })
		assert.Panic(t, func() { Do(func() int { panic("here") }) })
	})
	t.Run("DoMany", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			for val := range DoMany(Slice(Wrap(22), Wrap(44), Wrap(66))) {
				assert.True(t, val%11 == 0)
				assert.True(t, val%22 == 0)
				assert.True(t, val < 88)
			}
			count := 0
			op := func() int { count++; return count }
			for val := range DoMany(Slice(op, op, op)) {
				assert.Equal(t, val, count)
			}

			assert.Equal(t, count, 3)
		})
		t.Run("EarlyReturn", func(t *testing.T) {
			count := 0
			for val := range DoMany(Slice(Wrap(22), Wrap(44), Wrap(66))) {
				if count == 2 {
					break
				}
				count++

				assert.True(t, val%11 == 0)
				assert.True(t, val%22 == 0)
				assert.True(t, val < 88)
			}
			assert.Equal(t, count, 2)
		})
		t.Run("SkipNil ", func(t *testing.T) {
			count := 0

			for val := range DoMany(Slice(Wrap(22), nil, Wrap(44), nil, Wrap(66))) {
				count++
				assert.NotZero(t, val)
			}
			assert.Equal(t, count, 3)
		})
	})
	t.Run("DoMany2", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			op := func() (int, bool) { count++; v := rand.Int(); return v, v%2 == 0 }
			for val, even := range DoMany2(Slice(op, op, op, op)) {
				assert.Equal(t, even, val%2 == 0)
			}
			assert.Equal(t, count, 4)
		})
		t.Run("EarlyReturn", func(t *testing.T) {
			count := 0
			op := func() (int, bool) { count++; v := rand.Int(); return v, v%2 == 0 }
			for val, even := range DoMany2(Slice(op, op, op, op)) {
				if count == 2 {
					break
				}
				assert.Equal(t, even, val%2 == 0)
			}
			assert.Equal(t, count, 2)
		})
		t.Run("SkipNil ", func(t *testing.T) {
			count := 0
			op := func() (int, bool) { count++; v := rand.Int(); return v, v%2 == 0 }
			for val, even := range DoMany2(Slice(op, op, nil, op)) {
				assert.Equal(t, even, val%2 == 0)
			}
			assert.Equal(t, count, 3)
		})
	})
	t.Run("ApplyMany", func(t *testing.T) {
		t.Run("Safety", func(t *testing.T) {
			assert.NotPanic(t, func() { ApplyMany(nil, []int{1, 2, 3}) })
			assert.NotPanic(t, func() { ApplyMany[int](nil, nil) })
		})
		t.Run("IsCalled", func(t *testing.T) {
			count := 0
			ApplyMany(func(int) { count++ }, []int{1, 2, 3})
			assert.Equal(t, count, 3)
		})
	})
	t.Run("Convert", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			total := 0
			for out := range Convert(func(a string) int { return Must(strconv.Atoi(a)) }, slices.Values([]string{"2", "2", "2"})) {
				count++
				total += out
				assert.Equal(t, 2, out)
			}
			assert.Equal(t, count, 3)
			assert.Equal(t, total, 6)
		})

		t.Run("EarlyReturn", func(t *testing.T) {
			count := 0
			total := 0
			for out := range Convert(func(a string) int { return Must(strconv.Atoi(a)) }, slices.Values([]string{"2", "2", "2"})) {
				if count == 2 {
					break
				}
				count++
				total += out
				assert.Equal(t, 2, out)
			}
			assert.Equal(t, count, 2)
			assert.Equal(t, total, 4)
		})
	})
	t.Run("SafeApply", func(t *testing.T) {
		assert.NotPanic(t, func() { ApplySafe(nil, 4) })
		assert.Panic(t, func() { ApplySafe(func(int) { panic("here") }, 4) })
		var out int
		ApplySafe(func(in int) { out = in }, 42)
		assert.Equal(t, out, 42)
	})
}

func TestContexts(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		var cc context.Context
		CallWithTimeout(10*time.Millisecond, func(ctx context.Context) {
			assert.NotError(t, ctx.Err())
			cc = ctx
			time.Sleep(100 * time.Millisecond)
			assert.Error(t, ctx.Err())
			assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		})
		assert.ErrorIs(t, cc.Err(), context.DeadlineExceeded)
	})
	t.Run("ScopeTimeout", func(t *testing.T) {
		var cc context.Context
		CallWithTimeout(10*time.Millisecond, func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
	t.Run("Scope", func(t *testing.T) {
		var cc context.Context
		CallWithContext(func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
	t.Run("Do", func(t *testing.T) {
		var cc context.Context
		assert.Equal(t, 42, DoWithContext(func(ctx context.Context) int {
			cc = ctx
			check.NotError(t, ctx.Err())
			return 42
		}))
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
}

func TestJoin(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.NotPanic(t, func() { Join(Slice[func()]()...) })
		assert.NotPanic(t, func() { Join(Slice[func()](nil, nil)...) })
		assert.NotPanic(t, func() { Join(Slice[func()]()...) })
		assert.NotPanic(t, func() { Join() })
		assert.NotPanic(t, func() { Join(nil, nil) })
	})
	t.Run("Called", func(t *testing.T) {
		t.Run("Mixed", func(t *testing.T) {
			count := 0
			fn := func() { count++ }
			Join(fn, fn, nil, fn, fn)()
			assert.Equal(t, count, 4)
		})
		t.Run("AllValid", func(t *testing.T) {
			count := 0
			fn := func() { count++ }
			Join(fn, fn, fn, fn)()
			assert.Equal(t, count, 4)
		})
	})
}

func TestError(t *testing.T) {
	t.Run("Check", func(t *testing.T) {
		value, ok := Check(100, errors.New("foo"))
		check.True(t, !ok)
		check.Equal(t, 0, value)

		value, ok = Check(100, nil)
		check.True(t, ok)
		check.Equal(t, 100, value)
	})
	t.Run("Check", func(t *testing.T) {
		out, ok := Check(41, nil)
		assert.Equal(t, out, 41)
		assert.True(t, ok)

		out, ok = Check(41, errors.New("fail"))
		assert.Zero(t, out)
		assert.True(t, !ok)
	})
	t.Run("Ignore", func(t *testing.T) {
		check.NotPanic(t, func() { Ignore(errors.New("new")) })
		check.NotPanic(t, func() { Ignore[*testing.T](nil) })
	})
}

func TestPanicProtection(t *testing.T) {
	const perr ers.Error = "panic error"

	t.Run("Wrap", func(t *testing.T) {
		t.Run("RecoverCall", func(t *testing.T) {
			fn := func() { panic(perr) }
			assert.NotPanic(t, func() {
				assert.Error(t, WrapRecoverCall(fn)())
				assert.ErrorIs(t, WrapRecoverCall(fn)(), perr)
			})
		})
		t.Run("RecoverDo", func(t *testing.T) {
			fn := func() int { panic(perr) }
			assert.NotPanic(t, func() {
				out, err := WrapRecoverDo(fn)()
				assert.Error(t, err)
				assert.Zero(t, out)
				assert.ErrorIs(t, err, perr)
			})
		})
	})
	t.Run("Helpers", func(t *testing.T) {
		t.Run("Apply", func(t *testing.T) {
			if err := WithRecoverApply(nil, 123); err == nil {
				t.Error("expected error")
			}

			if err := WithRecoverApply(func(in int) { panic(in) }, 123); err == nil {
				t.Error("expected error")
			}

			if err := WithRecoverApply(func(in int) {
				if in != 123 {
					t.Fatal(in)
				}
			}, 123); err != nil {
				t.Error(err)
			}
		})
	})
	t.Run("Check", func(t *testing.T) {
		t.Run("NoError", func(t *testing.T) {
			err := WithRecoverCall(func() { t.Log("function runs") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("WithPanic", func(t *testing.T) {
			err := WithRecoverCall(func() { panic("function runs") })
			if err == nil {
				t.Fatal(err)
			}
			errText := err.Error()
			if !strings.Contains(errText, "recovered panic") {
				t.Error(err, "!=", errText)
			}

			if !strings.Contains(errText, "function runs") {
				t.Error(err, "!=", errText)
			}
		})
	})
	t.Run("SafeWithPanic", func(t *testing.T) {
		ok, err := WithRecoverDo(func() bool {
			panic(errors.New("error"))
		})
		assert.Error(t, err)
		check.True(t, !ok)
	})
	t.Run("SafeOK", func(t *testing.T) {
		t.Run("Not", func(t *testing.T) {
			num, ok := Check(WithRecoverDo(func() int { panic("error?") }))
			assert.True(t, !ok)
			assert.Zero(t, num)
		})
		t.Run("Passes", func(t *testing.T) {
			num, ok := Check(WithRecoverDo(func() int { return 42 }))
			assert.True(t, ok)
			assert.Equal(t, 42, num)
		})
	})
	t.Run("Recover", func(t *testing.T) {
		var called bool
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrRecoveredPanic)
			called = true
		}
		assert.NotPanic(t, func() {
			defer Recover(ob)
			assert.True(t, !called)
			panic("hi")
		})
		assert.True(t, called)
	})
	t.Run("ApplyTimes", func(t *testing.T) {
		inc := 0
		ApplyTimes(10, func(n int) { inc++; check.Equal(t, n, 42) }, 42)
		assert.Equal(t, 10, inc)
	})
	t.Run("Ignore", func(t *testing.T) {
		t.Run("FirstAndSecond", func(t *testing.T) {
			const first int = 42
			const second bool = true
			t.Run("First", func(t *testing.T) {
				assert.Equal(t, second, IgnoreFirst(func() (int, bool) { return first, second }()))
			})
			t.Run("Second", func(t *testing.T) {
				assert.Equal(t, first, IgnoreSecond(func() (int, bool) { return first, second }()))
			})
		})
		t.Run("Single", func(t *testing.T) {
			called := 0
			Ignore(func() int { called++; return 1 }())
			assert.Equal(t, called, 1)
		})
	})
	t.Run("DefautlApply", func(t *testing.T) {
		ct := 0
		op := func(in int) int { ct++; check.Equal(t, in, 42); return 42 * 2 }
		out := DefaultApply(99, op, 42)
		check.Equal(t, 1, ct)
		check.Equal(t, out, 84)

		out = DefaultApply(0, op, 42)
		check.Equal(t, 1, ct)
		check.Equal(t, out, 0)
	})
	t.Run("Filter", func(t *testing.T) {
		t.Run("Safe", func(t *testing.T) {
			var out int
			assert.NotPanic(t, func() {
				out = FilterSafe(nil, 44)
			})

			assert.Equal(t, out, 44)
		})
		t.Run("PanicNormal", func(t *testing.T) {
			var out int
			assert.Panic(t, func() {
				out = Filter(nil, 44)
			})

			assert.Equal(t, out, 0)
		})
		t.Run("BasicSafe", func(t *testing.T) {
			out := FilterSafe(func(in int) int { return in - 2 }, 44)

			assert.Equal(t, out, 42)
		})
		t.Run("Basic", func(t *testing.T) {
			out := Filter(func(in int) int { return in - 2 }, 44)

			assert.Equal(t, out, 42)
		})
	})
}
