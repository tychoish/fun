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
			if err.Error() != "function runs [recovered panic] <2>" {
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
			if err.Error() != "EOF [recovered panic] <2>" {
				t.Error(err)
			}
		})
	})
	t.Run("SafeOK", func(t *testing.T) {
		t.Run("Not", func(t *testing.T) {
			num, ok := SafeOK(func() (int, error) { return 42, io.EOF })
			assert.True(t, !ok)
			assert.Zero(t, num)
		})
		t.Run("Passes", func(t *testing.T) {
			num, ok := SafeOK(func() (int, error) { return 42, nil })
			assert.True(t, ok)
			assert.Equal(t, 42, num)
		})
	})
}
