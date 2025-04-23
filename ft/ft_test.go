package ft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestWhen(t *testing.T) {
	t.Run("Do", func(t *testing.T) {
		out := WhenDo(true, func() int { return 100 })
		check.Equal(t, out, 100)

		out = WhenDo(false, func() int { return 100 })
		check.Equal(t, out, 0)
	})
	t.Run("Call", func(t *testing.T) {
		called := false
		WhenCall(true, func() { called = true })
		check.True(t, called)

		called = false
		WhenCall(false, func() { called = true })
		check.True(t, !called)
	})
	t.Run("Apply", func(t *testing.T) {
		called := false
		WhenApply(true, func(in int) { called = true; check.Equal(t, in, 42) }, 42)
		check.True(t, called)

		called = false
		WhenApply(false, func(in int) { called = true; check.Equal(t, in, 42) }, 40)
		check.True(t, !called)
	})
	t.Run("ApplyFuture", func(t *testing.T) {
		called := false
		futureCalled := false
		WhenApplyFuture(true, func(in int) { called = true; check.Equal(t, in, 42) }, func() int { futureCalled = true; return 42 })
		check.True(t, called)
		check.True(t, futureCalled)

		called = false
		futureCalled = false
		WhenApplyFuture(false, func(in int) { called = true; check.Equal(t, in, 42) }, func() int { futureCalled = true; return 42 })
		check.True(t, Not(called))
		check.True(t, Not(futureCalled))
	})

	t.Run("Not", func(t *testing.T) {
		check.True(t, !Not(true))
		check.True(t, Not(false))
	})
	t.Run("Ternary", func(t *testing.T) {
		check.Equal(t, 100, IfValue(true, 100, 900))
		check.Equal(t, 900, IfValue(false, 100, 900))
	})
}

func TestMust(t *testing.T) {
	assert.Panic(t, func() { Must("32", ers.Error("whoop")) })
	assert.Panic(t, func() { MustBeOk("32", false) })
	assert.NotPanic(t, func() { check.Equal(t, "32", Must("32", nil)) })
	assert.NotPanic(t, func() { check.Equal(t, "32", MustBeOk("32", true)) })
}

func TestContains(t *testing.T) {
	t.Run("Exists", func(t *testing.T) {
		assert.True(t, Contains(1, []int{12, 3, 44, 1}))
	})
	t.Run("NotExists", func(t *testing.T) {
		assert.True(t, !Contains(1, []int{12, 3, 44}))
	})
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
		check.Equal(t, 0, UnlessDo(true, func() int { count++; return count }))
		check.Equal(t, 1, UnlessDo(false, func() int { count++; return count }))
		check.Equal(t, 2, UnlessDo(false, func() int { count++; return count }))
		check.Equal(t, 0, UnlessDo(true, func() int { count++; return count }))
	})
	t.Run("Call", func(t *testing.T) {
		count := 0
		UnlessCall(true, func() { count++ })
		check.Equal(t, 0, count)
		UnlessCall(false, func() { count++ })
		check.Equal(t, 1, count)
		UnlessCall(false, func() { count++ })
		check.Equal(t, 2, count)
		UnlessCall(true, func() { count++ })
		check.Equal(t, 2, count)
	})
}

func TestWhenHandle(t *testing.T) {
	called := false
	assert.True(t, !called)
	WhenHandle(func(in int) bool { return in == 42 }, func(in int) { WhenCall(in == 42, func() { called = true }) }, 100)
	assert.True(t, !called)
	WhenHandle(func(in int) bool { return in == 42 }, func(in int) { WhenCall(in == 42, func() { called = true }) }, 42)
	assert.True(t, called)
}

func TestApply(t *testing.T) {
	called := 0
	out := ApplyFuture(func(n int) { called++; check.Equal(t, 42, n) }, 42)
	check.Equal(t, 0, called)
	out()
	check.Equal(t, 1, called)
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
		assert.NotPanic(t, func() { SafeCall(nil) })
		assert.NotPanic(t, func() { SafeCall(fn) })
		assert.NotPanic(t, func() { SafeCall(nil) })
		assert.NotPanic(t, func() { SafeCall(fn) })
		check.Equal(t, count, 2)
	})
	t.Run("DoTimes", func(t *testing.T) {
		count := 0
		DoTimes(42, func() { count++ })
		assert.Equal(t, count, 42)
	})
	t.Run("SafeWrap", func(t *testing.T) {
		var f func()
		assert.NotPanic(t, SafeWrap(f))
		assert.Panic(t, f)

		var called bool
		f = func() { called = true }
		SafeWrap(f)()
		assert.True(t, called)
	})
	t.Run("Once", func(t *testing.T) {
		count := 0
		op := func() { count++ }
		DoTimes(128, Once(op))
		assert.Equal(t, count, 1)
	})
	t.Run("Flip", func(t *testing.T) {
		op := func() (int, bool) { return 42, true }
		num, ok := op()
		check.True(t, ok)
		check.Equal(t, 42, num)
		ok, num = Flip(op()) //nolint
	})
	t.Run("Ignore", func(t *testing.T) {
		const first int = 42
		const second bool = true
		t.Run("First", func(t *testing.T) {
			assert.Equal(t, second, IgnoreFirst(func() (int, bool) { return first, second }()))
		})
		t.Run("Second", func(t *testing.T) {
			assert.Equal(t, first, IgnoreSecond(func() (int, bool) { return first, second }()))
		})
	})
	t.Run("SafeOK", func(t *testing.T) {
		assert.True(t, !SafeDo[bool](nil))
		assert.True(t, nil == SafeDo[*bool](nil))
		assert.True(t, nil == SafeDo[*testing.T](nil))
		assert.Equal(t, 1, SafeDo(func() int { return 1 }))
		assert.Equal(t, 412, SafeDo(func() int { return 412 }))
	})
	t.Run("OnceDo", func(t *testing.T) {
		t.Parallel()
		count := &atomic.Int64{}
		mfn := OnceDo(func() int { count.Add(1); return 42 })
		wg := &sync.WaitGroup{}
		for i := 0; i < 64; i++ {
			wg.Add(1)
			// this function panics rather than
			// asserts because it's very likely to
			// be correct, and to avoid testing.T
			// mutexes.
			go func() {
				defer wg.Done()
				for i := 0; i < 64; i++ {
					if val := mfn(); val != 42 {
						panic(fmt.Errorf("mnemonic function produced %d not 42", val))
					}
				}
			}()
		}
		wg.Wait()
		assert.Equal(t, count.Load(), 1)
	})
	t.Run("SafeCast", func(t *testing.T) {
		assert.Zero(t, SafeCast[int](any(0)))
		assert.True(t, SafeCast[bool](any(true)))
		assert.True(t, !SafeCast[bool](any(false)))
		assert.Equal(t, "hello world", SafeCast[string](any("hello world")))
		assert.NotZero(t, SafeCast[time.Time](time.Now()))

		var foo = "foo"
		var tt testing.TB
		assert.NotZero(t, SafeCast[*string](&foo))
		assert.Zero(t, SafeCast[*testing.T](tt))
	})
	t.Run("Ignore", func(t *testing.T) {
		called := 0
		Ignore(func() int { called++; return 1 }())
		assert.Equal(t, called, 1)
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
		assert.NotPanic(t, func() { SafeApply(nil, 4) })
		assert.Panic(t, func() { SafeApply(func(int) { panic("here") }, 4) })
		var out int
		SafeApply(func(in int) { out = in }, 42)
		assert.Equal(t, out, 42)
	})

}

func TestContexts(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		var cc context.Context
		WithTimeout(10*time.Millisecond, func(ctx context.Context) {
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
		WithTimeout(10*time.Millisecond, func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
	t.Run("Scope", func(t *testing.T) {
		var cc context.Context
		WithContext(func(ctx context.Context) {
			cc = ctx
			assert.NotError(t, ctx.Err())
		})
		assert.ErrorIs(t, cc.Err(), context.Canceled)
	})
}

func TestJoin(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		assert.NotPanic(t, func() { Join(Slice[func()]()) })
		assert.NotPanic(t, func() { Join(Slice[func()](nil, nil)) })
	})
	t.Run("Called", func(t *testing.T) {
		count := 0
		fn := func() { count++ }
		Join(Slice(fn, fn, nil, fn, fn))()
		assert.Equal(t, count, 4)
	})
}

func TestError(t *testing.T) {
	t.Run("Check", func(t *testing.T) {
		out, ok := Check(41, nil)
		assert.Equal(t, out, 41)
		assert.True(t, ok)

		out, ok = Check(41, errors.New("fail"))
		assert.Zero(t, out)
		assert.True(t, !ok)
	})

	t.Run("Ignore", func(t *testing.T) {
		check.NotPanic(t, func() { IgnoreError(errors.New("new")) })
		check.NotPanic(t, func() { IgnoreError(nil) })
	})
}

func TestPanicProtection(t *testing.T) {
	const perr ers.Error = "panic error"

	t.Run("Wrap", func(t *testing.T) {
		t.Run("Call", func(t *testing.T) {
			fn := func() { panic(perr) }
			assert.NotPanic(t, func() {
				assert.Error(t, WrapRecoverCall(fn)())
				assert.ErrorIs(t, WrapRecoverCall(fn)(), perr)
			})
		})
		t.Run("Do", func(t *testing.T) {
			fn := func() int { panic(perr) }
			assert.NotPanic(t, func() {
				out, err := WrapRecoverDo(fn)()
				assert.Error(t, err)
				assert.Zero(t, out)
				assert.ErrorIs(t, err, perr)
			})
		})
		t.Run("OK", func(t *testing.T) {
			fn := func() (int, error) { panic(perr) }
			assert.NotPanic(t, func() {
				out, ok := WrapRecoverOk(fn)()
				assert.True(t, !ok)
				assert.Zero(t, out)
			})
		})
		t.Run("Happy", func(t *testing.T) {
			assert.NotPanic(t, func() {
				assert.NotError(t, WrapRecoverCall(func() {})())
				out, err := WrapRecoverDo(func() int { return 42 })()
				assert.Equal(t, out, 42)
				assert.NotError(t, err)
				out, ok := WrapRecoverOk(func() (int, error) { return 12, nil })()
				assert.Equal(t, out, 12)
				assert.True(t, ok)
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
			num, ok := WithRecoverOk(func() (int, error) { return 42, io.EOF })
			assert.True(t, !ok)
			assert.Zero(t, num)
		})
		t.Run("Passes", func(t *testing.T) {
			num, ok := WithRecoverOk(func() (int, error) { return 42, nil })
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
}
