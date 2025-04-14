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
	"github.com/tychoish/fun/internal"
)

func TestHandlers(t *testing.T) {
	t.Parallel()
	const root ers.Error = ers.Error("root-error")
	t.Run("ErrorHandlerSingle", func(t *testing.T) {
		hf, ef := MAKE.ErrorHandlerSingle()

		t.Run("NoopsOnZero", func(t *testing.T) {
			ft.DoTimes(100, func() {
				hf(nil)
				assert.NotError(t, ef())
			})
		})

		t.Run("OnceStatic", func(t *testing.T) {
			hf(root)
			assert.ErrorIs(t, ef(), root)
			ft.DoTimes(100, func() {
				hf(nil)
				assert.ErrorIs(t, ef(), root)
			})
		})

		t.Run("OtherErrors", func(t *testing.T) {
			hf(ers.Error("hello"))
			assert.ErrorIs(t, ef(), root)
		})
	})
	t.Run("ErrorHandlerAbort", func(t *testing.T) {
		count := 0
		eh := MAKE.ErrorHandlerWithAbort(func() { count++ })
		checkNoopSemantics := func(n int) {
			t.Run("NoopErrors", func(t *testing.T) {
				t.Run("NotError", func(t *testing.T) {
					eh(nil)
					check.Equal(t, count, n)
					var err error
					eh(err)
					check.Equal(t, count, n)
				})
				t.Run("ContextCanceled", func(t *testing.T) {
					eh(context.Canceled)
					check.Equal(t, count, n)

					eh(context.DeadlineExceeded)
					check.Equal(t, count, n)
				})
				t.Run("Wrapped", func(t *testing.T) {
					eh(fmt.Errorf("oops: %w", context.Canceled))
					check.Equal(t, count, n)

					eh(fmt.Errorf("oops: %w", context.DeadlineExceeded))
					check.Equal(t, count, n)
				})

			})
		}
		checkNoopSemantics(0)

		eh(root)
		check.Equal(t, count, 1)

		checkNoopSemantics(1)

		eh(root)

		check.Equal(t, count, 2)
	})
	t.Run("ErrorHandler", func(t *testing.T) {
		count := 0
		proc, fut := MAKE.ErrorCollector()
		proc = proc.Join(func(err error) { count++ })

		proc(nil)
		check.NotError(t, fut())
		check.Equal(t, count, 1)
		check.Equal(t, len(ers.Unwind(fut())), 0)

		proc(root)
		check.ErrorIs(t, fut(), root)
		check.Equal(t, len(ers.Unwind(fut())), 1)
		check.Equal(t, count, 2) //

		proc(io.EOF)
		check.ErrorIs(t, fut(), io.EOF) //
		check.ErrorIs(t, fut(), root)
		check.Equal(t, len(ers.Unwind(fut())), 2)
		check.Equal(t, count, 3) //

		proc(nil)
		check.Equal(t, count, 4) //
		check.Equal(t, len(ers.Unwind(fut())), 2)
	})
	t.Run("Error", func(t *testing.T) {
		called := 0
		oef := MAKE.ErrorHandler(func(_ error) { called++ })
		oef(nil)
		check.Equal(t, called, 0)
		oef(io.EOF)
		check.Equal(t, called, 1)
	})
	t.Run("ErrorHandlerWithoutEOF", func(t *testing.T) {
		count := 0
		proc := MAKE.ErrorHandlerWithoutTerminating(func(err error) {
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
	t.Run("Recover", func(t *testing.T) {
		var called bool
		ob := func(err error) {
			check.Error(t, err)
			check.ErrorIs(t, err, ers.ErrRecoveredPanic)
			called = true
		}
		assert.NotPanic(t, func() {
			defer MAKE.Recover(ob)
			panic("hi")
		})
		check.True(t, called)
	})
	t.Run("ErrorHandlerWithoutTerminating", func(t *testing.T) {
		count := 0
		proc := MAKE.ErrorHandlerWithoutTerminating(func(err error) {
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
		check.Equal(t, count, 2)

		proc(ers.ErrInvariantViolation)
		check.Equal(t, count, 3)

		proc(nil)
		check.Equal(t, count, 3)
	})
	t.Run("ErrorHandlerWithoutCancelation", func(t *testing.T) {
		count := 0
		proc := MAKE.ErrorHandlerWithoutCancelation(func(err error) {
			count++
			if ers.IsExpiredContext(err) || err == nil {
				t.Error("unexpected error", err)
			}
		})
		proc(nil)
		check.Equal(t, count, 0)

		proc(context.DeadlineExceeded)
		check.Equal(t, count, 0)

		proc(root)
		check.Equal(t, count, 1)

		proc(io.EOF)
		check.Equal(t, count, 2)

		proc(context.Canceled)
		check.Equal(t, count, 2)

		proc(ers.ErrInvariantViolation)
		check.Equal(t, count, 3)

		proc(nil)
		check.Equal(t, count, 3)
	})
	t.Run("Unwinder", func(t *testing.T) {
		t.Run("BasicUnwind", func(t *testing.T) {
			unwinder := MAKE.ErrorUnwindTransformer(ers.FilterNoop())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errs, err := unwinder(ctx, ers.Join(io.EOF, ErrNonBlockingChannelOperationSkipped, ers.ErrInvariantViolation))
			assert.NotError(t, err)
			check.Equal(t, len(errs), 3)
		})
		t.Run("Empty", func(t *testing.T) {
			unwinder := MAKE.ErrorUnwindTransformer(ers.FilterNoop())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errs, err := unwinder(ctx, nil)
			assert.NotError(t, err)
			check.Equal(t, len(errs), 0)
		})
	})
	t.Run("ErrorCollector", func(t *testing.T) {
		ob, prod := MAKE.ErrorCollector()
		ft.DoTimes(128, func() { ob(nil) })
		check.Equal(t, 0, len(internal.Unwind(prod.Resolve())))
		ft.DoTimes(128, func() { ob(ers.Error("test")) })
		check.Equal(t, 128, len(internal.Unwind(prod.Resolve())))
	})
	t.Run("StringFuture", func(t *testing.T) {
		t.Run("Sprintf", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprintf("%s:%d", "hi", 42)()) })
		t.Run("Sprintln", func(t *testing.T) { check.Equal(t, "hi : 42\n", MAKE.Sprintln("hi", ":", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprint("hi:", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprint("hi:", "42")()) })
		t.Run("Stringer", func(t *testing.T) { check.Equal(t, "Handlers<>", MAKE.Stringer(MAKE)()) })
		t.Run("Str", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Str([]any{"hi:", 42})()) })
		t.Run("Strf", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Strf("%s:%d", []any{"hi", 42})()) })
		t.Run("Strln", func(t *testing.T) { check.Equal(t, "hi : 42\n", MAKE.Strln([]any{"hi", ":", 42})()) })
		t.Run("StrJoin/Empty", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.StrJoin([]string{"hi", ":", "42"}, "")()) })
		t.Run("StrJoin/Dots", func(t *testing.T) { check.Equal(t, "hi.:.42", MAKE.StrJoin([]string{"hi", ":", "42"}, ".")()) })
		t.Run("StrSliceConcatinate", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.StrSliceConcatinate([]string{"hi", ":", "42"})()) })
		t.Run("StrConcatinate", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.StrConcatinate("hi", ":", "42")()) })
	})
	t.Run("Lines", func(t *testing.T) {
		buf := &bytes.Buffer{}
		last := sha256.Sum256([]byte(fmt.Sprint(time.Now().UTC().UnixMilli())))
		_, _ = fmt.Fprintf(buf, "%x", last)
		for i := 1; i < 128; i++ {
			next := sha256.Sum256(last[:])
			_, _ = fmt.Fprintf(buf, "\n%x", next)
		}

		count := 0
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var prev string
		check.NotError(t, MAKE.LinesWithSpaceTrimed(buf).ReadAll2(func(line string) {
			count++
			assert.Equal(t, len(line), 64)
			assert.NotEqual(t, prev, line)
		}).Run(ctx))
		check.Equal(t, count, 128)
	})
	t.Run("Transforms", func(t *testing.T) {
		t.Run("Itoa", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			out, err := ConvertStream(MAKE.Counter(10), MAKE.Itoa()).Slice(ctx)
			check.NotError(t, err)
			check.EqualItems(t, out, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"})
		})
		t.Run("Atoi", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			out := ft.Must(ConvertStream(SliceStream([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}), MAKE.Atoi()).Slice(ctx))

			check.EqualItems(t, out, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		})
	})
	t.Run("ErrorStack", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			assert.Equal(t, 0, MAKE.ErrorStackStream(&ers.Stack{}).Count(context.Background()))
		})
		t.Run("Number", func(t *testing.T) {
			errs := &ers.Stack{}
			errs.Add(ers.New("foof"), ers.New("boop"))
			assert.Equal(t, 2, MAKE.ErrorStackStream(errs).Count(context.Background()))
		})

		t.Run("Is", func(t *testing.T) {
			const err ers.Error = "foo"
			errs := &ers.Stack{}
			errs.Add(err)
			errs.Add(ers.Error("bar"))
			errs.Add(ers.Error("baz"))

			var count int
			var match int
			for e := range MAKE.ErrorStackStream(errs).Seq(context.Background()) {
				count++
				if errors.Is(e, err) {
					match++
				}
			}
			check.Equal(t, count, 3)
			check.Equal(t, match, 1)
		})

	})

}

func (Constructors) String() string { return "Handlers<>" }
