package erc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

func withPanicAsError(op func()) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = erc.Join(p.(error), ers.ErrRecoveredPanic)
		}
	}()
	op()
	return
}

func TestPanics(t *testing.T) {
	t.Run("MustNoPanic", func(t *testing.T) {
		ok := erc.Must(func() (bool, error) {
			return true, nil
		}())
		assert.True(t, ok)
	})
	t.Run("SafeNoPanic", func(t *testing.T) {
		ec := erc.Collector{}
		out := false
		ec.WithRecover(func() {
			out = erc.Must(func() (bool, error) {
				return true, nil
			}())
		})
		assert.NotError(t, ec.Resolve())
		assert.True(t, out)
	})
	t.Run("InvariantOk", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := withPanicAsError(func() {
				erc.InvariantOk(1 == 2, "math is a construct")
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
			assert.True(t, ers.IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := withPanicAsError(func() { erc.InvariantOk(false, err) })

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := withPanicAsError(func() { erc.InvariantOk(false, err, 42) })
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
			err := withPanicAsError(func() { erc.InvariantOk(false, 42) })
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("WithoutArgs", func(t *testing.T) {
			err := withPanicAsError(func() { erc.InvariantOk(1 == 2) })
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
			err := withPanicAsError(func() {
				erc.InvariantOk(1 == 2,
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
	t.Run("Invariant", func(t *testing.T) {
		t.Run("Nil", func(t *testing.T) {
			err := withPanicAsError(func() { erc.Invariant(nil, "hello") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("Expected", func(t *testing.T) {
			root := errors.New("kip")
			err := withPanicAsError(func() { erc.Invariant(root, "hello") })
			if err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("Panic", func(t *testing.T) {
			root := errors.New("kip")
			err := withPanicAsError(func() { erc.Invariant(root) })
			if err == nil {
				t.Fatal("expected error")
			}
			assert.ErrorIs(t, err, root)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
		})
		t.Run("Propagate", func(t *testing.T) {
			root := errors.New("kip")
			err := withPanicAsError(func() {
				erc.Invariant(root, "annotate")
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
		t.Run("NilAgain", func(t *testing.T) {
			err := withPanicAsError(func() {
				erc.Invariant(nil, "annotate")
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("MustBeOk", func(t *testing.T) {
		assert.NotPanic(t, func() {
			foo := erc.MustOk(func() (string, bool) { return "foo", true }())
			assert.Equal(t, "foo", foo)
		})
		assert.Panic(t, func() {
			foo := erc.MustOk(func() (string, bool) { return "foo", false }())
			assert.Equal(t, "foo", foo)
		})
	})
	t.Run("Handler", func(t *testing.T) {
		t.Run("Handler", func(t *testing.T) {
			var called bool
			var seen string
			of := func(in string) {
				called = true
				seen = in
			}

			of("hello")
			if !called {
				t.Error("not called")
			}
			if seen != "hello" {
				t.Errorf("unexpected value%q", seen)
			}
		})
		t.Run("Panic", func(t *testing.T) {
			var called bool
			of := func(string) {
				called = true
				panic("hi")
			}

			assert.Panic(t, func() { of("hi") })
			assert.True(t, called)
		})
	})
	t.Run("Must", func(t *testing.T) {
		t.Run("NilError", func(t *testing.T) {
			assert.NotPanic(t, func() {
				result := erc.Must(42, nil)
				assert.Equal(t, 42, result)
			})
		})
		t.Run("NonNilError", func(t *testing.T) {
			testErr := errors.New("test error")
			err := withPanicAsError(func() {
				erc.Must(42, testErr)
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, testErr)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("StringReturn", func(t *testing.T) {
			assert.NotPanic(t, func() {
				result := erc.Must("hello", nil)
				assert.Equal(t, "hello", result)
			})
		})
		t.Run("ComplexType", func(t *testing.T) {
			type testStruct struct {
				Value int
			}
			assert.NotPanic(t, func() {
				result := erc.Must(testStruct{Value: 99}, nil)
				assert.Equal(t, 99, result.Value)
			})
		})
	})
	t.Run("MustOk", func(t *testing.T) {
		t.Run("TrueCondition", func(t *testing.T) {
			assert.NotPanic(t, func() {
				result := erc.MustOk("success", true)
				assert.Equal(t, "success", result)
			})
		})
		t.Run("FalseCondition", func(t *testing.T) {
			err := withPanicAsError(func() {
				erc.MustOk("fail", false)
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
		t.Run("IntegerReturn", func(t *testing.T) {
			assert.NotPanic(t, func() {
				result := erc.MustOk(42, true)
				assert.Equal(t, 42, result)
			})
		})
		t.Run("WithFunction", func(t *testing.T) {
			assert.NotPanic(t, func() {
				result := erc.MustOk(func() (string, bool) { return "hello world", true }())
				assert.Equal(t, "hello world", result)
			})
		})
		t.Run("WithFunctionFail", func(t *testing.T) {
			err := withPanicAsError(func() {
				erc.MustOk(func() (string, bool) { return "hello world", false }())
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvariantViolation)
			assert.ErrorIs(t, err, ers.ErrRecoveredPanic)
		})
	})
}
