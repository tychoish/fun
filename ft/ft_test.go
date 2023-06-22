package ft

import (
	"errors"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
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

	// this is gross, but we have a pointer (non-nil) to an object
	// that is a pointer, which is nil.

	var dptr *string
	st := Ptr(dptr)
	assert.True(t, st != nil)
	assert.True(t, *st == nil)
	assert.Type[**string](t, st)
}

func TestDefault(t *testing.T) {
	assert.Equal(t, Default(0, 42), 42)
	assert.Equal(t, Default(77, 42), 77)

	assert.Equal(t, Default("", "kip"), "kip")
	assert.Equal(t, Default("merlin", "kip"), "merlin")
}

func TestWrap(t *testing.T) {
	t.Run("Wrapper", func(t *testing.T) {
		assert.NotError(t, Wrapper[error](nil)())
		assert.Error(t, Wrapper(errors.New("Hello"))())
		assert.Equal(t, Wrapper(1)(), 1)
		assert.Equal(t, Wrapper("hello")(), "hello")
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
	t.Run("IsOk", func(t *testing.T) {
		assert.True(t, IsOK(100, true))
		assert.True(t, !IsOK(100, false))
		assert.True(t, IsOK(func() (int, bool) { return 100, true }()))
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
		ok, num = Flip(op())
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
}
