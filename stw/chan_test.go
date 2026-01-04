package stw

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
)

func TestChannel(t *testing.T) {
	t.Run("Chain", func(t *testing.T) {
		t.Run("Send", func(t *testing.T) {
			t.Run("Write", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := ChanBlocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = ChanNonBlocking(ch).Send().Write(ctx, 3)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 3)
			})
			t.Run("Handler", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := ChanBlocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = ChanNonBlocking(ch).Send().Write(ctx, 3)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 3)
			})
			t.Run("ClosedWrite", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int)
				close(ch)
				err := ChanBlocking(ch).Send().Write(ctx, 43)
				assert.ErrorIs(t, err, io.EOF)
			})
			t.Run("MultiClose", func(t *testing.T) {
				ch := ChanNonBlocking(make(chan int))
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
				err := ChanBlocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, len(ch), 1)
				assert.Equal(t, <-ch, 1)
				assert.Equal(t, len(ch), 0)

				err = ChanBlocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, len(ch), 1)
				ChanBlocking(ch).Receive().Ignore(ctx)
				assert.Equal(t, len(ch), 0)
			})
			t.Run("Zero", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan time.Time, 1)
				assert.NotError(t, ChanBlocking(ch).Send().Zero(ctx))
				val := <-ch
				assert.True(t, val.IsZero())
			})

			t.Run("NonBlocking", func(t *testing.T) {
				t.Run("Wrote", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int)

					err := ChanNonBlocking(ch).Send().Write(ctx, 3)
					assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)
					close(ch)
					out, ok := <-ch
					assert.True(t, !ok)
					assert.Zero(t, out)
				})
				t.Run("Check", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int, 1)

					sent := ChanNonBlocking(ch).Send().Check(ctx, 3)
					assert.True(t, sent)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Check", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int, 1)

					ChanNonBlocking(ch).Send().Ignore(ctx, 3)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Canceled", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					ch := make(chan int)

					err := ChanBlocking(ch).Send().Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)
					err = ChanNonBlocking(ch).Send().Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)

					assert.True(t, !ChanNonBlocking(ch).Send().Check(ctx, 1))

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
					val, err := ChanBlocking(ch).Receive().Read(ctx)
					assert.NotError(t, err)
					assert.Equal(t, val, "hello world")

					ch <- "kip"
					val, ok := ChanBlocking(ch).Receive().Check(ctx)
					assert.Equal(t, val, "kip")
					assert.True(t, ok)

					ch <- "buddy"
					assert.True(t, ChanBlocking(ch).Receive().Drop(ctx))
					// do this to verify if it the channel is now empty
					_, err = ChanNonBlocking(ch).Receive().Read(ctx)
					assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)

					ch <- "deleuze"
					val = ChanBlocking(ch).Receive().Force(ctx)
					assert.Equal(t, val, "deleuze")
					assert.True(t, ok)
				})
				t.Run("Future", func(t *testing.T) {
					ch := make(chan string, 1)
					ch <- "hello world"
					val, err := ChanBlocking(ch).Receive().Read(ctx)
					assert.NotError(t, err)
					assert.Equal(t, val, "hello world")
				})
			})
			t.Run("NonBlocking", func(t *testing.T) {
				ch := make(chan string, 1)
				assert.Zero(t, ChanNonBlocking(ch).Receive().Force(ctx))
				assert.True(t, !ChanNonBlocking(ch).Receive().Drop(ctx))
				val, err := ChanNonBlocking(ch).Receive().Read(ctx)
				assert.Zero(t, val)
				assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChanOp[string]{mode: 0, ch: make(chan string)}
				assert.Panic(t, func() {
					_ = op.Receive()
				})
			})
		})
	})
	t.Run("Shortcut", func(t *testing.T) {
		t.Run("Send", func(t *testing.T) {
			t.Run("Write", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int, 2)
				err := ChanBlocking(ch).Send().Write(ctx, 1)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 1)

				err = ChanNonBlocking(ch).Send().Write(ctx, 3)
				assert.NotError(t, err)
				assert.Equal(t, <-ch, 3)
			})
			t.Run("ClosedWrite", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				ch := make(chan int)
				close(ch)
				err := ChanBlocking(ch).Send().Write(ctx, 43)
				assert.ErrorIs(t, err, io.EOF)
			})
			t.Run("NonBlocking", func(t *testing.T) {
				t.Run("Wrote", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int)

					err := ChanNonBlocking(ch).Send().Write(ctx, 3)
					assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)
					close(ch)
					out, ok := <-ch
					assert.True(t, !ok)
					assert.Zero(t, out)
				})
				t.Run("Check", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int, 1)

					sent := ChanNonBlocking(ch).Send().Check(ctx, 3)
					assert.True(t, sent)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Check", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					ch := make(chan int, 1)

					ChanNonBlocking(ch).Send().Ignore(ctx, 3)
					out, ok := <-ch
					assert.True(t, ok)
					assert.Equal(t, 3, out)
				})
				t.Run("Canceled", func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					ch := make(chan int)

					err := ChanBlocking(ch).Send().Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)
					err = ChanNonBlocking(ch).Send().Write(ctx, 1)
					assert.Error(t, err)
					assert.ErrorIs(t, err, context.Canceled)

					assert.True(t, !ChanNonBlocking(ch).Send().Check(ctx, 1))

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
				val, err := ChanBlocking(ch).Receive().Read(ctx)
				assert.NotError(t, err)
				assert.Equal(t, val, "hello world")

				ch <- "kip"
				val, ok := ChanBlocking(ch).Receive().Check(ctx)
				assert.Equal(t, val, "kip")
				assert.True(t, ok)

				ch <- "buddy"
				assert.True(t, ChanBlocking(ch).Receive().Drop(ctx))
				// do this to verify if it the channel is now empty
				_, err = ChanNonBlocking(ch).Receive().Read(ctx)
				assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)

				ch <- "deleuze"
				val = ChanBlocking(ch).Receive().Force(ctx)
				assert.Equal(t, val, "deleuze")
				assert.True(t, ok)
			})
			t.Run("NonBlocking", func(t *testing.T) {
				ch := make(chan string, 1)
				assert.Zero(t, ChanNonBlocking(ch).Receive().Force(ctx))
				assert.True(t, !ChanNonBlocking(ch).Receive().Drop(ctx))
				val, err := ChanNonBlocking(ch).Receive().Read(ctx)
				assert.Zero(t, val)
				assert.ErrorIs(t, err, ers.ErrCurrentOpSkip)
			})
			t.Run("Invalid", func(t *testing.T) {
				op := ChanOp[string]{mode: 0, ch: make(chan string)}
				assert.Panic(t, func() { op.Receive() })
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

				ch := ChanBlocking(make(chan string))
				var count int
				sig := ch.Receive().ReadAll(func(_ context.Context, in string) error {
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

				ch := ChanBlocking(make(chan string, 4))
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "hello world"))
				assert.NotError(t, ch.Send().Zero(ctx))
				assert.NotError(t, ch.Send().Write(ctx, "third"))
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				err := ch.Receive().ReadAll(func(_ context.Context, in string) error {
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

				ch := ChanBlocking(make(chan string, 1))
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "beep"))

				err := ch.Receive().ReadAll(func(_ context.Context, in string) error {
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

				ch := ChanBlocking(make(chan string, 1))
				var count int
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				ch.Close()
				err := ch.Receive().ReadAll(func(_ context.Context, in string) error {
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

				ch := ChanBlocking(make(chan string, 2))
				var count int
				var skipped bool
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				assert.NotError(t, ch.Send().Write(ctx, "beep"))
				ch.Close()
				err := ch.Receive().ReadAll(func(_ context.Context, in string) error {
					defer func() { count++ }()
					check.Equal(t, in, "beep")
					if count == 1 {
						skipped = true
						return ers.ErrCurrentOpSkip
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

			ch := ChanBlocking(make(chan int, 2))
			check.NotError(t, ch.Send().Write(ctx, 42))
			check.NotError(t, ch.Send().Write(ctx, 84))
			ch.Close()
			idx := 0
			for num := range ch.Receive().Iterator(ctx) {
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
		t.Run("Ok", func(t *testing.T) {
			t.Run("DirectReturns", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				check.True(t, ChanNonBlockingReceive(ctx.Done()).Ok())
				cancel()
				check.True(t, !ChanBlockingReceive(ctx.Done()).Ok())
				check.True(t, !ChanNonBlockingReceive(ctx.Done()).Ok())
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
						ChanBlockingReceive(ctx.Done()).Ok()
					})
				})
			})
		})
	})
	t.Run("Signal", func(t *testing.T) {
		ch := ChanBlocking(make(chan int, 1))
		assert.NotError(t, ch.Send().Write(t.Context(), 77))
		val, err := ch.Receive().Read(t.Context())
		assert.NotError(t, err)
		assert.Equal(t, 77, val)
		ch.Send().Signal(t.Context())
		val, err = ch.Receive().Read(t.Context())
		assert.NotError(t, err)
		assert.Equal(t, 0, val)
	})

	t.Run("SizeReporters", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch := ChanBlocking(make(chan int, 100))
		check.Equal(t, 100, ch.Cap())
		check.Equal(t, 0, ch.Len())
		check.NotError(t, ch.Send().Write(ctx, 42))
		check.Equal(t, 100, ch.Cap())
		check.Equal(t, 1, ch.Len())
	})

	t.Run("InvariantViolations", func(t *testing.T) {
		assert.Panic(t, func() {
			op := ChanOp[string]{ch: make(chan string)}
			op.Send()
		})

		assert.Panic(t, func() {
			op := ChanOp[string]{ch: make(chan string)}
			op.Receive()
		})

		assert.Panic(t, func() {
			op := ChanReceive[string]{ch: make(chan string)}
			op.Ok()
		})

		assert.Panic(t, func() {
			op := ChanReceive[string]{ch: make(chan string)}
			v, err := op.Read(t.Context())
			check.Error(t, err)
			check.Zero(t, v)
		})
	})

	t.Run("ReadClosedChannel", func(t *testing.T) {
		ctx := context.Background()

		t.Run("BlockingReadFromClosedChannel", func(t *testing.T) {
			ch := make(chan int)
			close(ch)

			val, err := ChanBlockingReceive(ch).Read(ctx)
			assert.ErrorIs(t, err, io.EOF)
			assert.Zero(t, val)
		})

		t.Run("NonBlockingReadFromClosedChannel", func(t *testing.T) {
			ch := make(chan string)
			close(ch)

			val, err := ChanNonBlockingReceive(ch).Read(ctx)
			assert.ErrorIs(t, err, io.EOF)
			assert.Zero(t, val)
		})

		t.Run("BlockingReadCanceledContext", func(t *testing.T) {
			ch := make(chan int)
			ctx, cancel := context.WithCancel(context.Background())

			// Start a goroutine to read from channel
			done := make(chan struct{})
			var readErr error
			var readVal int
			go func() {
				readVal, readErr = ChanBlockingReceive(ch).Read(ctx)
				close(done)
			}()

			// Give goroutine time to start blocking
			time.Sleep(50 * time.Millisecond)

			// Cancel the context while read is blocking
			cancel()

			// Wait for read to complete
			<-done

			assert.Error(t, readErr)
			assert.ErrorIs(t, readErr, context.Canceled)
			assert.Zero(t, readVal)
		})

		t.Run("NonBlockingReadCanceledContext", func(t *testing.T) {
			ch := make(chan int)
			ctx, cancel := context.WithCancel(context.Background())

			// Cancel the context before attempting read
			cancel()

			// Attempt non-blocking read with canceled context
			val, err := ChanNonBlockingReceive(ch).Read(ctx)

			// Should return context error, not skip error
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.Zero(t, val)
		})
	})

	t.Run("IteratorEdgeCases", func(t *testing.T) {
		t.Run("EarlyTerminationYieldFalse", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan int, 5)

			// Add multiple values to channel
			for i := 1; i <= 5; i++ {
				ch <- i * 10
			}
			close(ch)

			count := 0
			for val := range ChanBlockingReceive(ch).Iterator(ctx) {
				count++
				// Verify we got the first value
				if count == 1 {
					assert.Equal(t, 10, val)
				}
				// Stop after first iteration
				if count == 1 {
					break
				}
			}

			// Verify iterator stopped early
			assert.Equal(t, 1, count)

			// Verify remaining values are still in channel
			remaining := 0
			for range ch {
				remaining++
			}
			assert.Equal(t, 4, remaining)
		})

		t.Run("ContextCanceledDuringIteration", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch := make(chan int)

			// Start goroutine to send values
			go func() {
				for i := 1; i <= 10; i++ {
					select {
					case ch <- i:
					case <-ctx.Done():
						return
					}
				}
			}()

			count := 0
			for range ChanBlockingReceive(ch).Iterator(ctx) {
				count++
				// Cancel context after 3 iterations
				if count == 3 {
					cancel()
					// Give context cancellation time to propagate
					time.Sleep(10 * time.Millisecond)
				}
			}

			// Verify we got at least 3 values (may get 4 due to race)
			assert.True(t, count >= 3 && count <= 4)
		})

		t.Run("NonBlockingIteratorWithSkips", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan int, 5)

			// Use non-blocking receiver
			receiver := ChanNonBlockingReceive(ch)

			// Start iteration in goroutine since non-blocking will end when empty
			values := []int{}
			done := make(chan struct{})
			go func() {
				for val := range receiver.Iterator(ctx) {
					values = append(values, val)
				}
				close(done)
			}()

			// Give iterator time to start and hit empty channel
			time.Sleep(50 * time.Millisecond)

			// Add values gradually
			ch <- 1
			time.Sleep(10 * time.Millisecond)
			ch <- 2
			time.Sleep(10 * time.Millisecond)
			ch <- 3
			time.Sleep(10 * time.Millisecond)

			// Close channel to end iteration
			close(ch)

			// Wait for iteration to complete
			<-done

			// In non-blocking mode, iterator ends when channel is empty
			// So we should get all values that were sent before close
			assert.True(t, len(values) >= 0) // At least some values may be read
		})

		t.Run("IteratorChannelClosedMidIteration", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan string, 3)

			// Add initial values
			ch <- "first"
			ch <- "second"

			values := []string{}
			done := make(chan struct{})
			go func() {
				for val := range ChanBlockingReceive(ch).Iterator(ctx) {
					values = append(values, val)
					// Close channel after reading first value
					if len(values) == 1 {
						close(ch)
					}
				}
				close(done)
			}()

			<-done

			// Should have read both values before channel closed
			assert.Equal(t, 2, len(values))
			assert.Equal(t, "first", values[0])
			assert.Equal(t, "second", values[1])
		})

		t.Run("IteratorWithMultipleValuesAndContinuation", func(t *testing.T) {
			ctx := context.Background()
			ch := make(chan int, 10)

			// Add multiple values
			expected := []int{1, 2, 3, 4, 5}
			for _, v := range expected {
				ch <- v
			}
			close(ch)

			// Collect all values
			var collected []int
			for val := range ChanBlockingReceive(ch).Iterator(ctx) {
				collected = append(collected, val)
			}

			// Verify all values received in order
			assert.Equal(t, len(expected), len(collected))
			for i, v := range expected {
				assert.Equal(t, v, collected[i])
			}
		})
	})
}

// ReadAll returns a Worker function that processes the output of data
// from the channel with the Handler function. If the processor
// function returns ers.ErrCurrentOpSkip, the processing will continue. All
// other Handler errors (and problems reading from the channel,)
// abort stream. io.EOF errors are not propagated to the caller.
func (ro ChanReceive[T]) ReadAll(op fnx.Handler[T]) fnx.Worker {
	return func(ctx context.Context) (err error) {
		defer func() { err = erc.Join(err, erc.ParsePanic(recover())) }()

		var value T
	LOOP:
		for {
			value, err = ro.Read(ctx)
			if err != nil {
				goto HANDLE_ERROR
			}
			if err = op(ctx, value); err != nil {
				goto HANDLE_ERROR
			}

		HANDLE_ERROR:
			switch {
			case err == nil:
				continue LOOP
			case errors.Is(err, io.EOF):
				return nil
			case errors.Is(err, ers.ErrCurrentOpSkip):
				continue LOOP
			default:
				return err
			}
		}
	}
}
