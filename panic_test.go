package fun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/testt"
)

func TestPanics(t *testing.T) {
	t.Run("MustNoPanic", func(t *testing.T) {
		ok := Must(func() (bool, error) {
			return true, nil
		}())
		assert.True(t, ok)
	})
	t.Run("SafeWithPanic", func(t *testing.T) {
		ok, err := Safe(func() bool {
			return Must(func() (bool, error) {
				return true, errors.New("error")
			}())
		})
		assert.Error(t, err)
		check.True(t, !ok)
	})
	t.Run("SafeNoPanic", func(t *testing.T) {
		ok, err := Safe(func() bool {
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
	t.Run("Check", func(t *testing.T) {
		t.Run("NoError", func(t *testing.T) {
			err := Check(func() { t.Log("function runs") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("WithPanic", func(t *testing.T) {
			err := Check(func() { panic("function runs") })
			if err == nil {
				t.Fatal(err)
			}
			if err.Error() != "panic: function runs: recovered panic" {
				t.Error(err)
			}
		})
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := Check(func() {
				Invariant(1 == 2, "math is a construct")
			})
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrInvariantViolation)
			assert.ErrorIs(t, err, ErrRecoveredPanic)
			assert.True(t, IsInvariantViolation(err))
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := Check(func() { Invariant(false, err) })

			assert.ErrorIs(t, se, err)
		})
		t.Run("ErrorPlus", func(t *testing.T) {
			err := errors.New("kip")
			se := Check(func() { Invariant(false, err, 42) })
			if !errors.Is(se, err) {
				t.Fatal(err, se)
			}
			if !strings.Contains(se.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("NoError", func(t *testing.T) {
			err := Check(func() { Invariant(false, 42) })
			if !strings.Contains(err.Error(), "42") {
				t.Error(err)
			}
		})
		t.Run("WithoutArgs", func(t *testing.T) {
			err := Check(func() { Invariant(1 == 2) })
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
			err := Check(func() {
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
	t.Run("MergedError", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			e := &errorTest{}
			err := &internal.MergedError{}
			if errors.As(err, &e) {
				t.Fatal("should not validate")
			}
		})
		t.Run("Current", func(t *testing.T) {
			e := &errorTest{}
			err := &internal.MergedError{
				Current: &errorTest{val: 100},
			}
			if !errors.As(err, &e) {
				t.Fatal("should not validate")
			}
			if e.val != 100 {
				t.Fatal(e)
			}
		})
		t.Run("Wrapped", func(t *testing.T) {
			e := &errorTest{}
			err := &internal.MergedError{
				Wrapped: &errorTest{val: 100},
			}
			if !errors.As(err, &e) {
				t.Fatal("should not validate")
			}
			if e.val != 100 {
				t.Fatal(e)
			}
		})
		t.Run("WrappedAndCurrent", func(t *testing.T) {
			e := &errorTest{}
			err := &internal.MergedError{
				Wrapped: &errorTest{val: 1000},
				Current: &errorTest{val: 9000},
			}
			if !errors.As(err, &e) {
				t.Fatal("should not validate")
			}
			if e.val != 9000 {
				t.Fatal(e)
			}
		})
	})
	t.Run("InvariantMust", func(t *testing.T) {
		t.Run("Nil", func(t *testing.T) {
			err := Check(func() { InvariantMust(nil, "hello") })
			if err != nil {
				t.Fatal(err)
			}
		})
		t.Run("Expected", func(t *testing.T) {
			root := errors.New("kip")
			err := Check(func() { InvariantMust(root, "hello") })
			if err == nil {
				t.Fatal("expected error")
			}
		})
	})
	t.Run("InvariantCheck", func(t *testing.T) {
		t.Run("Propogate", func(t *testing.T) {
			root := errors.New("kip")
			err := Check(func() {
				InvariantCheck(func() error { return root }, "annotate")
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
			err := Check(func() {
				InvariantCheck(func() error { return nil }, "annotate")
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	})
	t.Run("Ignore", func(t *testing.T) {
		t.Run("SwallowError", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return errors.New("exists")
			}, "hello world")
			assert.True(t, called.Load())
		})
		t.Run("IndifferentError", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return nil
			}, "hello world")
			assert.True(t, called.Load())
		})
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			Ignore(func(in string) error {
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
		})

	})
	t.Run("IgnoreMust", func(t *testing.T) {
		t.Run("SwallowError", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return 100, errors.New("exists")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
		t.Run("IndifferentError", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return 100, nil
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
		t.Run("IndifferentValue", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (*int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				return nil, errors.New("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Zero(t, output)
		})
		t.Run("Panic", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (int, error) {
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Zero(t, output)
		})
		t.Run("PanicDefault", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (val int, _ error) {
				defer func() { val = 100 }()
				called.Store(true)
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			// defers that don't recover aren't run
			assert.Zero(t, output)
		})
		t.Run("DefersRun", func(t *testing.T) {
			called := &atomic.Bool{}
			output := IgnoreMust(func(in string) (val int, _ error) {
				defer func() { val = 100 }()
				defer func() { called.Store(true) }()
				defer func() { _ = recover() }()
				check.Equal(t, "hello world", in)
				panic("hello")
			}, "hello world")
			assert.True(t, called.Load())
			assert.Equal(t, 100, output)
		})
	})
	t.Run("Protect", func(t *testing.T) {
		assert.NotPanic(t, func() {
			value, err := Protect(func(int) (out string, err error) {
				panic(context.Canceled)
			})(1)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.ErrorIs(t, err, ErrRecoveredPanic)
			assert.Zero(t, value)
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

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }
