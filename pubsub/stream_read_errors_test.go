package pubsub

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
)

func TestStreamReadDoesNotRepeatTerminalError(t *testing.T) {
	boom := errors.New("boom")

	t.Run("Plain", func(t *testing.T) {
		st := MakeStream(func(context.Context) (int, error) { return 0, boom })
		_, err := st.Read(t.Context())
		check.ErrorIs(t, err, boom)
		check.Equal(t, len(ers.Unwind(err)), 1)
		check.Equal(t, len(ers.Unwind(st.Close())), 1)
	})
	t.Run("HookErrorsStillSurface", func(t *testing.T) {
		hook := errors.New("hook failed")
		st := MakeStream(func(context.Context) (int, error) { return 0, boom }).
			WithHook(func(s *Stream[int]) { s.AddError(hook) })
		_, err := st.Read(t.Context())
		check.ErrorIs(t, err, boom)
		check.ErrorIs(t, err, hook)
		check.Equal(t, len(ers.Unwind(err)), 2)
	})
	t.Run("TerminatingErrorsAreJoinedWithCollected", func(t *testing.T) {
		hook := errors.New("hook failed")
		st := MakeStream(func(context.Context) (int, error) { return 0, io.EOF }).
			WithHook(func(s *Stream[int]) { s.AddError(hook) })
		_, err := st.Read(t.Context())
		check.ErrorIs(t, err, io.EOF)
		check.ErrorIs(t, err, hook)
	})
}

func TestConvertStreamDoesNotRepeatSourceError(t *testing.T) {
	boom := errors.New("boom")
	ident := Convert(fnx.MakeConverterErr(func(i int) (int, error) { return i, nil }))

	t.Run("SourceFailure", func(t *testing.T) {
		out := ident.Stream(MakeStream(func(context.Context) (int, error) { return 0, boom }))
		_, err := out.Read(t.Context())
		check.ErrorIs(t, err, boom)
		check.Equal(t, len(ers.Unwind(err)), 1)
		check.Equal(t, len(ers.Unwind(out.Close())), 1)
	})
	t.Run("SourceCloseErrorAfterExhaustion", func(t *testing.T) {
		hook := errors.New("hook failed")
		n := 0
		src := MakeStream(func(context.Context) (int, error) {
			if n++; n > 2 {
				return 0, io.EOF
			}
			return n, nil
		}).WithHook(func(s *Stream[int]) { s.AddError(hook) })
		out := ident.Stream(src)
		for range 2 {
			_, err := out.Read(t.Context())
			check.NotError(t, err)
		}
		_, err := out.Read(t.Context())
		check.ErrorIs(t, err, io.EOF)
		check.ErrorIs(t, out.Close(), hook)
	})
	t.Run("ConvertFailureAddsSourceCloseErrors", func(t *testing.T) {
		hook := errors.New("hook failed")
		src := MakeStream(func(context.Context) (int, error) { return 1, nil }).
			WithHook(func(s *Stream[int]) { s.AddError(hook) })
		out := Convert(fnx.MakeConverterErr(func(int) (int, error) { return 0, boom })).Stream(src)
		_, err := out.Read(t.Context())
		check.ErrorIs(t, err, boom)
		check.ErrorIs(t, out.Close(), hook)
	})
}
