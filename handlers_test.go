package fun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
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
			unwinder := HF.ErrorUnwindTransformer(ers.FilterNoop())
			ctx := testt.Context(t)
			errs, err := unwinder(ctx, ers.Join(io.EOF, ErrNonBlockingChannelOperationSkipped, ErrInvariantViolation))
			assert.NotError(t, err)
			check.Equal(t, len(errs), 3)
		})
		t.Run("Empty", func(t *testing.T) {
			unwinder := HF.ErrorUnwindTransformer(ers.FilterNoop())
			ctx := testt.Context(t)
			errs, err := unwinder(ctx, nil)
			assert.NotError(t, err)
			check.Equal(t, len(errs), 0)
		})
	})
	t.Run("ErrorCollector", func(t *testing.T) {
		ob, prod := HF.ErrorCollector()
		ft.DoTimes(128, func() { ob(nil) })
		check.Equal(t, 0, len(prod.Force()))
		ft.DoTimes(128, func() { ob(ers.Error("test")) })
		check.Equal(t, 128, len(prod.Force()))
	})
	t.Run("StringFuture", func(t *testing.T) {
		t.Run("Sprintf", func(t *testing.T) { check.Equal(t, "hi:42", HF.Sprintf("%s:%d", "hi", 42)()) })
		t.Run("Sprintln", func(t *testing.T) { check.Equal(t, "hi : 42\n", HF.Sprintln("hi", ":", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", HF.Sprint("hi:", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", HF.Sprint("hi:", "42")()) })
		t.Run("Stringer", func(t *testing.T) { check.Equal(t, "Handlers<>", HF.Stringer(HF)()) })
		t.Run("Str", func(t *testing.T) { check.Equal(t, "hi:42", HF.Str([]any{"hi:", 42})()) })
		t.Run("Strf", func(t *testing.T) { check.Equal(t, "hi:42", HF.Strf("%s:%d", []any{"hi", 42})()) })
		t.Run("Strln", func(t *testing.T) { check.Equal(t, "hi : 42\n", HF.Strln([]any{"hi", ":", 42})()) })
		t.Run("StrJoin", func(t *testing.T) { check.Equal(t, "hi:42", HF.StrJoin([]string{"hi", ":", "42"})()) })
		t.Run("StrJoinWith", func(t *testing.T) { check.Equal(t, "hi : 42", HF.StrJoinWith([]string{"hi", ":", "42"}, " ")()) })
		t.Run("StrConcatinate", func(t *testing.T) { check.Equal(t, "hi:42", HF.StrConcatinate("hi", ":", "42")()) })
	})
	t.Run("Lines", func(t *testing.T) {
		buf := &bytes.Buffer{}
		last := sha256.Sum256([]byte(fmt.Sprint(time.Now().UTC().UnixMilli())))
		buf.WriteString(fmt.Sprintf("%x", last))
		for i := 1; i < 128; i++ {
			next := sha256.Sum256(last[:])
			buf.WriteString(fmt.Sprintf("\n%x", next))
		}

		count := 0
		ctx := testt.Context(t)
		var prev string
		check.NotError(t, HF.Lines(buf).Observe(ctx, func(line string) {
			count++
			assert.Equal(t, len(line), 64)
			assert.NotEqual(t, prev, line)
		}))
		check.Equal(t, count, 128)
	})
}

func (Handlers) String() string { return "Handlers<>" }
