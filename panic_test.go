package fun

import (
	"context"
	"errors"
	"strings"
	"testing"
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
}
