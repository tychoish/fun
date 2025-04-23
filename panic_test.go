package fun

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/ft"
)

func TestPanics(t *testing.T) {
	t.Run("MustNoPanic", func(t *testing.T) {
		ok := ft.Must(func() (bool, error) {
			return true, nil
		}())
		assert.True(t, ok)
	})
	t.Run("SafeNoPanic", func(t *testing.T) {
		ok, err := ft.WithRecoverDo(func() bool {
			return ft.Must(func() (bool, error) {
				return true, nil
			}())
		})
		if err != nil {
			t.Error("error should be non-nil")
		}
		if !ok {
			t.Error("should be zero value of T")
		}
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := ft.WithRecoverCall(func() {
				Invariant.Ok(1 == 2, "math is a construct")
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.True(t, ers.IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := ft.WithRecoverCall(func() { Invariant.IsTrue(false, err) })

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := ft.WithRecoverCall(func() { Invariant.Ok(false, err, 42) })
			if !errors.Is(se, err) {
				t.Log("se", se)
				t.Log("err", err)
				t.FailNow()
			}
			if !strings.Contains(se.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("NoError", func(t *testing.T) {
			err := ft.WithRecoverCall(func() { Invariant.IsTrue(false, 42) })
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("WithoutArgs", func(t *testing.T) {
			err := ft.WithRecoverCall(func() { Invariant.IsTrue(1 == 2) })
			if !errors.Is(err, ers.ErrInvariantViolation) {
				t.Fatal(err)
			}
			if !errors.Is(err, ers.ErrRecoveredPanic) {
				t.Fatal(err)
			}
		})
		t.Run("CheckError", func(t *testing.T) {
			if ers.IsInvariantViolation(nil) {
				t.Error("nil error shouldn't read as invariant")
			}
			if ers.IsInvariantViolation(errors.New("foo")) {
				t.Error("arbitrary errors are not invariants")
			}
		})
		t.Run("LongInvariant", func(t *testing.T) {
			err := ft.WithRecoverCall(func() {
				Invariant.IsTrue(1 == 2,
					"math is a construct",
					"1 == 2",
				)
			})
			if err == nil {
				t.FailNow()
			}
			if !strings.Contains(err.Error(), "construct 1 == 2") {
				t.Error(err)
			}
		})
	})
	t.Run("InvariantMust", func(t *testing.T) {
		t.Run("Nil", func(t *testing.T) {
			err := ft.WithRecoverCall(func() { Invariant.Must(nil, "hello") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("Expected", func(t *testing.T) {
			root := errors.New("kip")
			err := ft.WithRecoverCall(func() { Invariant.Must(root, "hello") })
			if err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("Panic", func(t *testing.T) {
			root := errors.New("kip")
			err := ft.WithRecoverCall(func() { Invariant.Must(root) })
			if err == nil {
				t.Fatal("expected error")
			}
			assert.ErrorIs(t, err, root)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
		})
		t.Run("Propagate", func(t *testing.T) {
			root := errors.New("kip")
			err := ft.WithRecoverCall(func() {
				Invariant.Must(root, "annotate")
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ers.ErrInvariantViolation) {
				t.Error(err)
			}
			if !errors.Is(err, root) {
				t.Error(err)
			}
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			if !strings.Contains(err.Error(), "annotate") {
				t.Log("-->", err.Error())
				t.Error(err)
			}
		})
		t.Run("Nil", func(t *testing.T) {
			err := ft.WithRecoverCall(func() {
				Invariant.Must(nil, "annotate")
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("MustBeOk", func(t *testing.T) {
		assert.NotPanic(t, func() {
			foo := ft.MustBeOk(func() (string, bool) { return "foo", true }())
			assert.Equal(t, "foo", foo)
		})
		assert.Panic(t, func() {
			foo := ft.MustBeOk(func() (string, bool) { return "foo", false }())
			assert.Equal(t, "foo", foo)
		})
	})
	t.Run("Sugar", func(t *testing.T) {
		t.Run("IsFalse", func(t *testing.T) {
			assert.Panic(t, func() { Invariant.IsFalse(true, "can't be false") })
			assert.Panic(t, func() { Invariant.IsFalse(true, "can't be false") })
			assert.Panic(t, func() { Invariant.IsFalse(true, "can't be false") })
			assert.Panic(t, func() { Invariant.IsFalse(ft.Ptr(3) != ft.Ptr(5), "can't be false") })
			assert.Panic(t, func() { Invariant.IsFalse(&time.Time{} != ft.Ptr(time.Now()), "can't be false") })
			assert.NotPanic(t, func() { Invariant.IsFalse(true == false, "can't be false") })
			assert.NotPanic(t, func() { Invariant.IsFalse(false, "can't be false") })
			assert.NotPanic(t, func() { Invariant.IsFalse(!true, "can't be false") })
		})
	})
	t.Run("Handler", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var of fn.Handler[string]
		t.Run("Worker", func(t *testing.T) {
			var called bool
			of = func(string) {
				called = true
				panic(io.EOF)
			}

			assert.NotPanic(t, func() {
				err := of.Safe()("hi")

				assert.ErrorIs(t, err, io.EOF)
				assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			})
			assert.True(t, called)
		})
		t.Run("Handler", func(t *testing.T) {
			var called bool
			var seen string
			of = func(in string) {
				called = true
				seen = in
			}

			err := FromHandler(of).Read(ctx, "hello")
			if err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Error("not called")
			}
			if seen != "hello" {
				t.Errorf("unexpected value%q", seen)

			}
		})
		t.Run("Wait", func(t *testing.T) {
			var called bool
			of = func(string) {
				called = true
				panic("hi")
			}

			assert.NotPanic(t, func() { ts := of.Safe(); assert.Error(t, ts("hi")) })
			assert.True(t, called)
		})
		t.Run("Failure", func(t *testing.T) {
			assert.Panic(t, func() { Invariant.Failure() })
			assert.Panic(t, func() { Invariant.Failure("this") })
			assert.Panic(t, func() { Invariant.Failure(true) })
			assert.Panic(t, func() { Invariant.Failure(false) })
			assert.Panic(t, func() { Invariant.Failure(nil) })
		})
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		ec := &erc.Collector{}
		extractErrors(ec, []any{nil, ers.Error("hi"), 1, true})
		check.Equal(t, ec.Len(), 2)

		var nerr error
		ec = &erc.Collector{}
		extractErrors(ec, []any{nil, ers.Error("hi"), func() error { return nil }(), nerr, 2, false})
		check.Equal(t, ec.Len(), 2)
	})

}
