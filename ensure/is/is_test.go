package is

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
)

func TestFundamentals(t *testing.T) {
	t.Parallel()
	t.Run("Assert", func(t *testing.T) {
		check.True(t, assert(false, "hello")() != nil)
		check.True(t, assert(true, "world")() == nil)
		check.Equal(t, len(assert(false, "world")()), 1)
		check.Equal(t, len(assert(true, "hello")()), 0)
	})
	t.Run("NilSemantics", func(t *testing.T) {
		t.Run("Append", func(t *testing.T) {
			var ex []string
			check.True(t, ex == nil)

			ex = append(ex, ex...)
			check.True(t, ex == nil)

			exx := []string{}
			check.True(t, exx != nil)

			ex = append(ex, exx...)
			check.True(t, ex == nil)
		})
		t.Run("Sliceify", func(t *testing.T) {
			var base []string
			ex := dt.Sliceify(base)
			check.True(t, ex == nil)

			ex.Extend(base)
			check.True(t, ex == nil)

			exx := []string{}
			check.True(t, exx != nil)

			ex.Extend(exx)
			check.True(t, ex == nil)
		})
	})
	t.Run("Plist", func(t *testing.T) {
		metadata := Plist()
		check.True(t, metadata != nil)
		check.Equal(t, metadata.Len(), 0)
		metadata.Add("one", 2)
		metadata.Add("two", 3)
		check.Equal(t, metadata.Len(), 2)

	})
}

func isNil(t *testing.T, fn That) {
	t.Helper()
	out := fn()
	if out != nil {
		t.Error(out)
	}
}

func notNil(t *testing.T, fn That) {
	t.Helper()
	if fn() == nil {
		t.Error("expected errors")
	}
}

func TestAssertions(t *testing.T) {
	t.Parallel()
	t.Run("Passing", func(t *testing.T) {
		isNil(t, Contained(400, []int{400, 300, 200}))
		isNil(t, EqualTo("abc", "abc"))
		isNil(t, EqualTo(1, 1))
		isNil(t, Error(errors.New("error")))
		isNil(t, Error(fmt.Errorf("error%s", "f")))
		isNil(t, Error(io.EOF))
		isNil(t, ErrorIs(fmt.Errorf("hi: %w", io.EOF), io.EOF))
		isNil(t, False(false))
		isNil(t, NilPtr[*testing.T](nil))
		isNil(t, Nil(nil))
		isNil(t, NotNil(t))
		isNil(t, NotContained(42, []int{400, 300, 200}))
		isNil(t, NotEqualTo("abc", "def"))
		isNil(t, NotEqualTo(100, 1000))
		isNil(t, NotError(nil))
		isNil(t, NotError(zeroOf[error]()))
		isNil(t, NotErrorIs(errors.New("hi"), io.EOF))
		isNil(t, NotNilPtr(t))
		isNil(t, NotPanic(func() {}))
		isNil(t, NotSubstring("beeps", "honk"))
		isNil(t, NotType[int]("hi"))
		isNil(t, NotZero("hi"))
		isNil(t, NotZero(100))
		isNil(t, NotZero(t))
		isNil(t, NotZero(true))
		isNil(t, Panic(func() { panic("expected") }))
		isNil(t, Substring("boops", "oops"))
		isNil(t, True(true))
		isNil(t, Type[int](100))
		isNil(t, Type[string]("hi"))
		isNil(t, Zero(""))
		isNil(t, Zero(0))
		isNil(t, Zero(0.0))
		isNil(t, Zero(false))
		isNil(t, Zero[*testing.T](nil))
	})
	t.Run("Failing", func(t *testing.T) {
		notNil(t, Contained(42, []int{400, 300, 200}))
		notNil(t, EqualTo("abc", "def"))
		notNil(t, EqualTo(100, 1000))
		notNil(t, Error(nil))
		notNil(t, Error(zeroOf[error]()))
		notNil(t, ErrorIs(errors.New("hi"), io.EOF))
		notNil(t, False(true))
		notNil(t, NilPtr(t))
		notNil(t, NilPtr(t))
		notNil(t, NotNil(nil))
		notNil(t, NotNil(io.EOF)) // because error
		notNil(t, Nil(t))
		notNil(t, NotContained(400, []int{400, 300, 200}))
		notNil(t, NotEqualTo("abc", "abc"))
		notNil(t, NotEqualTo(1, 1))
		notNil(t, NotError(errors.New("error")))
		notNil(t, NotError(fmt.Errorf("error%s", "f")))
		notNil(t, NotError(io.EOF))
		notNil(t, NotErrorIs(fmt.Errorf("hi: %w", io.EOF), io.EOF))
		notNil(t, NotPanic(func() { panic("expected") }))
		notNil(t, NotSubstring("boops", "oops"))
		notNil(t, NotType[int](100))
		notNil(t, NotType[string]("hi"))
		notNil(t, NotZero(""))
		notNil(t, NotZero(0))
		notNil(t, NotZero(0.0))
		notNil(t, NotZero(false))
		notNil(t, NotZero[*testing.T](nil))
		notNil(t, Panic(func() {}))
		notNil(t, Substring("beeps", "honk"))
		notNil(t, True(false))
		notNil(t, Type[int]("hi"))
		notNil(t, Zero("hi"))
		notNil(t, Zero(100))
		notNil(t, Zero(t))
		notNil(t, Zero(true))
	})
	t.Run("And", func(t *testing.T) {
		t.Run("NilSafe", func(t *testing.T) {
			op := And(nil, nil)
			out := op()
			check.Equal(t, len(out), 1)
		})
		t.Run("NoError", func(t *testing.T) {
			called := 0
			var op That
			op = func() []string { called++; return nil }
			op = And(op, op, op, op, op, op, op)
			out := op()
			check.Equal(t, len(out), 0)
			check.True(t, out == nil)
			check.Equal(t, 7, called)
		})
		t.Run("EarlyError", func(t *testing.T) {
			called := 0
			var op That
			op = func() []string { called++; return []string{"hello"} }
			op = And(op, op, op, op, op, op, op)
			out := op()
			check.Equal(t, len(out), 1)
			check.True(t, out != nil)
			check.Equal(t, 1, called)
		})
	})
	t.Run("All", func(t *testing.T) {
		t.Run("NilSafe", func(t *testing.T) {
			op := All(nil, nil)
			out := op()
			check.Equal(t, len(out), 2)
		})
		t.Run("SingleErrors", func(t *testing.T) {
			called := 0
			var op That
			op = func() []string { called++; return []string{t.Name()} }
			op = All(op, op, op, op, op, op, op)
			out := op()
			check.True(t, out != nil)
			check.Equal(t, len(out), 7)
			check.Equal(t, 7, called)
		})
		t.Run("MultiErrors", func(t *testing.T) {
			called := 0
			var op That
			op = func() []string { called++; return []string{"failure", fmt.Sprint(called), t.Name()} }
			op = All(op, op, op, op, op, op, op)
			out := op()
			check.True(t, out != nil)
			check.Equal(t, len(out), 21)
			check.Equal(t, 7, called)
		})
	})
}
