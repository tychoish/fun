package fnx

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

func TestConstructors(t *testing.T) {
	t.Run("Converter", func(t *testing.T) {
		t.Run("WorkerToOp", func(t *testing.T) {
			ec := &erc.Collector{}
			converter := MAKE.ConvertWorkerToOperation(ec.Push)
			tctx := t.Context()
			count := 0
			op, err := converter.Convert(tctx, Worker(func(ctx context.Context) error {
				check.True(t, ctx != tctx)
				count++
				return io.EOF
			}))
			assert.NotError(t, err)
			op.Run(context.Background())
			assert.Equal(t, 1, count)
			err = ec.Resolve()
			assert.Error(t, err)
			assert.ErrorIs(t, err, io.EOF)
		})
		t.Run("OpToWorker", func(t *testing.T) {
			converter := MAKE.ConvertOperationToWorker()
			tctx := t.Context()
			count := 0
			op, err := converter.Convert(tctx, Operation(func(ctx context.Context) {
				check.True(t, ctx != tctx)
				count++
			}))
			assert.NotError(t, err)
			assert.NotError(t, op(context.Background()))
			assert.Equal(t, 1, count)
		})
	})

	t.Run("ImportedFromFNX", func(t *testing.T) {
		t.Run("MAKE.ErrorChannelWorker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			expected := errors.New("cat")
			var ch chan error
			t.Run("NilChannel", func(t *testing.T) {
				assert.NotError(t, MAKE.ErrorChannelWorker(ch).Run(ctx))
			})
			t.Run("ClosedChannel", func(t *testing.T) {
				ch = make(chan error)
				close(ch)
				assert.NotError(t, MAKE.ErrorChannelWorker(ch).Run(ctx))
			})
			t.Run("ContextCanceled", func(t *testing.T) {
				nctx, cancel := context.WithCancel(context.Background())
				cancel()
				ch = make(chan error)
				err := MAKE.ErrorChannelWorker(ch).Run(nctx)
				assert.ErrorIs(t, err, context.Canceled)
			})
			t.Run("Error", func(t *testing.T) {
				ch = make(chan error, 1)
				ch <- expected
				err := MAKE.ErrorChannelWorker(ch).Run(ctx)
				assert.ErrorIs(t, err, expected)
			})
			t.Run("NilError", func(t *testing.T) {
				ch = make(chan error, 1)
				ch <- nil
				err := MAKE.ErrorChannelWorker(ch).Run(ctx)
				assert.NotError(t, err)
			})
		})
		t.Run("MAKE.ErrorChannelWorker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ch := make(chan error, 1)
			wf := MAKE.ErrorChannelWorker(ch)
			root := ers.New("will-be-cached")
			ch <- root
			err := wf(ctx)
			check.ErrorIs(t, err, root)
			close(ch)
			check.NotError(t, wf(ctx))
			check.NotError(t, wf(ctx))
			check.NotError(t, wf(ctx))
		})
		t.Run("ContextChannelWorker", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			err := MAKE.ContextChannelWorker(ctx).Run(t.Context())
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.True(t, t.Context().Err() == nil)
		})
	})
	t.Run("Signal", func(t *testing.T) {
		closeWaiter, wait := MAKE.Signal()
		trigger, signal := MAKE.Signal()

		var count int
		wg := &sync.WaitGroup{}
		wg.Add(2)
		go func() {
			defer wg.Done()
			check.NotError(t, wait(t.Context()))
			check.Equal(t, count, 1)
			count++
			trigger()
		}()
		go func() {
			defer wg.Done()
			count++
			check.Equal(t, count, 1)
			closeWaiter()
			check.NotError(t, signal(t.Context()))
			check.Equal(t, count, 2)
			count++
		}()
		wg.Wait()

		assert.Equal(t, count, 3)
	})
	t.Run("WorkerHandler", func(t *testing.T) {
		count := 0
		worker := Worker(func(ctx context.Context) error { assert.NotNil(t, ctx); count++; return nil })
		wh := MAKE.WorkerHandler()
		assert.NotError(t, wh.Read(t.Context(), worker))
		assert.Equal(t, count, 1)
	})

	t.Run("OperationHandler", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			op := Operation(func(ctx context.Context) {
				assert.NotNil(t, ctx)
				count++
			})
			handler := MAKE.OperationHandler()
			assert.NotError(t, handler.Read(t.Context(), op))
			assert.Equal(t, count, 1)
		})

		t.Run("PanicHandling", func(t *testing.T) {
			t.Run("StringPanic", func(t *testing.T) {
				op := Operation(func(ctx context.Context) {
					panic("test panic")
				})
				handler := MAKE.OperationHandler()
				err := handler.Read(t.Context(), op)
				assert.Error(t, err)
				assert.Substring(t, err.Error(), "test panic")
			})

			t.Run("ErrorPanic", func(t *testing.T) {
				expectedErr := errors.New("panic error")
				op := Operation(func(ctx context.Context) {
					panic(expectedErr)
				})
				handler := MAKE.OperationHandler()
				err := handler.Read(t.Context(), op)
				assert.Error(t, err)
				assert.ErrorIs(t, err, expectedErr)
			})

			t.Run("InterfacePanic", func(t *testing.T) {
				op := Operation(func(ctx context.Context) {
					panic(42)
				})
				handler := MAKE.OperationHandler()
				err := handler.Read(t.Context(), op)
				assert.Error(t, err)
			})

			t.Run("StructPanic", func(t *testing.T) {
				type testStruct struct {
					msg string
				}
				op := Operation(func(ctx context.Context) {
					panic(testStruct{msg: "structured panic"})
				})
				handler := MAKE.OperationHandler()
				err := handler.Read(t.Context(), op)
				assert.Error(t, err)
			})
		})

		t.Run("ContextCancellation", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			executed := false
			op := Operation(func(ctx context.Context) {
				executed = true
			})

			handler := MAKE.OperationHandler()
			// The operation should still execute even with a canceled context
			// because the handler doesn't check the context before running
			err := handler.Read(ctx, op)
			assert.NotError(t, err)
			assert.True(t, executed)
		})

		t.Run("MultipleOperations", func(t *testing.T) {
			handler := MAKE.OperationHandler()
			count := 0

			for i := 0; i < 10; i++ {
				op := Operation(func(ctx context.Context) {
					count++
				})
				assert.NotError(t, handler.Read(t.Context(), op))
			}

			assert.Equal(t, count, 10)
		})

		t.Run("PanicRecoveryDoesNotAffectSubsequentCalls", func(t *testing.T) {
			handler := MAKE.OperationHandler()

			// First call panics
			panicOp := Operation(func(ctx context.Context) {
				panic("first panic")
			})
			err := handler.Read(t.Context(), panicOp)
			assert.Error(t, err)

			// Second call should work fine
			count := 0
			normalOp := Operation(func(ctx context.Context) {
				count++
			})
			assert.NotError(t, handler.Read(t.Context(), normalOp))
			assert.Equal(t, count, 1)

			// Third call panics again
			panicOp2 := Operation(func(ctx context.Context) {
				panic("second panic")
			})
			err = handler.Read(t.Context(), panicOp2)
			assert.Error(t, err)
		})

		t.Run("ConcurrentExecution", func(t *testing.T) {
			handler := MAKE.OperationHandler()
			var mu sync.Mutex
			count := 0
			wg := &sync.WaitGroup{}

			for i := 0; i < 100; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					op := Operation(func(ctx context.Context) {
						mu.Lock()
						count++
						mu.Unlock()
					})
					err := handler.Read(context.Background(), op)
					check.NotError(t, err)
				}()
			}

			wg.Wait()
			assert.Equal(t, count, 100)
		})
	})
}
