package itertool

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/pubsub"
	"github.com/tychoish/fun/set"
)

func getConstructors[T comparable](t *testing.T, ctx context.Context) []FixtureIteratorConstuctors[T] {
	return []FixtureIteratorConstuctors[T]{
		{
			Name: "SliceIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				return Slice(elems)
			},
		},
		{
			Name: "VariadicIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				return Variadic(elems...)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return Channel(vals)
			},
		},
		{
			Name: "SetIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				set := set.MakeUnordered[T](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator()
			},
		},
		{
			Name: "OrderedSetIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				set := set.MakeOrdered[T](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator()
			},
		},
		{
			Name: "QueueIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				cue, err := pubsub.NewQueue[T](pubsub.QueueOptions{
					SoftQuota: len(elems),
					HardLimit: 2 * len(elems),
				})
				if err != nil {
					t.Fatal(err)
				}

				for idx := range elems {
					if err := cue.Add(elems[idx]); err != nil {
						t.Fatal(err)
					}
				}

				if err = cue.Close(); err != nil {
					t.Fatal(err)
				}

				return cue.Iterator()
			},
		},
	}

}

func TestIteratorImplementations(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	elems := []FixtureData[string]{
		{
			Name:     "Basic",
			Elements: []string{"a", "b", "c", "d"},
		},
		{
			Name:     "Large",
			Elements: GenerateRandomStringSlice(100),
		},
	}

	filters := []FixtureIteratorFilter[string]{
		{
			Name:   "Unsynchronized",
			Filter: func(in fun.Iterator[string]) fun.Iterator[string] { return in },
		},
		{
			Name: "Synchronized",
			Filter: func(in fun.Iterator[string]) fun.Iterator[string] {
				return Synchronize(in)

			},
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(ctx, t, elems, getConstructors[string](t, ctx), filters)
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorStringAlgoTests(ctx, t, elems, getConstructors[string](t, ctx), filters)
	})
}

func TestIteratorAlgoInts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	elemGenerator := func() []int {
		e := make([]int, 100)
		for idx := range e {
			e[idx] = idx
		}
		return e
	}

	elems := []FixtureData[int]{
		{
			Name:     "Basic",
			Elements: []int{1, 2, 3, 4, 5},
		},
		{
			Name:     "Large",
			Elements: elemGenerator(),
		},
	}

	filters := []FixtureIteratorFilter[int]{
		{
			Name:   "Unsynchronized",
			Filter: func(in fun.Iterator[int]) fun.Iterator[int] { return in },
		},
		{
			Name: "Synchronized",
			Filter: func(in fun.Iterator[int]) fun.Iterator[int] {
				return Synchronize(in)

			},
		},
	}

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(ctx, t, elems, getConstructors[int](t, ctx), filters)
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorIntegerAlgoTests(ctx, t, elems, getConstructors[int](t, ctx), filters)
	})
}

func TestWrap(t *testing.T) {
	base := Slice([]string{"a", "b"})
	wrapped := Synchronize(base)
	maybeBase := wrapped.(interface{ Unwrap() fun.Iterator[string] })
	if maybeBase == nil {
		t.Fatal("should not be nil")
	}
	if maybeBase.Unwrap() != base {
		t.Error("should be the same object")
	}
}

func TestRangeSplit(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Range", func(t *testing.T) {
		t.Run("Basic", func(t *testing.T) {
			input := Slice(GenerateRandomStringSlice(10))

			rf := Range(ctx, input)

			set := set.MakeOrdered[string](10)

			var st string
			for rf(ctx, &st) {
				set.Add(st)
			}

			if set.Len() != 10 {
				t.Error("did not sufficently iteratre", set.Len())
			}
		})
		t.Run("Parallel", func(t *testing.T) {
			input := Slice(GenerateRandomStringSlice(100))

			rf := Range(ctx, input)

			set := set.Synchronize(set.MakeOrdered[string](100))

			wg := &fun.WaitGroup{}
			for i := 0; i < 10; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()

					var st string
					for rf(ctx, &st) {
						set.Add(st)
					}
				}()
			}

			wg.Wait(ctx)

			if set.Len() != 100 {
				t.Error("did not iterate enough")

			}
		})
	})
	t.Run("Split", func(t *testing.T) {
		input := Slice(GenerateRandomStringSlice(100))

		splits := Split(ctx, 0, input)
		if splits != nil {
			t.Fatal("should be nil if empty")
		}

		splits = Split(ctx, 10, input)
		if len(splits) != 10 {
			t.Fatal("didn't make enough split")
		}

		set := set.Synchronize(set.MakeOrdered[string](100))

		wg := &fun.WaitGroup{}
		for _, iter := range splits {
			wg.Add(1)

			go func(it fun.Iterator[string]) {
				defer wg.Done()

				for it.Next(ctx) {
					set.Add(it.Value())
				}
			}(iter)
		}

		wg.Wait(ctx)

		if set.Len() != 100 {
			t.Error("did not iterate enough")

		}

	})
}

func TestTools(t *testing.T) {
	t.Parallel()
	for i := 0; i < 10; i++ {
		t.Run(fmt.Sprint("Iteration", i), func(t *testing.T) {
			t.Run("CancelCollectChannel", func(t *testing.T) {
				bctx, bcancel := context.WithCancel(context.Background())
				defer bcancel()

				ctx, cancel := context.WithCancel(bctx)
				defer cancel()

				pipe := make(chan string, 1)
				sig := make(chan struct{})

				go func() {
					defer close(sig)
					for {
						select {
						case <-bctx.Done():
							return
						case pipe <- t.Name():
							continue
						}
					}
				}()

				output := CollectChannel(ctx, Channel(pipe))
				runtime.Gosched()

				count := 0
			CONSUME:
				for {
					select {
					case _, ok := <-output:
						if ok {
							count++
							cancel()
						}
						if !ok {
							break CONSUME
						}
					case <-sig:
						break CONSUME
					case <-time.After(10 * time.Millisecond):
						break CONSUME
					}
				}
				if count != 1 {
					t.Error(count)
				}
			})
		})
	}
	t.Run("MapWorkerSendingBlocking", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string, 1)
		output := make(chan int)
		catcher := &erc.Collector{}
		wg := &fun.WaitGroup{}
		pipe <- t.Name()
		wg.Add(1)
		go mapWorker(
			ctx,
			catcher,
			wg,
			Options{},
			func(ctx context.Context, in string) (int, error) { return 53, nil },
			func() {},
			pipe,
			output,
		)
		time.Sleep(10 * time.Millisecond)
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		wg.Wait(ctx)

		count := 0
	CONSUME:
		for {
			select {
			case _, ok := <-output:
				if ok {
					count++
					continue
				}
				break CONSUME
			case <-ctx.Done():
				break CONSUME
			}
		}
	})
	t.Run("MergeReleases", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string)
		iter := Merge(ctx, Channel(pipe), Channel(pipe), Channel(pipe))
		pipe <- t.Name()

		time.Sleep(10 * time.Millisecond)
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		if iter.Next(ctx) {
			t.Error("no iteration", iter.Value())
		}
	})
}

func TestParallelForEach(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Basic", func(t *testing.T) {
		for i := int64(0); i <= 8; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				elems := makeIntSlice(200)
				seen := set.Synchronize(set.NewOrdered[int]())
				err := ParallelForEach(ctx,
					Slice(elems),
					func(ctx context.Context, in int) error {
						jitter := time.Duration(rand.Int63n(1 + i*int64(time.Millisecond)))
						time.Sleep(time.Millisecond + jitter)
						seen.Add(in)
						return nil
					},
					Options{NumWorkers: int(i)},
				)
				if err != nil {
					t.Fatal(err)
				}
				out, err := CollectSlice(ctx, seen.Iterator())
				if err != nil {
					t.Fatal(err)
				}
				if len(out) != len(elems) {
					t.Log("output", out)
					t.Log("input", elems)
					t.Error("unequal length slices")
				}

				matches := 0
				for idx := range out {
					if out[idx] == elems[idx] {
						matches++
					}
				}
				if i < 2 && matches != len(out) {
					t.Error("should all with 1 worker", matches, len(out))
				} else if i >= 2 && matches == len(out) {
					t.Error("should not all match", matches, len(out))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		seen := set.Synchronize(set.MakeNewOrdered[int]())
		err := ParallelForEach(ctx,
			Slice(makeIntSlice(200)),
			func(ctx context.Context, in int) error {
				seen.Add(in)
				runtime.Gosched()
				if in >= 100 {
					panic("error")
				}
				return nil
			},
			Options{
				NumWorkers:      3,
				ContinueOnPanic: true,
			},
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 200 {
			// panics and expected
			t.Error(len(errs))
		}
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seen := set.Synchronize(set.NewUnordered[int]())
		err := ParallelForEach(ctx,
			Slice(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				if in == 8 {
					// make sure something else
					// has a chance to run before
					// the event.
					runtime.Gosched()
					panic("gotcha")
				} else {
					seen.Add(in)
				}

				<-ctx.Done()
				return nil
			},
			Options{
				NumWorkers:      10,
				ContinueOnPanic: false,
			},
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		if seen.Len() < 1 {
			t.Error("should have only seen one", seen.Len())
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 2 {
			// panic + expected
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ParallelForEach(ctx,
			Slice(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				if in == 8 {
					cancel()
					panic("gotcha")
				}
				return nil
			},
			Options{
				NumWorkers:      4,
				ContinueOnPanic: false,
			},
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
	})
	t.Run("CollectAllErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ParallelForEach(ctx,
			Slice(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			Options{
				NumWorkers:      4,
				ContinueOnError: true,
			},
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 10 {
			t.Error(len(errs))
		}

	})
	t.Run("CollectAllErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ParallelForEach(ctx,
			Slice(makeIntSlice(100)),
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
			},
			Options{
				NumWorkers:      2,
				ContinueOnError: false,
			},
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > 2 {
			t.Error(len(errs))
		}
	})
}

func TestParallelObserve(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Basic", func(t *testing.T) {
		for i := 1; i <= 8; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				elems := makeIntSlice(200)
				seen := set.Synchronize(set.MakeNewOrdered[int]())
				err := ParallelObserve(ctx,
					Slice(elems),
					func(in int) {
						seen.Add(in)
						if i%2 == 0 {
							// yield to make sure
							// other threads can
							// run sometimes
							runtime.Gosched()
						}
					},
					Options{NumWorkers: i},
				)
				if err != nil {
					t.Fatal(err)
				}
				out, err := CollectSlice(ctx, seen.Iterator())
				if err != nil {
					t.Fatal(err)
				}
				if len(out) != len(elems) {
					t.Log("output", out)
					t.Log("input", elems)
					t.Error("unequal length slices")
				}

				matches := 0
				for idx := range out {
					if out[idx] == elems[idx] {
						matches++
					}
				}
				if i == 1 && matches != len(out) {
					t.Error("should  all with 1 worker", matches, len(out))
				} else if i > 1 && matches == len(out) {
					// this test might be flaky on systems with smaller
					// numbers of cores because it relies on worker
					// threads executing out of order
					t.Error("should not all match", matches, len(out))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		seen := set.Synchronize(set.MakeNewOrdered[int]())
		err := ParallelObserve(ctx,
			Slice(makeIntSlice(200)),
			func(in int) {
				seen.Add(in)
				runtime.Gosched()
				if in >= 100 {
					panic("error")
				}
			},
			Options{
				NumWorkers:      3,
				ContinueOnPanic: true,
			},
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 200 {
			t.Error(len(errs))
		}
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seen := set.Synchronize(set.NewOrdered[int]())
		err := ParallelObserve(ctx,
			Slice(makeIntSlice(50)),
			func(in int) {
				if in == 4 {
					// make sure something else
					// has a chance to run before
					// the event.
					runtime.Gosched()
					panic("gotcha")
				} else {
					seen.Add(in)
				}
			},
			Options{
				NumWorkers:      20,
				ContinueOnPanic: false,
			},
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		if seen.Len() < 1 {
			t.Error("should have seen more than one", seen.Len())
		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 2 {
			// panic sentinel
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ParallelObserve(ctx,
			Slice(makeIntSlice(10)),
			func(in int) {
				if in == 8 {
					cancel()
					panic("gotcha")
				}
			},
			Options{
				NumWorkers:      4,
				ContinueOnPanic: false,
			},
		)
		if err == nil {
			t.Error("should have propogated an error")
		}
	})
	t.Run("WorkerPool", func(t *testing.T) {
		t.Run("ProcessSerial", func(t *testing.T) {
			t.Run("Count", func(t *testing.T) {
				count := 0
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { count++; return nil }
				}

				if err := ProcessWork(ctx, Slice(workers)); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, count, 100)
			})
			t.Run("Panic", func(t *testing.T) {
				expected := errors.New("hello")
				workers := make([]fun.WorkerFunc, 100)
				count := 0
				for i := range workers {
					workers[i] = func(context.Context) error { count++; panic(expected) }
				}

				err := ProcessWork(ctx, Slice(workers))
				if err == nil {
					t.Fatal("expected error")
				}
				errs := erc.Unwind(err)
				if len(errs) != 2 {
					t.Error("unexpected number of errors", len(errs))
				}
				if !errors.Is(err, expected) {
					t.Error(err)
				}
				if count != 1 {
					t.Error("should only run once")
				}
			})
			t.Run("Order", func(t *testing.T) {
				const num = 100
				out := make([]int, num)
				workers := make([]fun.WorkerFunc, num)
				for i := range workers {
					cp := i
					workers[i] = func(context.Context) error { out[cp] = cp; return nil }
				}

				if err := ProcessWork(ctx, Slice(workers)); err != nil {
					t.Fatal(err)
				}

				assert.True(t, sort.IntsAreSorted(out))
				assert.Equal(t, num-1, out[len(out)-1])
			})
			t.Run("ErrorCollection", func(t *testing.T) {
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { return errors.New("womp") }
				}

				err := ProcessWork(ctx, Slice(workers))
				if err == nil {
					t.Fatal("expected errors")
				}
				errs := erc.Unwind(err)
				if len(errs) != 1 {
					t.Log(errs)
					t.Error("unexpected number of errors", len(errs))
				}
			})
		})
		t.Run("Parallel", func(t *testing.T) {
			t.Run("Count", func(t *testing.T) {
				count := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { count.Add(1); return nil }
				}

				if err := WorkerPool(ctx, Slice(workers), Options{NumWorkers: 4}); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, count.Load(), 100)
			})
			t.Run("ErrorCollection", func(t *testing.T) {
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { return errors.New("womp") }
				}

				err := WorkerPool(ctx, Slice(workers), Options{NumWorkers: 4, ContinueOnError: true})
				if err == nil {
					t.Fatal("expected errors")
				}
				errs := erc.Unwind(err)
				if len(errs) != 100 {
					t.Log(errs)
					t.Error("unexpected number of errors", len(errs))
				}
			})
			t.Run("Panic", func(t *testing.T) {
				expected := errors.New("womp")
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { panic(expected) }
				}

				err := WorkerPool(ctx, Slice(workers), Options{NumWorkers: 4, ContinueOnPanic: true})
				if err == nil {
					t.Fatal("expected errors")
				}
				errs := erc.Unwind(err)
				if len(errs) != 200 {
					t.Error("unexpected number of errors", len(errs))
				}
				assert.ErrorIs(t, err, expected)
			})
		})
		t.Run("ObserverSerial", func(t *testing.T) {
			t.Run("Count", func(t *testing.T) {
				count := &atomic.Int64{}
				called := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { called.Add(1); return nil }
				}

				ObserveWorker(ctx, Slice(workers), func(err error) { count.Add(1); check.NotError(t, err) })

				// observe only is called if there's
				// an error or a panic, so not at all here
				assert.Equal(t, count.Load(), 0)
				assert.Equal(t, called.Load(), 100)
			})
			t.Run("Panic", func(t *testing.T) {
				count := &atomic.Int64{}
				called := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				expected := errors.New("womp")
				for i := range workers {
					workers[i] = func(context.Context) error { called.Add(1); panic(expected) }
				}

				ObserveWorker(ctx, Slice(workers), func(err error) { count.Add(1); check.ErrorIs(t, err, expected) })

				// observe must abort on panics
				assert.Equal(t, count.Load(), 1)
				assert.Equal(t, called.Load(), 1)
			})
			t.Run("ErrorCollection", func(t *testing.T) {
				count := &atomic.Int64{}
				called := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				expected := errors.New("womp")
				for i := range workers {
					workers[i] = func(context.Context) error { called.Add(1); return expected }
				}

				ObserveWorker(ctx, Slice(workers), func(err error) { count.Add(1); check.ErrorIs(t, err, expected) })

				// observe is necessarily continue on error
				check.Equal(t, count.Load(), 100)
				check.Equal(t, called.Load(), 100)
			})
		})
		t.Run("ObserveParallel", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			t.Run("Count", func(t *testing.T) {
				count := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				for i := range workers {
					workers[i] = func(context.Context) error { count.Add(1); return nil }
				}

				ObserveWorkerPool(ctx, Slice(workers), func(err error) {
					check.NotError(t, err)
					// this should never be called
					check.True(t, false)
				}, Options{NumWorkers: 4})

				assert.Equal(t, count.Load(), 100)
			})
			t.Run("Panic", func(t *testing.T) {
				count := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				expected := errors.New("womp")
				for i := range workers {
					workers[i] = func(context.Context) error {
						count.Add(1)
						runtime.Gosched()
						panic(expected)
					}
				}

				observations := &atomic.Int64{}
				ObserveWorkerPool(ctx, Slice(workers), func(err error) {
					check.Error(t, err)
					observations.Add(1)
					check.ErrorIs(t, err, expected)
					errs := erc.Unwind(err)
					check.Equal(t, len(errs), 200)
				}, Options{
					NumWorkers:      4,
					ContinueOnPanic: true,
				})

				check.Equal(t, count.Load(), 100)
				check.Equal(t, observations.Load(), 1)
			})
			t.Run("ErrorCollection", func(t *testing.T) {
				count := &atomic.Int64{}
				workers := make([]fun.WorkerFunc, 100)
				expected := errors.New("womp")
				for i := range workers {
					workers[i] = func(context.Context) error { count.Add(1); return expected }
				}

				observations := &atomic.Int64{}
				ObserveWorkerPool(ctx, Slice(workers), func(err error) {
					check.Error(t, err)
					observations.Add(1)
					check.ErrorIs(t, err, expected)
				}, Options{
					NumWorkers:      4,
					ContinueOnError: true,
				})

				assert.Equal(t, count.Load(), 100)
				assert.Equal(t, observations.Load(), 100)
			})
		})
	})
}
