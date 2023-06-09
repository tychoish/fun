package ers

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestErrors(t *testing.T) {
	t.Run("Predicates", func(t *testing.T) {
		check.True(t, IsTerminating(io.EOF))
		check.True(t, IsTerminating(context.Canceled))
		check.True(t, IsTerminating(context.DeadlineExceeded))
		check.True(t, !IsTerminating(Error("hello")))
		check.True(t, IsTerminating(Merge(Error("beep"), io.EOF)))
		check.True(t, IsTerminating(Merge(Error("beep"), context.Canceled)))
		check.True(t, IsTerminating(Merge(Error("beep"), context.DeadlineExceeded)))

		check.True(t, !ContextExpired(io.EOF))
		check.True(t, !ContextExpired(Merge(Error("beep"), io.EOF)))

		check.True(t, ContextExpired(context.Canceled))
		check.True(t, ContextExpired(context.DeadlineExceeded))
		check.True(t, ContextExpired(Merge(Error("beep"), context.DeadlineExceeded)))
		check.True(t, ContextExpired(Merge(Error("beep"), context.Canceled)))
	})
	t.Run("Ok", func(t *testing.T) {
		var err error
		check.True(t, Ok(err))
		err = errors.New("hi")
		check.True(t, !Ok(err))
	})
	t.Run("Filter", func(t *testing.T) {
		err := Merge(Error("beep"), context.Canceled)
		check.Error(t, err)
		check.NotError(t, Filter(err, context.Canceled))
		check.NotError(t, Filter(err, context.Canceled, io.EOF))
		check.Error(t, Filter(err, io.EOF))
		check.NotError(t, Filter(nil, io.EOF))
		check.NotError(t, Filter(nil, nil))

		err = Merge(Error("beep"), io.EOF)
		check.Error(t, err)
		check.Error(t, Filter(err, context.Canceled))
		check.NotError(t, Filter(err, io.EOF))
		check.NotError(t, Filter(err, io.EOF, context.DeadlineExceeded))

	})
	t.Run("Collector", func(t *testing.T) {
		ec := &Collector{}
		check.Zero(t, ec.Len())
		check.NotError(t, ec.Resolve())
		check.True(t, !ec.HasErrors())
		const ErrCountMeOut Error = "countm-me-out"
		op := func() error { return ErrCountMeOut }

		ec.Add(Merge(Error("beep"), context.Canceled))
		check.True(t, ec.HasErrors())
		check.Equal(t, 1, ec.Len())
		ec.Add(op())
		check.Equal(t, 2, ec.Len())
		ec.Add(Error("boop"))
		check.Equal(t, 3, ec.Len())

		ec.Add(nil)
		check.Equal(t, 3, ec.Len())
		check.True(t, ec.HasErrors())

		err := ec.Resolve()
		check.Error(t, err)
		check.Error(t, Filter(err, io.EOF, context.DeadlineExceeded))
		check.NotError(t, Filter(err, context.Canceled))
		check.NotError(t, Filter(err, ErrCountMeOut))
		check.ErrorIs(t, err, ErrCountMeOut)
	})
}
