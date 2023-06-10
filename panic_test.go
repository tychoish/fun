package fun

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func TestPanics(t *testing.T) {
	t.Run("MustNoPanic", func(t *testing.T) {
		ok := Must(func() (bool, error) {
			return true, nil
		}())
		assert.True(t, ok)
	})
	t.Run("SafeNoPanic", func(t *testing.T) {
		ok, err := ers.Safe(func() bool {
			return Must(func() (bool, error) {
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
			err := ers.Check(func() {
				Invariant(1 == 2, "math is a construct")
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvariantViolation)
			assert.ErrorIs(t, err, ErrRecoveredPanic)
			assert.True(t, IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := ers.Check(func() { Invariant(false, err) })

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := ers.Check(func() { Invariant(false, err, 42) })
			if !errors.Is(se, err) {
				t.Fatal(err, se)
			}
			if !strings.Contains(se.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("NoError", func(t *testing.T) {
			err := ers.Check(func() { Invariant(false, 42) })
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("WithoutArgs", func(t *testing.T) {
			err := ers.Check(func() { Invariant(1 == 2) })
			if !errors.Is(err, ErrInvariantViolation) {
				t.Fatal(err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Fatal(err)
			}
		})
		t.Run("CheckError", func(t *testing.T) {
			if IsInvariantViolation(nil) {
				t.Error("nil error shouldn't read as invariant")
			}
			if IsInvariantViolation(errors.New("foo")) {
				t.Error("arbitrary errors are not invariants")
			}
		})
		t.Run("LongInvariant", func(t *testing.T) {
			err := ers.Check(func() {
				Invariant(1 == 2,
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
			err := ers.Check(func() { InvariantMust(nil, "hello") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("Expected", func(t *testing.T) {
			root := errors.New("kip")
			err := ers.Check(func() { InvariantMust(root, "hello") })
			if err == nil {
				t.Fatal("expected error")
			}
		})
		t.Run("Panic", func(t *testing.T) {
			root := errors.New("kip")
			err := ers.Check(func() { InvariantMust(root) })
			if err == nil {
				t.Fatal("expected error")
			}
			assert.ErrorIs(t, err, root)
			assert.ErrorIs(t, err, ErrInvariantViolation)
		})
		t.Run("Propogate", func(t *testing.T) {
			root := errors.New("kip")
			err := ers.Check(func() {
				InvariantMust(root, "annotate")
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrInvariantViolation) {
				t.Error(err)
			}
			if !errors.Is(err, root) {
				t.Error(err)
			}
			assert.ErrorIs(t, err, ErrRecoveredPanic)
			if !strings.Contains(err.Error(), "annotate") {
				t.Error(err)
			}
		})
		t.Run("Nil", func(t *testing.T) {
			err := ers.Check(func() {
				InvariantMust(nil, "annotate")
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("MustBeOk", func(t *testing.T) {
		assert.NotPanic(t, func() {
			foo := MustBeOk(func() (string, bool) { return "foo", true }())
			assert.Equal(t, "foo", foo)
		})
		assert.Panic(t, func() {
			foo := MustBeOk(func() (string, bool) { return "foo", false }())
			assert.Equal(t, "foo", foo)
		})
	})
	t.Run("Observer", func(t *testing.T) {
		ctx := testt.Context(t)
		var of Observer[string]
		t.Run("Worker", func(t *testing.T) {
			var called bool
			of = func(string) {
				called = true
				panic(io.EOF)
			}

			assert.NotPanic(t, func() {
				err := of.Worker("hi")(ctx)
				assert.ErrorIs(t, err, io.EOF)
				assert.ErrorIs(t, err, ErrRecoveredPanic)
			})
			assert.True(t, called)
		})
		t.Run("Processor", func(t *testing.T) {
			var called bool
			var seen string
			of = func(in string) {
				called = true
				seen = in
			}

			err := of.Processor()(ctx, "hello")
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

			assert.Panic(t, func() { of.Wait("hi")(ctx) })
			assert.True(t, called)
		})
	})
}
