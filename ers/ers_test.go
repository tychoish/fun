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
		check.True(t, Ok(err))
		err = errors.New("hi")
		check.True(t, !Ok(err))
	})
	t.Run("Filter", func(t *testing.T) {
		err := errors.Join(Error("beep"), context.Canceled)
		check.Error(t, err)
		check.NotError(t, FilterExclude(context.Canceled).Run(err))
		check.NotError(t, FilterExclude(context.Canceled, io.EOF).Run(err))
		check.Error(t, FilterExclude(io.EOF).Run(err))
		check.Error(t, FilterExclude(nil).Run(io.EOF))
		check.Error(t, FilterExclude().Run(err))
		check.Error(t, FilterExclude().Run(io.EOF))
		check.NotError(t, FilterExclude(io.EOF).Run(nil))
		check.NotError(t, FilterExclude(nil).Run(nil))
		check.NotError(t, FilterExclude().Run(nil))
		check.NotError(t, FilterNoop().Run(nil))
		check.Error(t, FilterNoop().Run(io.EOF))
		check.Error(t, FilterNoop().Run(context.Canceled))
		err = errors.Join(Error("beep"), io.EOF)
		check.Error(t, err)
		check.Error(t, FilterExclude(context.Canceled).Run(err))
		check.NotError(t, FilterExclude(io.EOF).Run(err))
		check.NotError(t, FilterExclude(io.EOF, context.DeadlineExceeded).Run(err))

		err = Error("boop")
		err = FilterConvert(io.EOF).Run(err)
		check.Equal(t, err, io.EOF)
		err = FilterConvert(io.EOF).Run(nil)
		check.NotError(t, err)
	})
	t.Run("FilterJoin", func(t *testing.T) {
		endAndCancelFilter := FilterJoin(FilterContext(), FilterTerminating(), nil)
		check.NotError(t, endAndCancelFilter(io.EOF))
		check.NotError(t, endAndCancelFilter(nil))
		check.NotError(t, endAndCancelFilter(context.Canceled))
		check.Error(t, endAndCancelFilter(New("will error")))
		// TODO write test that exercises short circuiting execution
	})
	t.Run("InvariantViolation", func(t *testing.T) {
		assert.True(t, IsInvariantViolation(ErrInvariantViolation))
		assert.True(t, IsInvariantViolation(errors.Join(io.EOF, Error("hello"), ErrInvariantViolation)))
		assert.True(t, !IsInvariantViolation(nil))
		assert.True(t, !IsInvariantViolation(9001))
		assert.True(t, !IsInvariantViolation(io.EOF))
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
	t.Run("As", func(t *testing.T) {
		var err error = &errorTest{val: 100}

		out := &errorTest{}

		check.True(t, As(err, &out))
		check.Equal(t, out.val, 100)
	})
	t.Run("Append", func(t *testing.T) {
		check.Equal(t, 0, len(Append([]error{}, nil, nil, nil)))
		check.Equal(t, 1, len(Append([]error{}, nil, Error("one"), nil)))
		check.Equal(t, 2, len(Append([]error{Error("hi")}, nil, Error("one"), nil)))
	})
	t.Run("Strings", func(t *testing.T) {
		strs := Strings([]error{Error("hi"), nil, Error("from"), Error("the"), nil, Error("other"), nil, Error("world"), nil})
		check.Equal(t, len(strs), 5)
		check.Equal(t, "hi from the other world", strings.Join(strs, " "))
	})
	t.Run("RemoveOK", func(t *testing.T) {
		check.Equal(t, 0, len(RemoveOk([]error{nil, nil, nil})))
		check.Equal(t, 3, cap(RemoveOk([]error{nil, nil, nil})))
		check.Equal(t, 1, len(RemoveOk([]error{nil, io.EOF, nil})))
		check.Equal(t, 3, len(RemoveOk([]error{Error("one"), io.EOF, New("two")})))
	})
	t.Run("OkCheck", func(t *testing.T) {
		var err *okErrTest
		_ = error(err) // compile time interface compliance test

		assert.True(t, Ok(err))
		assert.True(t, err == nil)
		err = &okErrTest{}

		assert.True(t, Ok(err))
		assert.True(t, err != nil)
		err.val = 41
		assert.True(t, !Ok(err))
	})
	t.Run("When", func(t *testing.T) {
		t.Run("Bypass", func(t *testing.T) {
			check.NotError(t, When(false, "hello"))
			check.True(t, When(false, "hello") == nil)
		})
		t.Run("True", func(t *testing.T) {
			check.Error(t, When(true, "hello"))
			check.ErrorIs(t, When(true, "hello"), Error("hello"))

			_, ok := When(true, "hello").(Error)
			check.True(t, ok)
		})

		const errval Error = "ERRO=42"

		t.Run("BasicString", func(t *testing.T) {
			err := When(false, errval)
			assert.NotError(t, err)
			err = When(true, errval)
			check.Error(t, err)
		})
		t.Run("WhenBasicWeirdType", func(t *testing.T) {
			err := When(false, 54)
			assert.NotError(t, err)
			err = When(true, 50000)
			check.Error(t, err)

			check.Substring(t, err.Error(), "50000")
			check.Substring(t, err.Error(), "int")
		})

		t.Run("Wrapping", func(t *testing.T) {
			err := Whenf(false, "no error %w", errval)
			assert.True(t, Ok(err))

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
}
