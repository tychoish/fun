package fnx

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
)

func TestConverter(t *testing.T) {
	t.Run("ConverterOK", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tfrm := MakeCovnerterOk(func(in string) (string, bool) { return in, true })
		out, err := tfrm(ctx, "hello")
		check.Equal(t, out, "hello")
		check.NotError(t, err)
		tfrm = MakeCovnerterOk(func(in string) (string, bool) { return in, false })
		out, err = tfrm(ctx, "bye")
		check.Error(t, err)
		check.ErrorIs(t, err, ers.ErrCurrentOpSkip)
		check.Equal(t, out, "bye")
	})
	t.Run("ConverterErr", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		tfrm := MakeConverterErr(func(in string) (string, error) { return in, nil })
		out, err := tfrm(ctx, "hello")
		check.Equal(t, out, "hello")
		check.NotError(t, err)
		tfrm = MakeConverterErr(func(in string) (string, error) { return in, io.EOF })
		out, err = tfrm(ctx, "bye")
		check.Error(t, err)
		check.ErrorIs(t, err, io.EOF)
		check.Equal(t, out, "bye")
	})
	t.Run("Block", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			count := 0
			mpf := Converter[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), nil
			})

			out, err := mpf.Wait(42)
			check.Equal(t, "42", out)
			check.NotError(t, err)
			check.Equal(t, count, 1)
		})
		t.Run("Error", func(t *testing.T) {
			count := 0
			mpf := Converter[int, string](func(ctx context.Context, in int) (string, error) {
				check.Equal(t, ctx, context.Background())
				check.Equal(t, in, 42)
				count++
				return fmt.Sprint(in), io.EOF
			})

			out, err := mpf.Wait(42)
			check.Equal(t, "42", out)
			check.Error(t, err)
			check.ErrorIs(t, err, io.EOF)
			check.Equal(t, count, 1)
		})
	})
	t.Run("Lock", func(t *testing.T) {
		count := &atomic.Int64{}

		mpf := MakeConverter(func(in int) string {
			check.Equal(t, in, 42)
			count.Add(1)
			return fmt.Sprint(in)
		})
		mpf = mpf.Lock()
		// tempt the race detector
		wg := &WaitGroup{}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg.Group(128, func(ctx context.Context) {
			out, err := mpf(ctx, 42)
			check.Equal(t, out, "42")
			check.NotError(t, err)
		}).Run(ctx)
		wg.Wait(ctx)
		check.Equal(t, count.Load(), 128)
	})
	t.Run("Panic", func(t *testing.T) {
		mpf := MakeConverter(func(in int) string {
			panic(in)
		})
		check.Panic(t, func() { _, _ = mpf(t.Context(), 22) })
		check.NotPanic(t, func() { _, _ = mpf.WithRecover().Convert(t.Context(), 22) })
	})
	t.Run("ToAny", func(t *testing.T) {
	})
}
