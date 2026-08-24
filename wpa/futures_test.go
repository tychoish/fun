package wpa

import (
	"context"
	"io"
	"iter"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/irt"
)

func TestFutures(t *testing.T) {
	t.Run("ProducerJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsValue", func(t *testing.T) {
			p := Producer[int](func() int { return 42 })
			v, err := p.Job(ctx)
			assert.NotError(t, err)
			assert.Equal(t, v, 42)
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			p := Producer[int](func() int { panic(expected) })
			_, err := p.Job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})

	t.Run("ResultJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsValue", func(t *testing.T) {
			r := Result[int](func() (int, error) { return 7, nil })
			v, err := r.Job(ctx)
			assert.NotError(t, err)
			assert.Equal(t, v, 7)
		})
		t.Run("PropagatesErrors", func(t *testing.T) {
			expected := ers.Error("test error")
			r := Result[int](func() (int, error) { return 0, expected })
			_, err := r.Job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			r := Result[int](func() (int, error) { panic(expected) })
			_, err := r.Job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})

	t.Run("FutureJob", func(t *testing.T) {
		ctx := context.Background()
		t.Run("ReturnsValue", func(t *testing.T) {
			f := fnx.Future[int](func(context.Context) (int, error) { return 9, nil })
			v, err := f.Job(ctx)
			assert.NotError(t, err)
			assert.Equal(t, v, 9)
		})
		t.Run("RecoversPanics", func(t *testing.T) {
			expected := ers.Error("panic error")
			f := fnx.Future[int](func(context.Context) (int, error) { panic(expected) })
			_, err := f.Job(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, expected)
		})
	})

	t.Run("ResolveFutures", func(t *testing.T) {
		t.Run("EmptySequence", func(t *testing.T) {
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice([]Producer[int]{})))
			assert.Equal(t, len(kvs), 0)
		})

		t.Run("AllSuccess", func(t *testing.T) {
			producers := []Producer[int]{
				func() int { return 1 },
				func() int { return 2 },
			}
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice(producers)))
			assert.Equal(t, len(kvs), 2)
			for _, kv := range kvs {
				check.NotError(t, kv.Value)
			}
		})

		t.Run("StopsOnFirstError", func(t *testing.T) {
			expected := ers.Error("test error")
			counter := &atomic.Int64{}
			results := []Result[int]{
				func() (int, error) { counter.Add(1); return 1, nil },
				func() (int, error) { counter.Add(1); return 0, expected },
				func() (int, error) { counter.Add(1); return 3, nil },
			}
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice(results)))
			assert.Equal(t, len(kvs), 2)
			check.NotError(t, kvs[0].Value)
			check.ErrorIs(t, kvs[1].Value, expected)
			assert.Equal(t, counter.Load(), int64(2))
		})

		t.Run("TerminatingErrorStopsWithoutYield", func(t *testing.T) {
			counter := &atomic.Int64{}
			results := []Result[int]{
				func() (int, error) { counter.Add(1); return 1, nil },
				func() (int, error) { counter.Add(1); return 0, io.EOF },
				func() (int, error) { counter.Add(1); return 3, nil },
			}
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice(results)))
			assert.Equal(t, len(kvs), 1)
			assert.Equal(t, counter.Load(), int64(2))
		})

		t.Run("SkipYieldsNothing", func(t *testing.T) {
			results := []Result[int]{
				func() (int, error) { return 1, nil },
				func() (int, error) { return 0, ers.ErrCurrentOpSkip },
				func() (int, error) { return 3, nil },
			}
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice(results)))
			assert.Equal(t, len(kvs), 2)
			check.Equal(t, kvs[0].Key, 1)
			check.Equal(t, kvs[1].Key, 3)
		})

		t.Run("PanicRecovery", func(t *testing.T) {
			expected := ers.Error("panic error")
			producers := []Producer[int]{
				func() int { return 1 },
				func() int { panic(expected) },
			}
			kvs := collect2(ResolveFutures[int](t.Context(), irt.Slice(producers)))
			assert.Equal(t, len(kvs), 2)
			check.NotError(t, kvs[0].Value)
			check.ErrorIs(t, kvs[1].Value, expected)
		})

		t.Run("StopsWhenConsumerBreaksEarly", func(t *testing.T) {
			counter := &atomic.Int64{}
			results := []Result[int]{
				func() (int, error) { counter.Add(1); return 1, nil },
				func() (int, error) { counter.Add(1); return 2, nil },
				func() (int, error) { counter.Add(1); return 3, nil },
			}

			seen := 0
			for range ResolveFutures[int](t.Context(), irt.Slice(results)) {
				seen++
				break
			}

			assert.Equal(t, seen, 1)
			assert.Equal(t, counter.Load(), int64(1))
		})
	})

	t.Run("ResolveFuturesAll", func(t *testing.T) {
		t.Run("EmptySequence", func(t *testing.T) {
			kvs := collect2(ResolveFuturesAll[int](t.Context(), irt.Slice([]Producer[int]{})))
			assert.Equal(t, len(kvs), 0)
		})

		t.Run("ProcessesAllDespiteErrors", func(t *testing.T) {
			expected := ers.Error("test error")
			counter := &atomic.Int64{}
			results := []Result[int]{
				func() (int, error) { counter.Add(1); return 1, nil },
				func() (int, error) { counter.Add(1); return 0, expected },
				func() (int, error) { counter.Add(1); return 3, nil },
			}
			kvs := collect2(ResolveFuturesAll[int](t.Context(), irt.Slice(results)))
			assert.Equal(t, len(kvs), 3)
			assert.Equal(t, counter.Load(), int64(3))
			check.NotError(t, kvs[0].Value)
			check.ErrorIs(t, kvs[1].Value, expected)
			check.NotError(t, kvs[2].Value)
		})
	})

	t.Run("ResolveFuturesWithPool", func(t *testing.T) {
		const jobCount = 40
		const minDuration = 10 * time.Millisecond

		t.Run("EmptySequence", func(t *testing.T) {
			kvs := collect2(ResolveFuturesWithPool[int](t.Context(), irt.Slice([]Producer[int]{}), WorkerGroupConfDefaults()))
			assert.Equal(t, len(kvs), 0)
		})

		t.Run("ConcurrentExecution", func(t *testing.T) {
			counter := &atomic.Int64{}
			producers := make([]Producer[int], jobCount)
			for i := range jobCount {
				producers[i] = func() int {
					counter.Add(1)
					time.Sleep(minDuration)
					return 1
				}
			}

			start := time.Now()
			kvs := collect2(ResolveFuturesWithPool[int](t.Context(), irt.Slice(producers), WorkerGroupConfDefaults()))
			dur := time.Since(start)

			assert.Equal(t, len(kvs), jobCount)
			assert.Equal(t, counter.Load(), int64(jobCount))
			if dur > 500*time.Millisecond {
				t.Errorf("took too long: %v", dur)
			}
		})

		t.Run("YieldsErrors", func(t *testing.T) {
			results := make([]Result[int], 10)
			for i := range 10 {
				results[i] = func() (int, error) { return 0, ers.Error("error") }
			}
			kvs := collect2(ResolveFuturesWithPool[int](t.Context(), irt.Slice(results), WorkerGroupConfContinueOnError(), WorkerGroupConfWorkerPerCPU()))
			assert.Equal(t, len(kvs), 10)
			for _, kv := range kvs {
				check.Error(t, kv.Value)
			}
		})

		t.Run("OpRunsConcurrentlyAcrossWorkers", func(t *testing.T) {
			inFlight := &atomic.Int64{}
			maxInFlight := &atomic.Int64{}

			producers := make([]Producer[int], 20)
			for i := range 20 {
				producers[i] = func() int {
					cur := inFlight.Add(1)
					defer inFlight.Add(-1)
					for {
						prev := maxInFlight.Load()
						if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
							break
						}
					}
					time.Sleep(5 * time.Millisecond)
					return 1
				}
			}

			irt.Apply2(ResolveFuturesWithPool[int](t.Context(), irt.Slice(producers), WorkerGroupConfWorkerPerCPU()), func(int, error) {})
			if maxInFlight.Load() < 2 {
				t.Errorf("expected concurrent execution, max in-flight was %d", maxInFlight.Load())
			}
		})
	})

	t.Run("InvalidOptions", func(t *testing.T) {
		t.Run("ResolveFuturesWithPoolNilErrorCollector", func(t *testing.T) {
			kvs := collect2(ResolveFuturesWithPool[int](
				t.Context(),
				irt.Slice([]Producer[int]{func() int { return 1 }}),
				WorkerGroupConfWithErrorCollector(nil),
			))
			assert.Equal(t, len(kvs), 1)
			check.Error(t, kvs[0].Value)
			check.ErrorIs(t, kvs[0].Value, ers.ErrInvalidInput)
		})
	})
}

func collect2[A, B any](seq iter.Seq2[A, B]) []irt.KV[A, B] { return irt.Collect(irt.KVjoin(seq)) }
