package fun

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tychoish/fun/internal"
)

func TestPanics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("MustNoPanic", func(t *testing.T) {
		ok := Must(func() (bool, error) {
			return true, nil
		}())
		if !ok {
			t.Error("should be true")
		}
	})
	t.Run("SafeWithPanic", func(t *testing.T) {
		ok, err := Safe(func() bool {
			return Must(func() (bool, error) {
				return true, errors.New("error")
			}())
		})
		if err == nil {
			t.Error("error should be non-nil")
		}
		if ok {
			t.Error("should be zero value of T")
		}
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
	t.Run("SafeCtxWithPanic", func(t *testing.T) {
		ok, err := SafeCtx(ctx, func(_ context.Context) bool {
			return Must(func() (bool, error) {
				return true, errors.New("error")
			}())
		})
		if err == nil {
			t.Error("error should be non-nil")
		}
		if ok {
			t.Error("should be zero value of T")
		}
	})
	t.Run("SafeCtxNoPanic", func(t *testing.T) {
		ok, err := SafeCtx(ctx, func(_ context.Context) bool {
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
			if err.Error() != "panic: function runs" {
				t.Error(err)
			}
		})
	})
	t.Run("Invariant", func(t *testing.T) {
		t.Run("End2End", func(t *testing.T) {
			err := Check(func() {
				Invariant(1 == 2, "math is a construct")
			})
			if err == nil {
				t.Error("expected error")
			}
			t.Log(err)
			if !strings.Contains(err.Error(), "math is a construct") {
				t.Error(err)
			}
			if !strings.HasPrefix(err.Error(), "panic:") {
				t.Error(err)
			}
			if !strings.Contains(err.Error(), "invariant violation") {
				t.Error(err)
			}
			if !errors.Is(err, ErrInvariantViolation) {
				t.Error("invariant violation wrapping")
			}
			if !IsInvariantViolation(err) {
				t.Error("invariant violation detection")
			}
		})
		t.Run("Error", func(t *testing.T) {
			err := errors.New("kip")
			se := Check(func() { Invariant(false, err) })
			if !errors.Is(se, err) {
				t.Fatal(err, se)
			}
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
			if errors.Unwrap(err) != ErrInvariantViolation {
				t.Error(err)
			}
			// this is the check function
			if !strings.HasPrefix(err.Error(), "panic:") {
				t.Error(err)
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
			if !strings.Contains(err.Error(), "panic:") {
				t.Error(err)
			}
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
}

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }
