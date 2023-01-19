package fun

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type wrapTestType struct {
	value int
}

func TestWrap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Wrap", func(t *testing.T) {
		l := &wrapTestType{value: 42}
		nl := Unwrap(l)
		if nl != nil {
			t.Fatal("should be nil")
		}
	})
	t.Run("Is", func(t *testing.T) {
		if Is[*testing.T](5) {
			t.Error("Is should return false when types do not match ")
		}
		if !Is[int](100) {
			t.Error("Is should return true when types match")
		}
	})
	t.Run("Errors", func(t *testing.T) {
		err := errors.New("root")
		wrapped := fmt.Errorf("wrap: %w", err)
		unwrapped := Unwrap(wrapped)
		if unwrapped != err {
			t.Fatal("unexpected unrwapping")
		}
	})
	t.Run("Panics", func(t *testing.T) {
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
	})
}
