package pubsub

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
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
