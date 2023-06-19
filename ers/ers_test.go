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
		check.True(t, IsTerminating(context.Canceled))
		check.True(t, IsTerminating(context.DeadlineExceeded))
		check.True(t, !IsTerminating(Error("hello")))
		check.True(t, IsTerminating(Join(Error("beep"), io.EOF)))
		check.True(t, IsTerminating(Join(Error("beep"), context.Canceled)))
		check.True(t, IsTerminating(Join(Error("beep"), context.DeadlineExceeded)))

		check.True(t, !ContextExpired(io.EOF))
		check.True(t, !ContextExpired(Join(Error("beep"), io.EOF)))

		check.True(t, ContextExpired(context.Canceled))
		check.True(t, ContextExpired(context.DeadlineExceeded))
		check.True(t, ContextExpired(Join(Error("beep"), context.DeadlineExceeded)))
		check.True(t, ContextExpired(Join(Error("beep"), context.Canceled)))
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
		check.NotError(t, FilterRemove(context.Canceled)(err))
		check.NotError(t, FilterRemove(context.Canceled, io.EOF)(err))
		check.Error(t, FilterRemove(io.EOF)(err))
		check.Error(t, FilterRemove(nil)(io.EOF))
		check.NotError(t, FilterRemove(io.EOF)(nil))
		check.NotError(t, FilterRemove(nil)(nil))

		err = Join(Error("beep"), io.EOF)
		check.Error(t, err)
		check.Error(t, FilterRemove(context.Canceled)(err))
		check.NotError(t, FilterRemove(io.EOF)(err))
		check.NotError(t, FilterRemove(io.EOF, context.DeadlineExceeded)(err))

		err = Error("boop")
		err = FilterConvert(io.EOF)(err)
		check.Equal(t, err, io.EOF)
		err = FilterConvert(io.EOF)(nil)
		check.NotError(t, err)

	})
	t.Run("FilterToRoot", func(t *testing.T) {
		filter := FilterToRoot()
		t.Run("Nil", func(t *testing.T) {
			assert.Zero(t, filter(nil))
		})
		t.Run("SliceEdge", func(t *testing.T) {
			err := &mergederr{}
			assert.Equal(t, filter(err), error(err))
		})
		t.Run("Spliced", func(t *testing.T) {
			list := Join(Error("hello"), Error("this"), io.EOF)
			root := filter(list)
			assert.Equal(t, io.EOF, root)
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
		t.Run("Merged", func(t *testing.T) {
			err := Join(errors.New("hello"), io.EOF)
			root := filter(err)
			assert.Equal(t, io.EOF, root)
		})
	})
}
