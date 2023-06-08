package ers

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestPanics(t *testing.T) {
	t.Run("SafeWithPanic", func(t *testing.T) {
		ok, err := Safe(func() bool {
			panic(errors.New("error"))
		})
		assert.Error(t, err)
		check.True(t, !ok)
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
			if err.Error() != "function runs: recovered panic" {
				t.Error(err)
			}
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
	t.Run("ParsePanic", func(t *testing.T) {
		t.Run("NilInput", func(t *testing.T) {
			err := ParsePanic(nil)
			if err != nil {
				t.Fatal(err)
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
				t.Error("is EOF", err)
			}
			if !errors.Is(err, ErrRecoveredPanic) {
				t.Error("not wrapped", err)
			}
			if err.Error() != "EOF: recovered panic" {
				t.Error(err)
			}
		})
	})
}
