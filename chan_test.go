package fun

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ft"
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
			t.Run("Handler", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := Blocking(ch).Send().Handler().Run(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = NonBlocking(ch).Send().Handler().Run(ctx, 3)
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
			t.Run("MultiClose", func(t *testing.T) {
				ch := NonBlocking(make(chan int))
				for i := 0; i < 10; i++ {
					check.NotPanic(t, ch.Close)
				}
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
					assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)
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

					err = (ChanSend[int]{ch: ch}).Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, io.EOF)
					err = (ChanSend[int]{mode: 42, ch: ch}).Write(ctx, 1)
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

					ch <- "buddy"
					assert.True(t, Blocking(ch).Receive().Drop(ctx))
					// do this to verify if it the channel is now empty
					_, err = NonBlocking(ch).Receive().Read(ctx)
					assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)

					ch <- "deleuze"
					val = Blocking(ch).Receive().Force(ctx)
					assert.Equal(t, val, "deleuze")
					assert.True(t, ok)
				})
				t.Run("Generator", func(t *testing.T) {
					ch := make(chan string, 1)
					ch <- "hello world"
					val, err := Blocking(ch).Receive().Generator().Run(ctx)
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
				assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChanOp[string]{mode: 0, ch: make(chan string)}
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
					assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)
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

					err = (ChanSend[int]{ch: ch}).Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, io.EOF)
					err = (ChanSend[int]{mode: 42, ch: ch}).Write(ctx, 1)
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

				ch <- "buddy"
				assert.True(t, BlockingReceive(ch).Drop(ctx))
				// do this to verify if it the channel is now empty
				_, err = NonBlockingReceive(ch).Read(ctx)
				assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)

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
				assert.ErrorIs(t, err, ErrNonBlockingChannelOperationSkipped)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChanOp[string]{mode: 0, ch: make(chan string)}
				val, err := op.Receive().Read(ctx)
				assert.Error(t, err)
				assert.ErrorIs(t, err, io.EOF)
				assert.Zero(t, val)
			})
		})
	})
	t.Run("Consumption", func(t *testing.T) {
		perr := errors.New("panic error")
		experr := errors.New("anticpated")

		t.Run("Receive", func(t *testing.T) {
			t.Run("SimpleJob", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := Chan[string]().Blocking()
				var count int
				sig := ch.Receive().Consume(func(_ context.Context, in string) error {
					defer func() { count++ }()
					switch count {
					case 0:
						check.Equal(t, in, "hello world")
					case 1:
						check.Zero(t, in)
					default:
						check.Equal(t, in, "panic trigger")
						panic(perr)
					}
					return nil
				}).Signal(ctx)

				assert.NotError(t, ch.Send().Write(ctx, "hello world"))
				assert.NotError(t, ch.Send().Zero(ctx))
				assert.NotError(t, ch.Send().Write(ctx, "panic trigger"))
				assert.ErrorIs(t, <-sig, perr)

				cancel()
				ch.Close()
				assert.NotError(t, <-sig)
			})
			t.Run("Error", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := Chan[string](4).Blocking()
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "hello world"))
				assert.NotError(t, ch.Send().Zero(ctx))
				assert.NotError(t, ch.Send().Write(ctx, "third"))
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				err := ch.Receive().Consume(func(_ context.Context, in string) error {
					defer func() { count++ }()
					switch count {
					case 0:
						check.Equal(t, in, "hello world")
					case 1:
						check.Zero(t, in)
					case 2:
						check.Equal(t, in, "third")
					case 3:
						check.Equal(t, in, "beep")
						return experr
					}
					return nil
				}).Run(ctx)
				check.Equal(t, count, 4)
				assert.Error(t, err)
				assert.ErrorIs(t, err, experr)
			})
			t.Run("SuppressHandlerEOF", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := Blocking(make(chan string, 1))
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "beep"))

				err := ch.Receive().Consume(func(_ context.Context, in string) error {
					defer func() { count++ }()
					check.Equal(t, in, "beep")
					return io.EOF
				}).Run(ctx)

				check.Equal(t, count, 1)
				assert.NotError(t, err)
			})
			t.Run("ErrorPropogationEOF", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := Blocking(make(chan string, 1))
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				ch.Close()
				err := ch.Receive().Consume(func(_ context.Context, in string) error {
					defer func() { count++ }()
					check.Equal(t, in, "beep")
					return nil
				}).Run(ctx)

				check.Equal(t, count, 1)
				check.NotError(t, err)
			})
			t.Run("Continue", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := Blocking(make(chan string, 2))
				var count int
				var skipped bool
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				ch.Close()
				err := ch.Receive().Consume(func(_ context.Context, in string) error {
					defer func() { count++ }()
					check.Equal(t, in, "beep")
					if count == 1 {
						skipped = true
						return ErrStreamContinue
					}
					return nil
				}).Run(ctx)

				check.Equal(t, count, 2)
				check.True(t, skipped)
				check.NotError(t, err)
			})
		})
		t.Run("Seq", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := Chan[int](2)
			check.NotError(t, ch.Send().Write(ctx, 42))
			check.NotError(t, ch.Send().Write(ctx, 84))
			ch.Close()
			idx := 0
			for num := range ch.Seq(ctx) {
				switch idx {
				case 0:
					check.Equal(t, num, 42)
				case 1:
					check.Equal(t, num, 84)
				default:
					t.Fatal("impossible situation", idx, num)
				}
				idx++
			}
		})
		t.Run("Seq2", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := Chan[int](2)
			check.NotError(t, ch.Send().Write(ctx, 42))
			check.NotError(t, ch.Send().Write(ctx, 84))
			ch.Close()

			for idx, num := range ch.Seq2(ctx) {
				switch idx {
				case 0:
					check.Equal(t, num, 42)
				case 1:
					check.Equal(t, num, 84)
				default:
					t.Fatal("impossible situation", idx, num)
				}
			}
		})
		t.Run("Ok", func(t *testing.T) {
			t.Run("DirectReturns", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				check.True(t, NonBlockingReceive(ctx.Done()).Ok())
				cancel()
				check.True(t, !BlockingReceive(ctx.Done()).Ok())
				check.True(t, !NonBlockingReceive(ctx.Done()).Ok())
			})
			t.Run("Impossible", func(t *testing.T) {
				ch := &ChanOp[struct{}]{ch: make(chan struct{}), mode: 43}
				assert.Panic(t, func() { ch.Receive().Ok() })
			})
			t.Run("Blocks", func(t *testing.T) {
				assert.MaxRuntime(t, 500*time.Millisecond, func() {
					assert.MinRuntime(t, 100*time.Millisecond, func() {
						ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
						defer cancel()
						BlockingReceive(ctx.Done()).Ok()
					})
				})
			})
		})
	})
	t.Run("Pipe", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		proc, prod := Blocking(make(chan string, 1)).Pipe()
		check.NotError(t, proc(ctx, "hello world!"))
		output, err := prod(ctx)
		check.NotError(t, err)
		check.Equal(t, output, "hello world!")
	})
	t.Run("Default", func(t *testing.T) {
		t.Run("EndToEnd", func(t *testing.T) {
			ch := make(chan int)
			check.True(t, ch != nil)
			chCp := ch
			check.Equal(t, ch, chCp)
			ch = DefaultChan(ch, 12).Channel()
			check.True(t, ch != nil)
			check.Equal(t, cap(ch), 0)
			check.Equal(t, ch, chCp)
			ch = DefaultChan[int](nil, 12).Channel()
			check.NotEqual(t, ch, chCp)
			check.Equal(t, cap(ch), 12)
		})
		t.Run("Passthrough", func(t *testing.T) {
			ch := make(chan string, 1)
			ch <- "hello"
			pipe := DefaultChan(ch, 100)
			check.Equal(t, pipe.Channel(), ch)
			check.Equal(t, cap(ch), 1)
			check.Equal(t, cap(pipe.Channel()), 1)
		})
		t.Run("Create", func(t *testing.T) {
			pipe := DefaultChan[string](nil, 100)
			check.True(t, pipe.Channel() != nil)
			check.Equal(t, cap(pipe.Channel()), 100)
		})
		t.Run("Panic", func(t *testing.T) {
			check.Panic(t, func() {
				_ = DefaultChan[string](nil, 100, 2)
			})
		})
	})
	t.Run("Filter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := Blocking(make(chan int, 100))
		for i := 0; i < 100; i++ {
			assert.NotError(t, ch.Send().Write(ctx, i))
		}
		ch.Close()
		count := 0
		err := ch.Receive().
			Filter(ctx,
				func(err error) { check.NotError(t, err) },
				func(in int) bool { return in%2 == 0 && in != 0 },
			).
			Stream().
			Observe(func(in int) {
				count++
				check.True(t, ft.Not(in == 0))
				check.True(t, ft.Not(in%2 != 0))
			}).
			Run(ctx)

		check.NotError(t, err)
		check.Equal(t, count, 49)
	})
	t.Run("SizeReporters", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := Blocking(make(chan int, 100))
		check.Equal(t, 100, ch.Cap())
		check.Equal(t, 0, ch.Len())
		check.NotError(t, ch.Send().Write(ctx, 42))
		check.Equal(t, 100, ch.Cap())
		check.Equal(t, 1, ch.Len())
	})
}
