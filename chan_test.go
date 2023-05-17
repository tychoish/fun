package fun

import (
	"context"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
)

func TestChannel(t *testing.T) {
	t.Run("Send", func(t *testing.T) {
		t.Run("Write", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int, 2)
			err := Blocking(ch).Send().Write(ctx, 1)
			assert.NotError(t, err)
			assert.Equal(t, <-ch, 1)

			err = NonBlocking(ch).Send().Write(ctx, 3)
			assert.NotError(t, err)
			assert.Equal(t, <-ch, 3)
		})
		t.Run("ClosedWrite", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan int)
			close(ch)
			err := Blocking(ch).Send().Write(ctx, 43)
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("NonBlocking", func(t *testing.T) {
			t.Run("Wrote", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int)

				err := NonBlocking(ch).Send().Write(ctx, 3)
				assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)
				close(ch)
				out, ok := <-ch
				assert.True(t, !ok)
				assert.Zero(t, out)
			})
			t.Run("Check", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 1)

				sent := NonBlocking(ch).Send().Check(ctx, 3)
				assert.True(t, sent)
				out, ok := <-ch
				assert.True(t, ok)
				assert.Equal(t, 3, out)
			})
			t.Run("Check", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 1)

				NonBlocking(ch).Send().Ignore(ctx, 3)
				out, ok := <-ch
				assert.True(t, ok)
				assert.Equal(t, 3, out)
			})
			t.Run("Canceled", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch := make(chan int)

				err := Blocking(ch).Send().Write(ctx, 1)
				assert.Error(t, err)
				assert.ErrorIs(t, err, context.Canceled)
				err = NonBlocking(ch).Send().Write(ctx, 1)
				assert.Error(t, err)
				assert.ErrorIs(t, err, context.Canceled)

				assert.True(t, !NonBlocking(ch).Send().Check(ctx, 1))

				err = (Send[int]{ch: ch}).Write(ctx, 1)
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
				err = (Send[int]{mode: 42, ch: ch}).Write(ctx, 1)
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
			})

		})
	})
	t.Run("Receive", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		t.Run("Blocking", func(t *testing.T) {
			ch := make(chan string, 1)
			ch <- "hello world"
			val, err := Blocking(ch).Recieve().Read(ctx)
			assert.NotError(t, err)
			assert.Equal(t, val, "hello world")

			ch <- "kip"
			val, ok := Blocking(ch).Recieve().Check(ctx)
			assert.Equal(t, val, "kip")
			assert.True(t, ok)

			ch <- "merlin"
			assert.True(t, Blocking(ch).Recieve().Drop(ctx))
			// do this to verify if it the channel is now empty
			_, err = NonBlocking(ch).Recieve().Read(ctx)
			assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)

			ch <- "deleuze"
			val = Blocking(ch).Recieve().Force(ctx)
			assert.Equal(t, val, "deleuze")
			assert.True(t, ok)
		})
		t.Run("NonBlocking", func(t *testing.T) {
			ch := make(chan string, 1)
			assert.Zero(t, NonBlocking(ch).Recieve().Force(ctx))
			assert.True(t, !NonBlocking(ch).Recieve().Drop(ctx))
			val, err := NonBlocking(ch).Recieve().Read(ctx)
			assert.Zero(t, val)
			assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)
		})
		t.Run("Invalid", func(t *testing.T) {
			op := ChannelOp[string]{mode: 0, ch: make(chan string)}
			val, err := op.Recieve().Read(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
			assert.Zero(t, val)
		})
	})
}
