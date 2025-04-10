package ers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestErrors(t *testing.T) {
	t.Run("Predicates", func(t *testing.T) {
		check.True(t, IsTerminating(io.EOF))
		check.True(t, !IsTerminating(Error("hello")))
		check.True(t, IsTerminating(Join(Error("beep"), io.EOF)))

		check.True(t, !IsExpiredContext(io.EOF))
		check.True(t, !IsExpiredContext(Join(Error("beep"), io.EOF)))

		check.True(t, IsExpiredContext(context.Canceled))
		check.True(t, IsExpiredContext(context.DeadlineExceeded))
		check.True(t, IsExpiredContext(Join(Error("beep"), context.DeadlineExceeded)))
		check.True(t, IsExpiredContext(Join(Error("beep"), context.Canceled)))
	})
	t.Run("Ok", func(t *testing.T) {
		var err error
		check.True(t, Ok(err))
		err = errors.New("hi")
		check.True(t, !Ok(err))
	})
	t.Run("Filter", func(t *testing.T) {
		err := Join(Error("beep"), context.Canceled)
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
		err = Join(Error("beep"), io.EOF)
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
	t.Run("FilterToRoot", func(t *testing.T) {
		filter := FilterToRoot()
		t.Run("Nil", func(t *testing.T) {
			assert.Zero(t, filter(nil))
		})
		t.Run("WrappingDirection", func(t *testing.T) {
			t.Run("Vector", func(t *testing.T) {
				root := filter(errors.Join(Error("hello"), Error("this"), io.EOF))
				assert.Equal(t, io.EOF, root)
			})
			t.Run("Stack", func(t *testing.T) {
				root := filter(Join(io.EOF, Error("this"), Error("hello")))
				assert.Equal(t, io.EOF, root)
			})
		})
		t.Run("EmptyWrap", func(t *testing.T) {
			list := Join()
			root := filter(list)
			assert.Equal(t, list, root)

		})
		t.Run("Stdlib", func(t *testing.T) {
			err := fmt.Errorf("hello: %w", io.EOF)
			root := filter(err)
			assert.Equal(t, io.EOF, root)
		})
	})
	t.Run("InvariantViolation", func(t *testing.T) {
		assert.True(t, IsInvariantViolation(ErrInvariantViolation))
		assert.True(t, IsInvariantViolation(Join(io.EOF, Error("hello"), ErrInvariantViolation)))
		assert.True(t, !IsInvariantViolation(nil))
		assert.True(t, !IsInvariantViolation(9001))
		assert.True(t, !IsInvariantViolation(io.EOF))
	})
	t.Run("Wrapped", func(t *testing.T) {
		err := errors.New("base")
		for i := 0; i < 100; i++ {
			err = fmt.Errorf("wrap %d: %w", i, err)
		}

		errs := Unwind(err)
		if len(errs) != 101 {
			t.Error(len(errs))
		}
		if errs[100].Error() != "base" {
			t.Error(errs[100])
		}
		check.Equal(t, 101, len(Unwind(err)))
	})
	t.Run("ExtractErrors", func(t *testing.T) {
		args, errs := ExtractErrors([]any{nil, Error("hi"), 1, true})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
		var nerr error
		args, errs = ExtractErrors([]any{nil, Error("hi"), func() error { return nil }(), nerr, 2, false})
		check.Equal(t, len(errs), 1)
		check.Equal(t, len(args), 2)
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
	t.Run("RemoveOK", func(t *testing.T) {
		check.Equal(t, 0, len(RemoveOk([]error{nil, nil, nil})))
		check.Equal(t, 3, cap(RemoveOk([]error{nil, nil, nil})))
		check.Equal(t, 1, len(RemoveOk([]error{nil, io.EOF, nil})))
		check.Equal(t, 3, len(RemoveOk([]error{Error("one"), io.EOF, New("two")})))
	})
	t.Run("Ignore", func(t *testing.T) {
		check.NotPanic(t, func() { Ignore(Error("new")) })
		check.NotPanic(t, func() { Ignore(nil) })
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
