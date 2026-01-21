package wpa

import (
	"context"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/testt"
)

func TestPool(t *testing.T) {
	t.Run("TaskJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsWorker", func(t *testing.T) {
			task := Task(func() error { return nil })
			assert.NotError(t, task.Job()(ctx))
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			task := Task(func() error { panic(expected) })
			err := task.Job()(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("PropagatesErrors", func(t *testing.T) {
			expected := ers.Error("test error")
			task := Task(func() error { return expected })
			err := task.Job()(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})
	t.Run("ThunkJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsWorker", func(t *testing.T) {
			called := &atomic.Bool{}
			thunk := Thunk(func() { called.Store(true) })
			err := thunk.Job()(ctx)
			assert.NotError(t, err)
			assert.True(t, called.Load())
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			thunk := Thunk(func() { panic(expected) })
			err := thunk.Job()(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("NoErrorOnSuccess", func(t *testing.T) {
			count := &atomic.Int64{}
			thunk := Thunk(func() { count.Add(1) })
			err := thunk.Job()(ctx)
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

			err := Run(irt.Slice(tasks)).Job()(t.Context())
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
			ctx := t.Context()

			panicThunkErr := TaskHandler(ctx, Thunk(func() { panic(panicErr) }))
			panicTaskErr := TaskHandler(ctx, Task(func() error { panic(panicErr) }))
			panicWorkerErr := TaskHandler(ctx, fnx.Worker(func(context.Context) error { panic(panicErr) }))
			panicOperationErr := TaskHandler(ctx, fnx.Operation(func(context.Context) { panic(panicErr) }))

			check.Error(t, panicThunkErr)
			check.ErrorIs(t, panicThunkErr, panicErr)

			check.Error(t, panicTaskErr)
			check.ErrorIs(t, panicTaskErr, panicErr)

			check.Error(t, panicWorkerErr)
			check.ErrorIs(t, panicWorkerErr, panicErr)

			check.Error(t, panicOperationErr)
			check.ErrorIs(t, panicOperationErr, panicErr)
		})
	})
}

func TestJobs(t *testing.T) {
	t.Run("ConvertsThunksToWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}
		thunks := []Thunk{
			func() { counter.Add(1) },
			func() { counter.Add(1) },
			func() { counter.Add(1) },
		}

		workers := irt.Collect(Jobs(irt.Slice(thunks)))
		check.Equal(t, len(workers), 3)

		for _, w := range workers {
			check.NotError(t, w(t.Context()))
		}
		check.Equal(t, counter.Load(), int64(3))
	})

	t.Run("ConvertsTasksToWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}
		tasks := []Task{
			func() error { counter.Add(1); return nil },
			func() error { counter.Add(1); return nil },
		}

		workers := irt.Collect(Jobs(irt.Slice(tasks)))
		check.Equal(t, len(workers), 2)

		for _, w := range workers {
			check.NotError(t, w(t.Context()))
		}
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("ConvertsWorkersToWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}
		inputWorkers := []fnx.Worker{
			func(context.Context) error { counter.Add(1); return nil },
			func(context.Context) error { counter.Add(1); return nil },
		}

		workers := irt.Collect(Jobs(irt.Slice(inputWorkers)))
		check.Equal(t, len(workers), 2)

		for _, w := range workers {
			check.NotError(t, w(t.Context()))
		}
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("ConvertsOperationsToWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}
		ops := []fnx.Operation{
			func(context.Context) { counter.Add(1) },
			func(context.Context) { counter.Add(1) },
		}

		workers := irt.Collect(Jobs(irt.Slice(ops)))
		check.Equal(t, len(workers), 2)

		for _, w := range workers {
			check.NotError(t, w(t.Context()))
		}
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		workers := irt.Collect(Jobs(irt.Slice([]Thunk{})))
		check.Equal(t, len(workers), 0)
	})
}

func TestTaskHandler(t *testing.T) {
	t.Run("RunsThunk", func(t *testing.T) {
		called := &atomic.Bool{}
		thunk := Thunk(func() { called.Store(true) })

		err := TaskHandler(t.Context(), thunk)
		check.NotError(t, err)
		check.True(t, called.Load())
	})

	t.Run("RunsTask", func(t *testing.T) {
		called := &atomic.Bool{}
		task := Task(func() error { called.Store(true); return nil })

		err := TaskHandler(t.Context(), task)
		check.NotError(t, err)
		check.True(t, called.Load())
	})

	t.Run("RunsWorker", func(t *testing.T) {
		called := &atomic.Bool{}
		worker := fnx.Worker(func(context.Context) error { called.Store(true); return nil })

		err := TaskHandler(t.Context(), worker)
		check.NotError(t, err)
		check.True(t, called.Load())
	})

	t.Run("RunsOperation", func(t *testing.T) {
		called := &atomic.Bool{}
		op := fnx.Operation(func(context.Context) { called.Store(true) })

		err := TaskHandler(t.Context(), op)
		check.NotError(t, err)
		check.True(t, called.Load())
	})

	t.Run("PropagatesErrors", func(t *testing.T) {
		expectedErr := ers.Error("test error")
		task := Task(func() error { return expectedErr })

		err := TaskHandler(t.Context(), task)
		check.Error(t, err)
		check.ErrorIs(t, err, expectedErr)
	})

	t.Run("RecoversPanics", func(t *testing.T) {
		panicErr := ers.Error("panic error")
		thunk := Thunk(func() { panic(panicErr) })

		err := TaskHandler(t.Context(), thunk)
		check.Error(t, err)
		check.ErrorIs(t, err, panicErr)
	})

	t.Run("RespectsContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		called := &atomic.Bool{}
		worker := fnx.Worker(func(ctx context.Context) error {
			called.Store(true)
			return ctx.Err()
		})

		err := TaskHandler(ctx, worker)
		check.Error(t, err)
		check.ErrorIs(t, err, context.Canceled)
		check.True(t, called.Load())
	})
}

func TestWithHandler(t *testing.T) {
	t.Run("BasicReaderConversion", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		workers := WithHandler(reader).For(seq)

		err := Run(workers).Run(t.Context())
		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(15)) // 1+2+3+4+5
	})

	t.Run("ReaderWithMultipleItems", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{10, 20, 30})
		workers := WithHandler(reader).For(seq)

		err := Run(workers).Run(t.Context())
		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(60))
	})

	t.Run("ReaderWithErrors", func(t *testing.T) {
		expectedErr := ers.Error("processing error")
		reader := Reader[int](func(value int) error {
			if value == 2 {
				return expectedErr
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3})
		workers := WithHandler(reader).For(seq)

		err := Run(workers).Run(t.Context())
		check.Error(t, err)
		check.ErrorIs(t, err, expectedErr)
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{})
		workers := WithHandler(reader).For(seq)

		err := Run(workers).Run(t.Context())
		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("WithRunAll", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.Error("even number")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		workers := WithHandler(reader).For(seq)

		err := RunAll(workers).Run(t.Context())
		check.Error(t, err)
		check.Equal(t, counter.Load(), int64(5)) // All items processed despite errors

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 2) // Two even numbers: 2 and 4
	})

	t.Run("WithRunWithPool", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8})
		workers := WithHandler(reader).For(seq)

		err := RunWithPool(workers, WorkerGroupConfDefaults()).Run(t.Context())
		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(8))
	})

	t.Run("StringReader", func(t *testing.T) {
		var result []string
		reader := Reader[string](func(s string) error {
			result = append(result, s)
			return nil
		})

		seq := irt.Slice([]string{"a", "b", "c"})
		workers := WithHandler(reader).For(seq)

		err := Run(workers).Run(t.Context())
		check.NotError(t, err)
		check.Equal(t, len(result), 3)
	})
}

func TestWithHandlerRun(t *testing.T) {
	t.Run("BasicExecution", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(15))
	})

	t.Run("StopsOnFirstError", func(t *testing.T) {
		counter := &atomic.Int64{}
		expectedErr := ers.Error("processing error")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				return expectedErr
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.Error(t, err)
		check.ErrorIs(t, err, expectedErr)
		check.Equal(t, counter.Load(), int64(3))
	})

	t.Run("SkipsOnErrCurrentOpSkip", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.ErrCurrentOpSkip
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(5)) // All items processed
	})

	t.Run("TerminatesOnEOF", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				return io.EOF
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(3))
	})

	t.Run("TerminatesOnErrCurrentOpAbort", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 4 {
				return ers.ErrCurrentOpAbort
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(4))
	})

	t.Run("RespectsContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				cancel()
				return context.Canceled
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).Run().For(seq).Run(ctx)

		check.Error(t, err)
		check.ErrorIs(t, err, context.Canceled)
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{})
		err := WithHandler(reader).Run().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(0))
	})
}

func TestWithHandlerRunAll(t *testing.T) {
	t.Run("ProcessesAllItemsDespiteErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.Error("even number error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).RunAll().For(seq).Run(t.Context())

		check.Error(t, err)
		check.Equal(t, counter.Load(), int64(5)) // All items processed

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 2) // Two errors for 2 and 4
	})

	t.Run("ReturnsNilWhenNoErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3})
		err := WithHandler(reader).RunAll().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("CollectsMultipleErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).RunAll().For(seq).Run(t.Context())

		check.Error(t, err)
		check.Equal(t, counter.Load(), int64(5))

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 5)
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{})
		err := WithHandler(reader).RunAll().For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("MixedSuccessAndFailure", func(t *testing.T) {
		var results []int
		var mu sync.Mutex

		reader := Reader[int](func(value int) error {
			mu.Lock()
			results = append(results, value)
			mu.Unlock()

			if value == 3 || value == 5 {
				return ers.Error("specific error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		err := WithHandler(reader).RunAll().For(seq).Run(t.Context())

		check.Error(t, err)
		check.Equal(t, len(results), 6)

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 2)
	})
}

func TestWithHandlerRunWithPool(t *testing.T) {
	t.Run("ConcurrentExecution", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8})
		start := time.Now()

		err := WithHandler(reader).RunWithPool(WorkerGroupConfDefaults()).For(seq).Run(t.Context())

		dur := time.Since(start)

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(8))

		// Should complete faster than serial execution (80ms)
		// but slower than single item (10ms)
		if dur > 200*time.Millisecond {
			t.Errorf("took too long: %v", dur)
		}
		if dur < 10*time.Millisecond {
			t.Errorf("completed too quickly, might not be concurrent: %v", dur)
		}
	})

	t.Run("CollectsAllErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		err := WithHandler(reader).RunWithPool(WorkerGroupConfDefaults()).For(seq).Run(t.Context())

		check.Error(t, err)
		check.Equal(t, counter.Load(), int64(5))

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 5)
	})

	t.Run("WithCustomNumWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4})
		err := WithHandler(reader).RunWithPool(WorkerGroupConfNumWorkers(2)).For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(4))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{})
		err := WithHandler(reader).RunWithPool(WorkerGroupConfDefaults()).For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("SomeSuccessSomeFailure", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			if value%2 == 0 {
				return ers.Error("even error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		err := WithHandler(reader).RunWithPool(WorkerGroupConfDefaults()).For(seq).Run(t.Context())

		check.Error(t, err)
		check.Equal(t, counter.Load(), int64(6))

		errs := ers.Unwind(err)
		check.Equal(t, len(errs), 3) // 2, 4, 6
	})

	t.Run("StringsWithPool", func(t *testing.T) {
		var results sync.Map
		counter := &atomic.Int64{}

		reader := Reader[string](func(s string) error {
			counter.Add(1)
			results.Store(s, true)
			time.Sleep(5 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]string{"a", "b", "c", "d", "e"})
		err := WithHandler(reader).RunWithPool(WorkerGroupConfDefaults()).For(seq).Run(t.Context())

		check.NotError(t, err)
		check.Equal(t, counter.Load(), int64(5))

		for _, s := range []string{"a", "b", "c", "d", "e"} {
			_, ok := results.Load(s)
			check.True(t, ok)
		}
	})
}

func TestWithHandlerForEachPull(t *testing.T) {
	t.Run("BasicExecution", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for err := range errorSeq {
			t.Errorf("unexpected error: %v", err)
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(15)) // 1+2+3+4+5
	})

	t.Run("StopsOnFirstError", func(t *testing.T) {
		counter := &atomic.Int64{}
		expectedErr := ers.Error("processing error")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				return expectedErr
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		var firstError error
		for err := range errorSeq {
			if firstError == nil {
				firstError = err
			}
			errorCount++
		}

		check.Equal(t, errorCount, 1)
		check.Error(t, firstError)
		check.ErrorIs(t, firstError, expectedErr)
		check.Equal(t, counter.Load(), int64(3)) // Stops after error
	})

	t.Run("SkipsOnErrCurrentOpSkip", func(t *testing.T) {
		counter := &atomic.Int64{}
		processed := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.ErrCurrentOpSkip
			}
			processed.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)              // ErrCurrentOpSkip doesn't yield errors
		check.Equal(t, counter.Load(), int64(5))   // All items processed
		check.Equal(t, processed.Load(), int64(9)) // 1+3+5
	})

	t.Run("TerminatesOnEOF", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				return io.EOF
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)            // io.EOF is terminating, doesn't yield error
		check.Equal(t, counter.Load(), int64(3)) // Stopped at EOF
	})

	t.Run("TerminatesOnErrCurrentOpAbort", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 4 {
				return ers.ErrCurrentOpAbort
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)            // ErrCurrentOpAbort is terminating
		check.Equal(t, counter.Load(), int64(4)) // Stopped at abort
	})

	t.Run("TerminatesOnErrContainerClosed", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				return ers.ErrContainerClosed
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("YieldsOnRecoveredPanic", func(t *testing.T) {
		counter := &atomic.Int64{}
		panicErr := ers.Error("panic error")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				panic(panicErr)
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		var firstError error
		for err := range errorSeq {
			if firstError == nil {
				firstError = err
			}
			errorCount++
		}

		check.Equal(t, errorCount, 1)
		check.Error(t, firstError)
		check.ErrorIs(t, firstError, panicErr)
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("RespectsContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				cancel()
				return context.Canceled
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(ctx)

		errorCount := 0
		var firstError error
		for err := range errorSeq {
			if firstError == nil {
				firstError = err
			}
			errorCount++
		}

		check.Equal(t, errorCount, 1)
		check.Error(t, firstError)
		check.ErrorIs(t, firstError, context.Canceled)
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("MultipleErrors", func(t *testing.T) {
		counter := &atomic.Int64{}
		expectedErr := ers.Error("test error")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				return expectedErr
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		// Should only yield one error and stop
		check.Equal(t, errorCount, 1)
		check.Equal(t, counter.Load(), int64(2))
	})

	t.Run("StringsWithErrors", func(t *testing.T) {
		var processed []string
		expectedErr := ers.Error("bad string")

		reader := Reader[string](func(s string) error {
			if s == "bad" {
				return expectedErr
			}
			processed = append(processed, s)
			return nil
		})

		seq := irt.Slice([]string{"a", "b", "bad", "c", "d"})
		errorSeq := WithHandler(reader).ForEach(seq).Pull(t.Context())

		errorCount := 0
		for err := range errorSeq {
			errorCount++
			check.ErrorIs(t, err, expectedErr)
		}

		check.Equal(t, errorCount, 1)
		check.Equal(t, len(processed), 2) // "a", "b"
	})
}

func TestWithHandlerForEachPullAll(t *testing.T) {
	t.Run("ProcessesAllItemsDespiteErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.Error("even number error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 2)            // Two errors for 2 and 4
		check.Equal(t, counter.Load(), int64(5)) // All items processed
	})

	t.Run("ReturnsNoErrorsWhenAllSucceed", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(int64(value))
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("YieldsAllErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 5) // All 5 items yield errors
		check.Equal(t, counter.Load(), int64(5))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("MixedSuccessAndFailure", func(t *testing.T) {
		var processed []int

		reader := Reader[int](func(value int) error {
			processed = append(processed, value)
			if value == 3 || value == 5 {
				return ers.Error("specific error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 2)     // Errors for 3 and 5
		check.Equal(t, len(processed), 6) // All items processed
	})

	t.Run("IncludesErrCurrentOpSkip", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return ers.ErrCurrentOpSkip
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errorCount := 0
		for err := range errorSeq {
			errorCount++
			check.ErrorIs(t, err, ers.ErrCurrentOpSkip)
		}

		// PullAll yields ALL errors, including ErrCurrentOpSkip
		check.Equal(t, errorCount, 2) // 2 and 4
		check.Equal(t, counter.Load(), int64(5))
	})

	t.Run("IncludesTerminatingErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				return io.EOF
			}
			if value == 5 {
				return ers.ErrCurrentOpAbort
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errors := irt.Collect(errorSeq)

		// PullAll processes all items and yields all errors
		check.Equal(t, len(errors), 2)
		check.ErrorIs(t, errors[0], io.EOF)
		check.ErrorIs(t, errors[1], ers.ErrCurrentOpAbort)
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("YieldsPanicErrors", func(t *testing.T) {
		counter := &atomic.Int64{}
		panicErr1 := ers.Error("panic 1")
		panicErr2 := ers.Error("panic 2")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 2 {
				panic(panicErr1)
			}
			if value == 4 {
				panic(panicErr2)
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		errors := irt.Collect(errorSeq)

		check.Equal(t, len(errors), 2)
		check.ErrorIs(t, errors[0], panicErr1)
		check.ErrorIs(t, errors[1], panicErr2)
		check.Equal(t, counter.Load(), int64(5))
	})

	t.Run("ContextCancellationYieldsError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value == 3 {
				cancel()
				return context.Canceled
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(ctx)

		errors := irt.Collect(errorSeq)

		// Should yield the context.Canceled error
		check.Equal(t, len(errors), 1)
		check.ErrorIs(t, errors[0], context.Canceled)
		check.Equal(t, counter.Load(), int64(5)) // Continues processing all items
	})

	t.Run("EarlyBreak", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		// Consumer breaks after first error
		errorCount := 0
		for range errorSeq {
			errorCount++
			if errorCount == 1 {
				break
			}
		}

		check.Equal(t, errorCount, 1)
		// Note: depending on buffering, some items might still be processed
		check.True(t, counter.Load() >= 1)
	})

	t.Run("EarlyReturnAbortsProcessing", func(t *testing.T) {
		started := &atomic.Int64{}
		completed := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			started.Add(1)
			// Add delay to ensure we can detect early termination
			time.Sleep(10 * time.Millisecond)
			completed.Add(1)
			return ers.Error("error")
		})

		// Create a larger sequence to ensure there's work in progress
		items := make([]int, 50)
		for i := range items {
			items[i] = i
		}
		seq := irt.Slice(items)

		errorSeq := WithHandler(reader).ForEach(seq).PullAll(t.Context())

		// Consumer breaks after receiving just 3 errors
		errorCount := 0
		for range errorSeq {
			errorCount++
			if errorCount == 3 {
				break
			}
		}

		check.Equal(t, errorCount, 3)

		// Give a moment for any in-flight operations to potentially complete
		time.Sleep(20 * time.Millisecond)

		// Verify that processing was aborted - we should have far fewer than 50 items processed
		startedCount := started.Load()
		completedCount := completed.Load()

		t.Logf("Started: %d, Completed: %d out of 50 items", startedCount, completedCount)

		// We should have significantly fewer items processed than the total
		// The exact number depends on timing, but should be much less than 50
		check.True(t, completedCount < 50)
		check.True(t, completedCount <= startedCount) // Sanity check

		// We should have at least processed the 3 that yielded errors
		check.True(t, completedCount >= 3)
	})
}

func TestWithHandlerForEachPullWithPool(t *testing.T) {
	t.Run("ConcurrentExecution", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8})
		start := time.Now()

		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		dur := time.Since(start)

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(8))

		// Should complete faster than serial (80ms) but slower than instant
		if dur > 200*time.Millisecond {
			t.Errorf("took too long: %v", dur)
		}
		if dur < 10*time.Millisecond {
			t.Errorf("completed too quickly, might not be concurrent: %v", dur)
		}
	})

	t.Run("YieldsAllErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			return ers.Error("error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 5)
		check.Equal(t, counter.Load(), int64(5))
	})

	t.Run("WithCustomNumWorkers", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(
			t.Context(),
			WorkerGroupConfNumWorkers(2),
		)

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(4))
	})

	t.Run("EmptySequence", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(0))
	})

	t.Run("SomeSuccessSomeFailure", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			if value%2 == 0 {
				return ers.Error("even error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 3) // 2, 4, 6
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("FiltersNilErrors", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			if value%2 == 0 {
				return nil
			}
			return ers.Error("odd error")
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errors := make([]error, 0, 3)
		for err := range errorSeq {
			errors = append(errors, err)
		}

		// Should yield errors only for odd numbers (1, 3, 5)
		check.Equal(t, len(errors), 3)
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("ContinueOnErrorBehavior", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			if value%2 == 0 {
				return ers.Error("even error")
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6})

		// With continue on error (default), all items should be processed
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(
			t.Context(),
			WorkerGroupConfDefaults(),
		)

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		// Should yield 3 errors (2, 4, 6)
		check.Equal(t, errorCount, 3)
		check.Equal(t, counter.Load(), int64(6))
	})

	t.Run("StringsWithPool", func(t *testing.T) {
		var results sync.Map
		counter := &atomic.Int64{}

		reader := Reader[string](func(s string) error {
			counter.Add(1)
			results.Store(s, true)
			time.Sleep(5 * time.Millisecond)
			return nil
		})

		seq := irt.Slice([]string{"a", "b", "c", "d", "e"})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(5))

		for _, s := range []string{"a", "b", "c", "d", "e"} {
			_, ok := results.Load(s)
			check.True(t, ok)
		}
	})

	t.Run("PanicRecovery", func(t *testing.T) {
		counter := &atomic.Int64{}
		panicErr := ers.Error("panic error")

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(5 * time.Millisecond)
			if value == 3 {
				panic(panicErr)
			}
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5})
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(t.Context(), WorkerGroupConfDefaults())

		errors := make([]error, 0, 1)
		for err := range errorSeq {
			errors = append(errors, err)
		}

		// Should have one panic error
		check.Equal(t, len(errors), 1)
		check.ErrorIs(t, errors[0], panicErr)
		check.Equal(t, counter.Load(), int64(5))
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		counter := &atomic.Int64{}
		started := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			started.Add(1)
			time.Sleep(10 * time.Millisecond)
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})

		// Cancel context after short delay
		go func() {
			time.Sleep(time.Millisecond)
			cancel()
		}()

		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(ctx, WorkerGroupConfDefaults())

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		// With 50ms per task and cancellation at 25ms, some tasks should not complete
		// (Note: some may start but not finish due to the sleep duration)
		testt.Log(t, counter.Load(), errorCount)
		check.True(t, counter.Load() < 12)
	})

	t.Run("InvalidOptions", func(t *testing.T) {
		counter := &atomic.Int64{}
		reader := Reader[int](func(value int) error {
			counter.Add(1)
			return nil
		})

		seq := irt.Slice([]int{1, 2, 3})

		// Test with nil error collector
		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(
			t.Context(),
			WorkerGroupConfWithErrorCollector(nil),
		)

		errors := make([]error, 0, 1)
		for err := range errorSeq {
			errors = append(errors, err)
		}

		// Should yield the validation error
		check.Equal(t, len(errors), 1)
		check.ErrorIs(t, errors[0], ers.ErrInvalidInput)
		check.Equal(t, counter.Load(), int64(0)) // No processing should occur
	})

	t.Run("LargeSequenceConcurrent", func(t *testing.T) {
		counter := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			counter.Add(1)
			time.Sleep(2 * time.Millisecond)
			return nil
		})

		// Create large sequence
		items := make([]int, 100)
		for i := range items {
			items[i] = i
		}

		seq := irt.Slice(items)
		start := time.Now()

		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(
			t.Context(),
			WorkerGroupConfNumWorkers(10),
		)

		errorCount := 0
		for range errorSeq {
			errorCount++
		}

		dur := time.Since(start)

		check.Equal(t, errorCount, 0)
		check.Equal(t, counter.Load(), int64(100))

		// With 10 workers and 2ms per item, should complete much faster than 200ms (serial)
		if dur > 100*time.Millisecond {
			t.Errorf("took too long: %v", dur)
		}
	})

	t.Run("EarlyReturnAbortsProcessing", func(t *testing.T) {
		started := &atomic.Int64{}
		completed := &atomic.Int64{}

		reader := Reader[int](func(value int) error {
			started.Add(1)
			// Add delay to ensure we can detect early termination
			time.Sleep(20 * time.Millisecond)
			completed.Add(1)
			return ers.Error("error")
		})

		// Create a large sequence
		items := make([]int, 100)
		for i := range items {
			items[i] = i
		}
		seq := irt.Slice(items)

		errorSeq := WithHandler(reader).ForEach(seq).PullWithPool(
			t.Context(),
			WorkerGroupConfNumWorkers(4),
		)

		// Consumer breaks after receiving just 5 errors
		errorCount := 0
		for range errorSeq {
			errorCount++
			if errorCount == 5 {
				break
			}
		}

		check.Equal(t, errorCount, 5)

		// Give a moment for any in-flight operations to potentially complete
		time.Sleep(50 * time.Millisecond)

		// Verify that processing was aborted
		startedCount := started.Load()
		completedCount := completed.Load()

		t.Logf("Started: %d, Completed: %d out of 100 items", startedCount, completedCount)

		// With 4 workers, we might start a few more tasks before they all receive the cancellation
		// But we should have significantly fewer than 100 items completed
		check.True(t, completedCount < 100)
		check.True(t, completedCount <= startedCount) // Sanity check

		// We should have at least completed the 5 that yielded errors
		check.True(t, completedCount >= 5)

		// Due to concurrent execution, we might have started more than completed
		// but should still be far less than the total
		check.True(t, startedCount < 50) // Should be much less than 100
	})
}
