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
}
