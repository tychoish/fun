package fun

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
)

func TestChannel(t *testing.T) {
	t.Run("Chain", func(t *testing.T) {
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
			t.Run("Processor", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := Blocking(ch).Processor()(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = NonBlocking(ch).Processor()(ctx, 3)
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
			t.Run("Ignore", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				assert.Equal(t, len(ch), 0)
				assert.Equal(t, cap(ch), 2)
				err := Blocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, len(ch), 1)
				assert.Equal(t, <-ch, 1)
				assert.Equal(t, len(ch), 0)

				err = Blocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, len(ch), 1)
				Blocking(ch).Receive().Ignore(ctx)
				assert.Equal(t, len(ch), 0)

			})
			t.Run("Zero", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan time.Time, 1)
				assert.NotError(t, Blocking(ch).Send().Zero(ctx))
				val := <-ch
				assert.True(t, val.IsZero())
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
				t.Run("Chain", func(t *testing.T) {
					ch := make(chan string, 1)
					ch <- "hello world"
					val, err := Blocking(ch).Receive().Read(ctx)
					assert.NotError(t, err)
					assert.Equal(t, val, "hello world")

					ch <- "kip"
					val, ok := Blocking(ch).Receive().Check(ctx)
					assert.Equal(t, val, "kip")
					assert.True(t, ok)

					ch <- "merlin"
					assert.True(t, Blocking(ch).Receive().Drop(ctx))
					// do this to verify if it the channel is now empty
					_, err = NonBlocking(ch).Receive().Read(ctx)
					assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)

					ch <- "deleuze"
					val = Blocking(ch).Receive().Force(ctx)
					assert.Equal(t, val, "deleuze")
					assert.True(t, ok)
				})
				t.Run("Producer", func(t *testing.T) {
					ch := make(chan string, 1)
					ch <- "hello world"
					val, err := Blocking(ch).Producer()(ctx)
					assert.NotError(t, err)
					assert.Equal(t, val, "hello world")
				})
			})
			t.Run("NonBlocking", func(t *testing.T) {
				ch := make(chan string, 1)
				assert.Zero(t, NonBlocking(ch).Receive().Force(ctx))
				assert.True(t, !NonBlocking(ch).Receive().Drop(ctx))
				val, err := NonBlocking(ch).Receive().Read(ctx)
				assert.Zero(t, val)
				assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChannelOp[string]{mode: 0, ch: make(chan string)}
				val, err := op.Receive().Read(ctx)
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
				assert.Zero(t, val)
			})
		})
	})
	t.Run("Shortcut", func(t *testing.T) {
		t.Run("Send", func(t *testing.T) {
			t.Run("Write", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := BlockingSend(ch).Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = NonBlockingSend(ch).Write(ctx, 3)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 3)
			})
			t.Run("ClosedWrite", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int)
				close(ch)
				err := BlockingSend(ch).Write(ctx, 43)
				assert.ErrorIs(t, err, io.EOF)
			})
			t.Run("NonBlocking", func(t *testing.T) {
				t.Run("Wrote", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int)

					err := NonBlockingSend(ch).Write(ctx, 3)
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

					sent := NonBlockingSend(ch).Check(ctx, 3)
					assert.True(t, sent)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Check", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int, 1)

					NonBlockingSend(ch).Ignore(ctx, 3)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Canceled", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					ch := make(chan int)

					err := BlockingSend(ch).Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)
					err = NonBlockingSend(ch).Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)

					assert.True(t, !NonBlockingSend(ch).Check(ctx, 1))

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
				val, err := BlockingReceive(ch).Read(ctx)
				assert.NotError(t, err)
				assert.Equal(t, val, "hello world")

				ch <- "kip"
				val, ok := BlockingReceive(ch).Check(ctx)
				assert.Equal(t, val, "kip")
				assert.True(t, ok)

				ch <- "merlin"
				assert.True(t, BlockingReceive(ch).Drop(ctx))
				// do this to verify if it the channel is now empty
				_, err = NonBlockingReceive(ch).Read(ctx)
				assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)

				ch <- "deleuze"
				val = BlockingReceive(ch).Force(ctx)
				assert.Equal(t, val, "deleuze")
				assert.True(t, ok)
			})
			t.Run("NonBlocking", func(t *testing.T) {
				ch := make(chan string, 1)
				assert.Zero(t, NonBlockingReceive(ch).Force(ctx))
				assert.True(t, !NonBlockingReceive(ch).Drop(ctx))
				val, err := NonBlockingReceive(ch).Read(ctx)
				assert.Zero(t, val)
				assert.ErrorIs(t, err, ErrSkippedNonBlockingChannelOperation)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChannelOp[string]{mode: 0, ch: make(chan string)}
				val, err := op.Receive().Read(ctx)
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
				assert.Zero(t, val)
			})
		})
	})

}
