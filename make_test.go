package fun

import (
	"context"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fn"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/ft"
)

func TestHandlers(t *testing.T) {
	t.Parallel()
	const root ers.Error = ers.Error("root-error")
	t.Run("ForBackground", func(t *testing.T) {
		count := 0
		op := fnx.Operation(func(context.Context) {
			count++
		}).Lock()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		jobs := []fnx.Operation{}

		ft.CallTimes(128, func() { jobs = append(jobs, op) })

		err := SliceStream(jobs).Parallel(fnx.MAKE.OperationHandler(), fnx.WorkerGroupConfNumWorkers(4)).Run(ctx)
		assert.NotError(t, err)
		check.Equal(t, count, 128)
	})
	t.Run("WorkerPools", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			// most of these depend on the race detector not hitting errors
			t.Run("Basic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := fnx.Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; return nil })
					check.NotError(t, MAKE.RunAllWorkers(VariadicStream(worker, worker, worker, worker, worker)).Run(tctx))
					check.Equal(t, 5, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := fnx.Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++ })

					check.NotError(t, MAKE.RunAllOperations(VariadicStream(op, op, op, op, op)).Run(tctx))
					check.Equal(t, 5, count)
				})
			})
			t.Run("Panic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := fnx.Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; panic("oops") })
					check.NotPanic(t, func() {
						check.Error(t, MAKE.RunAllWorkers(VariadicStream(worker, worker, worker, worker, worker)).Run(tctx))
					})
					check.Equal(t, 1, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := fnx.Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++; panic("oops") })

					check.NotPanic(t, func() {
						check.Error(t, MAKE.RunAllOperations(VariadicStream(op, op, op, op, op)).Run(tctx))
					})

					check.Equal(t, 1, count)
				})
			})
		})
		t.Run("WorkerPool", func(t *testing.T) {
			const wpJobCount = 75
			const minDuration = 25 * time.Millisecond

			t.Run("Basic", func(t *testing.T) {
				counter := &atomic.Int64{}
				wfs := make([]fnx.Worker, wpJobCount)
				for i := 0; i < wpJobCount; i++ {
					wfs[i] = func(context.Context) error {
						counter.Add(1)
						time.Sleep(minDuration)
						counter.Add(1)
						return nil
					}
				}
				start := time.Now()
				assert.Equal(t, counter.Load(), 0)

				err := MAKE.WorkerPool(SliceStream(wfs)).Run(t.Context())
				dur := time.Since(start)
				if dur > 500*time.Millisecond || dur < minDuration || dur < wpJobCount*time.Millisecond {
					t.Error(dur)
				}
				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), 2*wpJobCount)
			})
			t.Run("Errors", func(t *testing.T) {
				const experr ers.Error = "expected error"

				counter := &atomic.Int64{}
				wfs := make([]fnx.Worker, wpJobCount)
				for i := 0; i < wpJobCount; i++ {
					wfs[i] = func(context.Context) error { counter.Add(1); time.Sleep(minDuration); return experr }
				}

				start := time.Now()
				assert.Equal(t, counter.Load(), 0)

				err := MAKE.WorkerPool(SliceStream(wfs)).Run(t.Context())
				dur := time.Since(start)
				if dur > 500*time.Millisecond || dur < minDuration || dur < wpJobCount*time.Millisecond {
					t.Error(dur)
				}

				assert.Error(t, err)
				errs := ers.Unwind(err)

				assert.Equal(t, len(errs), wpJobCount)
				assert.Equal(t, counter.Load(), wpJobCount)

				for _, e := range errs {
					assert.ErrorIs(t, e, experr)
				}
			})
		})
	})

	t.Run("Merge", func(t *testing.T) {
		wfs := make([]fnx.Operation, 100)
		count := &atomic.Int64{}
		for i := 0; i < 100; i++ {
			wfs[i] = func(context.Context) {
				startAt := time.Now()
				defer count.Add(1)

				time.Sleep(10 * time.Millisecond)
				t.Log(time.Since(startAt))
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		start := time.Now()
		t.Log("before exec:", count.Load())
		err := MAKE.OperationPool(SliceStream(wfs)).Run(ctx)
		t.Log("after exec:", count.Load())
		if err != nil {
			t.Error(err)
		}
		dur := time.Since(start)
		t.Log("duration was:", dur)
		if dur > 500*time.Millisecond {
			t.Error(dur, "is not > 150ms")
		}
		if int(dur) < (1000 / runtime.NumCPU()) {
			t.Error(t, dur, "<", 1000/runtime.NumCPU())
		}
	})
	t.Run("Wait", func(t *testing.T) {
		ops := make([]int, 100)
		for i := 0; i < len(ops); i++ {
			ops[i] = rand.Int()
		}
		seen := make(map[int]struct{})
		counter := 0
		var of fn.Handler[int] = func(in int) {
			seen[in] = struct{}{}
			counter++
		}

		wf := SliceStream(ops).ReadAll(fnx.FromHandler(of))

		if len(seen) != 0 || counter != 0 {
			t.Error("should be lazy execution", counter, seen)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := wf(ctx); err != nil {
			t.Error(err)
		}

		if len(seen) != 100 {
			t.Error(len(seen), seen)
		}
		if counter != 100 {
			t.Error(counter)
		}
	})
}

func (Constructors) String() string { return "Constructors<>" }
