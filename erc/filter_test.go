package erc

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestFilter(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		err := errors.Join(ers.Error("beep"), context.Canceled)
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
		err = errors.Join(ers.Error("beep"), io.EOF)
		check.Error(t, err)
		check.Error(t, FilterExclude(context.Canceled).Run(err))
		check.NotError(t, FilterExclude(io.EOF).Run(err))
		check.NotError(t, FilterExclude(io.EOF, context.DeadlineExceeded).Run(err))

		err = ers.Error("boop")
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
		check.Error(t, endAndCancelFilter(ers.New("will error")))
		// TODO write test that exercises short circuiting execution
	})

}
