package ers

import (
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestPanics(t *testing.T) {
	t.Run("SafeWithPanic", func(t *testing.T) {
		ok, err := WithRecoverDo(func() bool {
			panic(errors.New("error"))
		})
		assert.Error(t, err)
		check.True(t, !ok)
	})
	t.Run("Recover", func(t *testing.T) {
		var called bool
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, ErrRecoveredPanic)
			called = true
		}
		assert.NotPanic(t, func() {
			defer Recover(ob)
			assert.True(t, !called)
			panic("hi")
		})
		assert.True(t, called)
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
			if err.Error() != "recovered panic: function runs" {
				t.Error(err)
			}
		})
	})
	t.Run("ParsePanic", func(t *testing.T) {
		t.Run("NilInput", func(t *testing.T) {
			err := ParsePanic(nil)
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("ErrorSlice", func(t *testing.T) {
			err := ParsePanic([]error{New("one"), WithTime(New("two"))})
			if err == nil {
				t.Fatal("expected error")
			}
			check.Equal(t, 3, len(Unwind(err)))
		})
		t.Run("ArbitraryObject", func(t *testing.T) {
			err := ParsePanic(t)
			if err == nil {
				t.Fatal("expected error")
			}

			check.Substring(t, err.Error(), "testing.T")
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
		})

		t.Run("TwoErrors", func(t *testing.T) {
			err := ParsePanic(io.EOF)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, io.EOF) {
				t.Error("not EOF", err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
		})
		t.Run("NotErrorObject", func(t *testing.T) {
			err := ParsePanic("EOF")
			if err == nil {
				t.Fatal("expected error")
			}
			if errors.Is(err, io.EOF) {
				t.Error(err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
			if err.Error() != "recovered panic: EOF" {
				t.Error(err)
			}
		})
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
	t.Run("WrapRecover", func(t *testing.T) {
		const perr Error = "panic error"

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

}
