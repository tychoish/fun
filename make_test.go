package fun

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

func TestHandlers(t *testing.T) {
	t.Parallel()
	const root ers.Error = ers.Error("root-error")
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
			unwinder := MAKE.ErrorUnwindTransformer(erc.NewFilter())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errs, err := unwinder(ctx, erc.Join(io.EOF, ErrNonBlockingChannelOperationSkipped, ers.ErrInvariantViolation))
			assert.NotError(t, err)
			check.Equal(t, len(errs), 3)
		})
		t.Run("Empty", func(t *testing.T) {
			unwinder := MAKE.ErrorUnwindTransformer(erc.NewFilter())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errs, err := unwinder(ctx, nil)
			assert.NotError(t, err)
			check.Equal(t, len(errs), 0)
		})
	})
	t.Run("StringFuture", func(t *testing.T) {
		t.Run("Sprintf", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprintf("%s:%d", "hi", 42)()) })
		t.Run("Sprintln", func(t *testing.T) { check.Equal(t, "hi : 42\n", MAKE.Sprintln("hi", ":", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprint("hi:", 42)()) })
		t.Run("Sprint", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Sprint("hi:", "42")()) })
		t.Run("Stringer", func(t *testing.T) { check.Equal(t, "Constructors<>", MAKE.Stringer(MAKE)()) })
		t.Run("Str", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Str([]any{"hi:", 42})()) })
		t.Run("Strf", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.Strf("%s:%d", []any{"hi", 42})()) })
		t.Run("Strln", func(t *testing.T) { check.Equal(t, "hi : 42\n", MAKE.Strln([]any{"hi", ":", 42})()) })
		t.Run("StringsJoin/Empty", func(t *testing.T) { check.Equal(t, "hi:42", MAKE.StringsJoin([]string{"hi", ":", "42"}, "")()) })
		t.Run("StringsJoin/Dots", func(t *testing.T) { check.Equal(t, "hi.:.42", MAKE.StringsJoin([]string{"hi", ":", "42"}, ".")()) })
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
		check.NotError(t, MAKE.LinesWithSpaceTrimed(buf).ReadAll(func(line string) {
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
			out, err := MAKE.Itoa().Stream(MAKE.Counter(10)).Slice(ctx)
			check.NotError(t, err)
			check.EqualItems(t, out, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"})
		})
		t.Run("Atoi", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			out, err := MAKE.Atoi().Stream(SliceStream([]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"})).Slice(ctx)
			assert.NotError(t, err)

			check.EqualItems(t, out, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		})
	})
	t.Run("Signal", func(t *testing.T) {
		close, wait := MAKE.Signal()
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
			close()
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
	t.Run("ErrorStream", func(t *testing.T) {
		ec := &erc.Collector{}
		ec.Push(ers.ErrContainerClosed)
		ec.Push(ers.ErrCurrentOpAbort)
		ec.Push(io.EOF)

		errs, err := MAKE.ErrorStream(ec).Slice(t.Context())
		check.NotError(t, err)
		t.Log(err, len(errs), errs)
		check.Equal(t, len(errs), 3)
		check.ErrorIs(t, errs[0], ers.ErrContainerClosed)
		check.ErrorIs(t, errs[1], ers.ErrCurrentOpAbort)
		check.ErrorIs(t, errs[2], io.EOF)
	})
	t.Run("Strings", func(t *testing.T) {
		sl := []error{io.EOF, context.Canceled, ers.ErrLimitExceeded}
		strs := ft.Must(MAKE.ConvertErrorsToStrings().Wait(sl))
		merged := strings.Join(strs, ": ")
		check.Substring(t, merged, "EOF")
		check.Substring(t, merged, "context canceled")
		check.Substring(t, merged, "limit exceeded")
	})

	t.Run("WorkerPools", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			// most of these depend on the race detector not hitting errors
			t.Run("Basic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; return nil })
					check.NotError(t, MAKE.RunAllWorkers(VariadicStream(worker, worker, worker, worker, worker)).Run(tctx))
					check.Equal(t, 5, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++ })

					check.NotError(t, MAKE.RunAllOperations(VariadicStream(op, op, op, op, op)).Run(tctx))
					check.Equal(t, 5, count)
				})
			})
			t.Run("Panic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; panic("oops") })
					check.NotPanic(t, func() {
						check.Error(t, MAKE.RunAllWorkers(VariadicStream(worker, worker, worker, worker, worker)).Run(tctx))
					})
					check.Equal(t, 1, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++; panic("oops") })

					check.NotPanic(t, func() {
						check.Error(t, MAKE.RunAllOperations(VariadicStream(op, op, op, op, op)).Run(tctx))
					})

					check.Equal(t, 1, count)
				})
			})
			t.Run("Force", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := Worker(func(ctx context.Context) error { check.True(t, ctx != tctx); count++; return nil })
					check.NotError(t, MAKE.ForceRunAllWorkers(VariadicStream(worker, worker, worker, worker, worker)))
					check.Equal(t, 5, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()

					check.NotError(t, MAKE.ForceRunAllOperations(VariadicStream(op, op, op, op, op)))
					check.Equal(t, 5, count)
				})
			})
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
					op(context.Background())
					assert.Equal(t, 1, count)
					err = ec.Resolve()
					assert.Error(t, err)
					assert.ErrorIs(t, err, io.EOF)
				})
				t.Run("OpToWorker", func(t *testing.T) {
					converter := MAKE.ConvertOperationToWorker()
					tctx := t.Context()
					count := 0
					worker, err := converter.Convert(tctx, Operation(func(ctx context.Context) {
						check.True(t, ctx != tctx)
						count++
					}))
					assert.NotError(t, err)
					assert.NotError(t, worker(context.Background()))
					assert.Equal(t, 1, count)
				})
			})
		})
	})
}

func (Constructors) String() string { return "Constructors<>" }
