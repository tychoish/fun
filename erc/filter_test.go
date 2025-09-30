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
		var erf Filter

		err := errors.Join(ers.Error("beep"), context.Canceled)
		check.Error(t, err)
		check.NotError(t, erf.Without(context.Canceled).Apply(err))
		check.NotError(t, erf.Without(context.Canceled, io.EOF).Apply(err))
		check.Error(t, erf.Without(io.EOF).Apply(err))
		check.Error(t, erf.Without(nil).Apply(io.EOF))
		check.Error(t, erf.Without().Apply(err))
		check.Error(t, erf.Without().Apply(io.EOF))
		check.NotError(t, erf.Without(io.EOF).Apply(nil))
		check.NotError(t, erf.Without(nil).Apply(nil))
		check.NotError(t, erf.Without().Apply(nil))
		check.NotError(t, NewFilter().Apply(nil))
		check.Error(t, NewFilter().Apply(io.EOF))
		check.Error(t, NewFilter().Apply(context.Canceled))
		err = errors.Join(ers.Error("beep"), io.EOF)
		check.Error(t, err)
		check.Error(t, erf.Without(context.Canceled).Apply(err))
		check.NotError(t, erf.Without(io.EOF).Apply(err))
		check.NotError(t, erf.Without(io.EOF, context.DeadlineExceeded).Apply(err))
	})
	t.Run("Chain", func(t *testing.T) {
		var erf Filter
		endAndCancelFilter := erf.WithoutContext().WithoutTerminating().Then(nil)
		check.NotError(t, endAndCancelFilter(io.EOF))
		check.NotError(t, endAndCancelFilter(nil))
		check.NotError(t, endAndCancelFilter(context.Canceled))
		check.Error(t, endAndCancelFilter(ers.New("will error")))
	})
	t.Run("WithEmptyList", func(t *testing.T) {
		var count int
		filter := Filter(func(err error) error { count++; return err })
		filter = filter.Without().Only().Then(filter)
		check.Error(t, filter(io.EOF))
		check.Equal(t, count, 2)

		filter = filter.Only(io.EOF).Then(filter)
		check.Error(t, filter(io.EOF))
		check.Equal(t, count, 6)
		check.NotError(t, filter(ers.New("foo")))
		check.Equal(t, count, 8)
	})
	t.Run("Next", func(t *testing.T) {
		var erf Filter
		t.Run("Direct/Error", func(t *testing.T) { check.Panic(t, func() { _ = erf(io.EOF) }) })
		t.Run("Direct/Nil", func(t *testing.T) { check.Panic(t, func() { _ = erf(nil) }) })
		t.Run("NillNextWithError", func(t *testing.T) { check.Panic(t, func() { _ = NewFilter().Next(erf)(io.EOF) }) })
		t.Run("NillNextWithNilError", func(t *testing.T) { check.Panic(t, func() { _ = NewFilter().Next(erf)(nil) }) })
		t.Run("Passthrough", func(t *testing.T) { check.NotPanic(t, func() { check.NotError(t, NewFilter()(nil)) }) })

		t.Run("WithApply", func(t *testing.T) {
			check.Panic(t, func() { check.ErrorIs(t, NewFilter().Next(erf).Apply(io.EOF), io.EOF) })
		})
	})
}
