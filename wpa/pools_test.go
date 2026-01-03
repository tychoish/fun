package wpa

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
)

func TestPool(t *testing.T) {
	t.Run("TaskJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsWorker", func(t *testing.T) {
			task := Task(func() error { return nil })
			job := task.Job()
			assert.NotError(t, job(ctx))
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			task := Task(func() error { panic(expected) })
			job := task.Job()
			err := job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("PropagatesErrors", func(t *testing.T) {
			expected := ers.Error("test error")
			task := Task(func() error { return expected })
			job := task.Job()
			err := job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})
	t.Run("ThunkJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsWorker", func(t *testing.T) {
			called := &atomic.Bool{}
			thunk := Thunk(func() { called.Store(true) })
			job := thunk.Job()
			assert.NotError(t, job(ctx))
			assert.True(t, called.Load())
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			thunk := Thunk(func() { panic(expected) })
			job := thunk.Job()
			err := job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("NoErrorOnSuccess", func(t *testing.T) {
			count := &atomic.Int64{}
			thunk := Thunk(func() { count.Add(1) })
			job := thunk.Job()
			err := job(ctx)
			assert.NotError(t, err)
			assert.Equal(t, count.Load(), int64(1))
		})
	})
	t.Run("Smoke", func(t *testing.T) {
		var ct int
		err := RunAll(irt.GenerateN(64, func() fnx.Worker { return fnx.MakeWorker(func() error { ct++; return nil }) })).Run(t.Context())
		if err != nil {
			t.Error(err)
		}
		if ct != 64 {
			t.Error(ct)
		}
	})

	t.Run("WorkerPools", func(t *testing.T) {
		t.Run("Serial", func(t *testing.T) {
			// most of these depend on the race detector not hitting errors
			t.Run("Basic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := fnx.Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; return nil })
					check.NotError(t, Run(irt.Args(worker, worker, worker, worker, worker)).Run(tctx))
					check.Equal(t, 5, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := fnx.Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++ })

					check.NotError(t, Run(irt.Args(op, op, op, op, op)).Run(tctx))
					check.Equal(t, 5, count)
				})
			})
			t.Run("Panic", func(t *testing.T) {
				t.Run("Workers", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					worker := fnx.Worker(func(ctx context.Context) error { check.True(t, ctx == tctx); count++; panic("oops") })
					check.NotPanic(t, func() {
						check.Error(t, Run(irt.Args(worker, worker, worker, worker, worker)).Run(tctx))
					})
					check.Equal(t, 1, count)
				})

				t.Run("Operations", func(t *testing.T) {
					count := 0
					tctx := t.Context()
					op := fnx.Operation(func(ctx context.Context) { check.True(t, ctx == tctx); count++; panic("oops") })

					check.NotPanic(t, func() {
						check.Error(t, Run(irt.Args(op, op, op, op, op)).Run(tctx))
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

				err := RunWithPool(irt.Slice(wfs), WorkerGroupConfDefaults()).Run(t.Context())
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

				err := RunWithPool(irt.Slice(wfs), WorkerGroupConfDefaults()).Run(t.Context())
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
		err := RunWithPool(irt.Slice(wfs), WorkerGroupConfDefaults()).Run(ctx)
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

	t.Run("InvalidOptions", func(t *testing.T) {
		t.Run("NilErrorCollector", func(t *testing.T) {
			executed := &atomic.Int64{}
			wfs := []fnx.Worker{
				func(context.Context) error { executed.Add(1); return nil },
				func(context.Context) error { executed.Add(1); return nil },
			}

			err := RunWithPool(
				irt.Slice(wfs),
				WorkerGroupConfWithErrorCollector(nil),
			).Run(t.Context())

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
			assert.Equal(t, executed.Load(), int64(0))
		})

		t.Run("ExcludeRecoveredPanic", func(t *testing.T) {
			executed := &atomic.Int64{}
			wfs := []fnx.Worker{
				func(context.Context) error { executed.Add(1); return nil },
				func(context.Context) error { executed.Add(1); return nil },
			}

			err := RunWithPool(
				irt.Slice(wfs),
				WorkerGroupConfAddExcludeErrors(ers.ErrRecoveredPanic),
			).Run(t.Context())

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
			assert.Equal(t, executed.Load(), int64(0))
		})

		t.Run("MultipleInvalidOptions", func(t *testing.T) {
			executed := &atomic.Int64{}
			wfs := []fnx.Worker{
				func(context.Context) error { executed.Add(1); return nil },
				func(context.Context) error { executed.Add(1); return nil },
			}

			err := RunWithPool(
				irt.Slice(wfs),
				WorkerGroupConfWithErrorCollector(nil),
				WorkerGroupConfAddExcludeErrors(ers.ErrRecoveredPanic),
			).Run(t.Context())

			assert.Error(t, err)
			assert.ErrorIs(t, err, ers.ErrInvalidInput)
			assert.Equal(t, executed.Load(), int64(0))
		})
	})

	t.Run("Run", func(t *testing.T) {
		t.Run("ContextCancellation", func(t *testing.T) {
			t.Run("StopsProcessingAndReturnsError", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); cancel(); return context.Canceled },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(ctx)

				assert.Error(t, err)
				assert.ErrorIs(t, err, context.Canceled)
				assert.Equal(t, counter.Load(), int64(2))
			})

			t.Run("DeadlineExceeded", func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()

				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { time.Sleep(5 * time.Millisecond); return nil },
					func(context.Context) error { counter.Add(1); return context.DeadlineExceeded },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(ctx)

				assert.Error(t, err)
				assert.ErrorIs(t, err, context.DeadlineExceeded)
			})
		})

		t.Run("TerminatingErrors", func(t *testing.T) {
			t.Run("EOF", func(t *testing.T) {
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return io.EOF },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), int64(3))
			})

			t.Run("ErrContainerClosed", func(t *testing.T) {
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return ers.ErrContainerClosed },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), int64(2))
			})

			t.Run("ErrCurrentOpAbort", func(t *testing.T) {
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return ers.ErrCurrentOpAbort },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), int64(4))
			})

			t.Run("AllTerminatingErrors", func(t *testing.T) {
				tests := []struct {
					name string
					err  error
				}{
					{name: "io.EOF", err: io.EOF},
					{name: "ErrContainerClosed", err: ers.ErrContainerClosed},
					{name: "ErrCurrentOpAbort", err: ers.ErrCurrentOpAbort},
				}

				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						counter := &atomic.Int64{}

						jobs := []fnx.Worker{
							func(context.Context) error { counter.Add(1); return nil },
							func(context.Context) error { counter.Add(1); return tt.err },
							func(context.Context) error { counter.Add(1); return nil },
						}

						err := Run(irt.Slice(jobs)).Run(t.Context())

						assert.NotError(t, err)
						assert.Equal(t, counter.Load(), int64(2))
					})
				}
			})
		})

		t.Run("ErrCurrentOpSkip", func(t *testing.T) {
			t.Run("ContinuesProcessing", func(t *testing.T) {
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return ers.ErrCurrentOpSkip },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return ers.ErrCurrentOpSkip },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), int64(5))
			})
		})

		t.Run("RegularError", func(t *testing.T) {
			t.Run("StopsProcessingAndReturnsError", func(t *testing.T) {
				counter := &atomic.Int64{}
				expectedErr := ers.Error("test error")

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return expectedErr },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.Error(t, err)
				assert.ErrorIs(t, err, expectedErr)
				assert.Equal(t, counter.Load(), int64(3))
			})
		})

		t.Run("MixedScenarios", func(t *testing.T) {
			t.Run("SkipThenTerminate", func(t *testing.T) {
				counter := &atomic.Int64{}

				jobs := []fnx.Worker{
					func(context.Context) error { counter.Add(1); return nil },
					func(context.Context) error { counter.Add(1); return ers.ErrCurrentOpSkip },
					func(context.Context) error { counter.Add(1); return ers.ErrCurrentOpSkip },
					func(context.Context) error { counter.Add(1); return io.EOF },
					func(context.Context) error { counter.Add(1); return nil },
				}

				err := Run(irt.Slice(jobs)).Run(t.Context())

				assert.NotError(t, err)
				assert.Equal(t, counter.Load(), int64(4))
			})
		})
	})

	t.Run("MixedJobTypesIntegration", func(t *testing.T) {
		t.Run("RunWithThunks", func(t *testing.T) {
			counter := &atomic.Int64{}

			thunks := []Thunk{
				func() { counter.Add(1) },
				func() { counter.Add(1) },
				func() { counter.Add(1) },
			}

			err := Run(irt.Slice(thunks)).Run(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, counter.Load(), int64(3))
		})

		t.Run("RunWithTasks", func(t *testing.T) {
			counter := &atomic.Int64{}

			tasks := []Task{
				func() error { counter.Add(1); return nil },
				func() error { counter.Add(1); return nil },
				func() error { counter.Add(1); return nil },
			}

			err := Run(irt.Slice(tasks)).Run(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, counter.Load(), int64(3))
		})

		t.Run("RunAllWithWorkers", func(t *testing.T) {
			counter := &atomic.Int64{}

			workers := []fnx.Worker{
				func(context.Context) error { counter.Add(1); return nil },
				func(context.Context) error { counter.Add(1); return nil },
				func(context.Context) error { counter.Add(1); return nil },
			}

			err := RunAll(irt.Slice(workers)).Run(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, counter.Load(), int64(3))
		})

		t.Run("RunAllWithOperations", func(t *testing.T) {
			counter := &atomic.Int64{}

			operations := []fnx.Operation{
				func(context.Context) { counter.Add(1) },
				func(context.Context) { counter.Add(1) },
				func(context.Context) { counter.Add(1) },
			}

			err := RunAll(irt.Slice(operations)).Run(t.Context())
			assert.NotError(t, err)
			assert.Equal(t, counter.Load(), int64(3))
		})

		t.Run("RunWithPoolMixedTypes", func(t *testing.T) {
			counter := &atomic.Int64{}

			thunks := []Thunk{
				func() { counter.Add(1); time.Sleep(5 * time.Millisecond) },
				func() { counter.Add(1); time.Sleep(5 * time.Millisecond) },
			}

			tasks := []Task{
				func() error { counter.Add(1); time.Sleep(5 * time.Millisecond); return nil },
				func() error { counter.Add(1); time.Sleep(5 * time.Millisecond); return nil },
			}

			workers := []fnx.Worker{
				func(context.Context) error { counter.Add(1); time.Sleep(5 * time.Millisecond); return nil },
				func(context.Context) error { counter.Add(1); time.Sleep(5 * time.Millisecond); return nil },
			}

			operations := []fnx.Operation{
				func(context.Context) { counter.Add(1); time.Sleep(5 * time.Millisecond) },
				func(context.Context) { counter.Add(1); time.Sleep(5 * time.Millisecond) },
			}

			err := RunWithPool(irt.Slice(thunks), WorkerGroupConfDefaults()).Run(t.Context())
			assert.NotError(t, err)

			err = RunWithPool(irt.Slice(tasks), WorkerGroupConfDefaults()).Run(t.Context())
			assert.NotError(t, err)

			err = RunWithPool(irt.Slice(workers), WorkerGroupConfDefaults()).Run(t.Context())
			assert.NotError(t, err)

			err = RunWithPool(irt.Slice(operations), WorkerGroupConfDefaults()).Run(t.Context())
			assert.NotError(t, err)

			assert.Equal(t, counter.Load(), int64(8))
		})

		t.Run("TasksWithErrors", func(t *testing.T) {
			counter := &atomic.Int64{}
			expectedErr := ers.Error("test error")

			tasks := []Task{
				func() error { counter.Add(1); return nil },
				func() error { counter.Add(1); return expectedErr },
				func() error { counter.Add(1); return nil },
			}

			err := Run(irt.Slice(tasks)).Run(t.Context())
			assert.Error(t, err)
			assert.ErrorIs(t, err, expectedErr)
			assert.Equal(t, counter.Load(), int64(2))
		})

		t.Run("ThunksWithPanics", func(t *testing.T) {
			counter := &atomic.Int64{}
			expectedErr := ers.Error("panic in thunk")

			thunks := []Thunk{
				func() { counter.Add(1) },
				func() { counter.Add(1); panic(expectedErr) },
				func() { counter.Add(1) },
			}

			err := Run(irt.Slice(thunks)).Run(t.Context())
			assert.Error(t, err)
			assert.ErrorIs(t, err, expectedErr)
			assert.Equal(t, counter.Load(), int64(2))
		})

		t.Run("AllTypesPanicRecovery", func(t *testing.T) {
			panicErr := ers.Error("panic error")

			panicThunk := Thunk(func() { panic(panicErr) })
			panicTask := Task(func() error { panic(panicErr) })
			panicWorker := fnx.Worker(func(context.Context) error { panic(panicErr) })
			panicOperation := fnx.Operation(func(context.Context) { panic(panicErr) })

			check.Error(t, panicThunk.Job()(t.Context()))
			check.ErrorIs(t, panicThunk.Job()(t.Context()), panicErr)

			check.Error(t, panicTask.Job()(t.Context()))
			check.ErrorIs(t, panicTask.Job()(t.Context()), panicErr)

			check.Error(t, panicWorker.Job()(t.Context()))
			check.ErrorIs(t, panicWorker.Job()(t.Context()), panicErr)

			check.Error(t, panicOperation.Job()(t.Context()))
			check.ErrorIs(t, panicOperation.Job()(t.Context()), panicErr)
		})
	})
}
