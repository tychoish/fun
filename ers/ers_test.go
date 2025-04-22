package ers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

type errorTest struct {
	val int
}

func (e *errorTest) Error() string { return fmt.Sprint("error: ", e.val) }

type okErrTest struct {
	val uint
}

func (e *okErrTest) Error() string { return fmt.Sprint("errno", e.val) }
func (e *okErrTest) Ok() bool      { return e == nil || e.val <= 0 }

// Strings renders (using the Error() method) a slice of errors into a
// slice of their string values.
func Strings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for idx := range errs {
		if !IsOk(errs[idx]) {
			out = append(out, errs[idx].Error())
		}
	}

	return out
}

func TestErrors(t *testing.T) {
	t.Run("Predicates", func(t *testing.T) {
		check.True(t, IsTerminating(io.EOF))
		check.True(t, !IsTerminating(Error("hello")))
		check.True(t, IsTerminating(errors.Join(Error("beep"), io.EOF)))

		check.True(t, !IsExpiredContext(io.EOF))
		check.True(t, !IsExpiredContext(errors.Join(Error("beep"), io.EOF)))

		check.True(t, IsExpiredContext(context.Canceled))
		check.True(t, IsExpiredContext(context.DeadlineExceeded))
		check.True(t, IsExpiredContext(errors.Join(Error("beep"), context.DeadlineExceeded)))
		check.True(t, IsExpiredContext(errors.Join(Error("beep"), context.Canceled)))
	})
	t.Run("Ok", func(t *testing.T) {
		var err error
		check.True(t, IsOk(err))
		err = errors.New("hi")
		check.True(t, !IsOk(err))
	})
	t.Run("IsError", func(t *testing.T) {
		check.True(t, !IsError(nil))
		check.True(t, IsError(New("error")))
	})
	t.Run("IsNil", func(t *testing.T) {
		check.True(t, !Is(nil, io.EOF))
		check.True(t, Is(io.EOF, io.EOF))
	})
	t.Run("Unwrap", func(t *testing.T) {
		werr := fmt.Errorf("hi: %w", io.EOF)
		check.True(t, !IsError(Unwrap(New("hello"))))
		check.True(t, IsError(Unwrap(werr)))
	})
	t.Run("Unwind", func(t *testing.T) {
		werr := fmt.Errorf("hi: %w", io.EOF)
		check.Equal(t, 1, len(Unwind(New("hello"))))
		check.Equal(t, 2, len(Unwind(werr)))
	})
	t.Run("As", func(t *testing.T) {
		var err error = &errorTest{val: 100}

		out := &errorTest{}

		check.True(t, As(err, &out))
		check.Equal(t, out.val, 100)
	})
	t.Run("Strings", func(t *testing.T) {
		strs := Strings([]error{Error("hi"), nil, Error("from"), Error("the"), nil, Error("other"), nil, Error("world"), nil})
		check.Equal(t, len(strs), 5)
		check.Equal(t, "hi from the other world", strings.Join(strs, " "))
	})
	t.Run("OkCheck", func(t *testing.T) {
		var err *okErrTest
		_ = error(err) // compile time interface compliance test

		assert.True(t, IsOk(err))
		assert.True(t, err == nil)
		err = &okErrTest{}

		assert.True(t, IsOk(err))
		assert.True(t, err != nil)
		err.val = 41
		assert.True(t, !IsOk(err))
	})
	t.Run("When", func(t *testing.T) {
		const errval Error = "ERRO=42"

		t.Run("BasicString", func(t *testing.T) {
			err := When(false, errval)
			assert.NotError(t, err)
			err = When(true, errval)
			check.Error(t, err)
		})
		t.Run("Wrapping", func(t *testing.T) {
			err := Whenf(false, "no error %w", errval)
			assert.True(t, IsOk(err))

			err = Whenf(true, "no error: %w", errval)
			assert.Error(t, err)
			assert.ErrorIs(t, err, errval)
			assert.True(t, err != errval)
		})
	})
	t.Run("Whenf", func(t *testing.T) {
		t.Run("Bypass", func(t *testing.T) {
			check.NotError(t, Whenf(false, "hello: %d", 1))
			check.True(t, Whenf(false, "hello %d", 1) == nil)
		})
		t.Run("True", func(t *testing.T) {
			inner := Error("inner error; error")
			check.Error(t, Whenf(true, "hello: %w", inner))
			check.ErrorIs(t, Whenf(true, "hello: %w", inner), inner)
			check.NotEqual(t, Whenf(true, "hello: %w", inner).Error(), inner.Error())
		})
	})
	t.Run("Strings", func(t *testing.T) {
		sl := []error{io.EOF, context.Canceled, ErrLimitExceeded}
		strs := Strings(sl)
		merged := strings.Join(strs, ": ")
		check.Substring(t, merged, "EOF")
		check.Substring(t, merged, "context canceled")
		check.Substring(t, merged, "limit exceeded")
	})
}
