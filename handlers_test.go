package fun

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/testt"
)

func TestHandlers(t *testing.T) {
	t.Parallel()
	const root ers.Error = ers.Error("root-error")
	t.Run("ErrorProcessor", func(t *testing.T) {
		count := 0
		ctx := testt.Context(t)
		proc := HF.ErrorProcessor(func(ctx context.Context, err error) error {
			count++
			return err
		})
		check.NotError(t, proc(ctx, nil))
		check.Equal(t, count, 0)

		check.ErrorIs(t, proc(ctx, root), root)
		check.Equal(t, count, 1)
		check.ErrorIs(t, proc(ctx, io.EOF), io.EOF)
		check.Equal(t, count, 2)
		check.NotError(t, proc(ctx, nil))
		check.Equal(t, count, 2)
	})
	t.Run("ErrorProcessorWithoutEOF", func(t *testing.T) {
		count := 0
		proc := HF.ErrorObserverWithoutEOF(func(err error) {
			count++
			if errors.Is(err, io.EOF) || err == nil {
				t.Error("unexpected error", err)
			}
		})
		proc(nil)
		check.Equal(t, count, 0)

		proc(root)
		check.Equal(t, count, 1)

		proc(io.EOF)
		check.Equal(t, count, 1)

		proc(context.Canceled)
		check.Equal(t, count, 2)

		proc(nil)
		check.Equal(t, count, 2)
	})
	t.Run("ErrorProcessorWithoutTerminating", func(t *testing.T) {
		count := 0
		proc := HF.ErrorObserverWithoutTerminating(func(err error) {
			count++
			if ers.IsTerminating(err) || err == nil {
				t.Error("unexpected error", err)
			}
		})
		proc(nil)
		check.Equal(t, count, 0)

		proc(root)
		check.Equal(t, count, 1)

		proc(io.EOF)
		check.Equal(t, count, 1)

		proc(context.Canceled)
		check.Equal(t, count, 1)

		proc(ErrInvariantViolation)
		check.Equal(t, count, 2)

		proc(nil)
		check.Equal(t, count, 2)
	})
	t.Run("Unwinder", func(t *testing.T) {
		t.Run("BasicUnwind", func(t *testing.T) {
			unwinder := HF.ErrorUndindTransformer(ers.FilterNoop())
			ctx := testt.Context(t)
			errs, err := unwinder(ctx, ers.Join(io.EOF, ErrNonBlockingChannelOperationSkipped, ErrInvariantViolation))
			assert.NotError(t, err)
			check.Equal(t, len(errs), 3)
		})
		t.Run("Empty", func(t *testing.T) {
			unwinder := HF.ErrorUndindTransformer(ers.FilterNoop())
			ctx := testt.Context(t)
			errs, err := unwinder(ctx, nil)
			assert.NotError(t, err)
			check.Equal(t, len(errs), 0)
		})
	})
}
