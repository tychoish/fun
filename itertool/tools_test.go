package itertool

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/testt"
)

func getConstructors[T comparable](t *testing.T) []FixtureIteratorConstuctors[T] {
	return []FixtureIteratorConstuctors[T]{
		{
			Name: "SlicifyIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.Sliceify(elems).Iterator()
			},
		},
		{
			Name: "Slice",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.SliceIterator(elems)
			},
		},
		{
			Name: "VariadicIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				return fun.VariadicIterator(elems...)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []T) *fun.Iterator[T] {
				vals := make(chan T, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return fun.Blocking(vals).Iterator()
			},
		},
		// {
		// 	Name: "QueueIterator",
		// 	Constructor: func(elems []T) *fun.Iterator[T] {
		// 		cue, err := pubsub.NewQueue[T](pubsub.QueueOptions{
		// 			SoftQuota: len(elems),
		// 			HardLimit: 2 * len(elems),
		// 		})
		// 		if err != nil {
		// 			t.Fatal(err)
		// 		}

		// 		for idx := range elems {
		// 			if err := cue.Add(elems[idx]); err != nil {
		// 				t.Fatal(err)
		// 			}
		// 		}

		// 		if err = cue.Close(); err != nil {
		// 			t.Fatal(err)
		// 		}

		// 		return cue.Iterator()
		// 	},
		// },
	}

}

func TestIteratorImplementations(t *testing.T) {
	t.Parallel()
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

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(t, elems, getConstructors[string](t))
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorStringAlgoTests(t, elems, getConstructors[string](t))
	})
}

func TestIteratorAlgoInts(t *testing.T) {
	t.Parallel()

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

	t.Run("SimpleOperations", func(t *testing.T) {
		RunIteratorImplementationTests(t, elems, getConstructors[int](t))
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorIntegerAlgoTests(t, elems, getConstructors[int](t))
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

				output := fun.Blocking(pipe).Iterator().Channel(ctx)
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

		mapWorker(
			catcher,
			Options{},
			func(ctx context.Context, in string) (int, error) { return 53, nil },
			fun.Blocking(pipe).Iterator(),
			fun.Blocking(output).Send(),
		).Ignore().Add(ctx, wg)
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
}

func TestParallelForEach(t *testing.T) {
	t.Parallel()

	ctx := testt.Context(t)

	t.Run("Basic", func(t *testing.T) {
		for i := int64(-1); i <= 12; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				elems := makeIntSlice(200)

				seen := &adt.Map[int, none]{}

				err := Process(ctx,
					fun.SliceIterator(elems),
					func(ctx context.Context, in int) error {
						abs := int64(math.Abs(float64(i)))

						jitter := time.Duration(rand.Int63n(1 + abs*int64(time.Millisecond)))
						time.Sleep(time.Millisecond + jitter)
						seen.Ensure(in)
						return nil
					},
					Options{NumWorkers: int(i)},
				)
				if err != nil {
					t.Fatal(err)
				}
				out := fun.Must(seen.Keys().Slice(ctx))

				testt.Log(t, "output", out)
				testt.Log(t, "input", elems)

				if len(out) != len(elems) {
					t.Error("unequal length slices")
				}

				matches := 0
				for idx := range out {
					if out[idx] == elems[idx] {
						matches++
					}
				}
				if i >= 2 && matches == len(out) {
					t.Error("should not all match", matches, len(out))
				}
			})
		}
	})

	t.Run("ContinueOnPanic", func(t *testing.T) {
		seen := &adt.Map[int, none]{}

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(200)),
			func(ctx context.Context, in int) error {
				seen.Ensure(in)
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

		errs := fun.Must(es.Iterator().Slice(ctx))

		if len(errs) != 200 {
			// panics and expected
			t.Error(len(errs))
		}
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seen := &adt.Map[int, none]{}

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(10)),
			func(ctx context.Context, in int) error {
				if in == 8 {
					// make sure something else
					// has a chance to run before
					// the event.
					runtime.Gosched()
					panic("gotcha")
				} else {
					seen.Ensure(in)
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
		errs := fun.Must(es.Iterator().Slice(ctx))
		if len(errs) != 2 {
			// panic + expected
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(10)),
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

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(10)),
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
		errs := fun.Must(es.Iterator().Slice(ctx))
		if len(errs) != 10 {
			t.Error(len(errs), "!= 10", errs)
		}

	})
	t.Run("CollectAllErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := Process(ctx,
			fun.SliceIterator(makeIntSlice(100)),
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
		errs := fun.Must(es.Iterator().Slice(ctx))
		// it's two and not one because each worker thread
		// ran one task before aborting
		if len(errs) > 2 {
			t.Error(len(errs))
		}
	})
}

func TestContains(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Exists", func(t *testing.T) {
		assert.True(t, Contains[int](ctx, 1, fun.SliceIterator([]int{12, 3, 44, 1})))
	})
	t.Run("NotExists", func(t *testing.T) {
		assert.True(t, !Contains[int](ctx, 1, fun.SliceIterator([]int{12, 3, 44})))
	})
}

func TestUniq(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sl := []int{1, 1, 2, 3, 5, 8, 9, 5}
	assert.Equal(t, fun.SliceIterator(sl).Count(ctx), 8)

	assert.Equal(t, Uniq(fun.SliceIterator(sl)).Count(ctx), 6)
}

func TestChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	num := []int{1, 2, 3, 5, 7, 9, 11, 13, 17, 19}
	iter := Chain[int](fun.SliceIterator(num), fun.SliceIterator(num))
	n := iter.Count(ctx)
	assert.Equal(t, len(num)*2, n)

	iter = Chain[int](fun.SliceIterator(num), fun.SliceIterator(num), fun.SliceIterator(num), fun.SliceIterator(num))
	cancel()
	n = iter.Count(ctx)
	assert.Equal(t, n, 0)
}

func TestDropZeros(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	all := make([]string, 100)
	n := fun.SliceIterator(all).Count(ctx)
	assert.Equal(t, 100, n)
	n = DropZeroValues[string](fun.SliceIterator(all)).Count(ctx)
	assert.Equal(t, 0, n)

	DropZeroValues[string](fun.SliceIterator(all)).Observe(ctx, func(in string) { assert.Zero(t, in) })

	all[45] = "49"
	n = DropZeroValues[string](fun.SliceIterator(all)).Count(ctx)
	assert.Equal(t, 1, n)
}

func TestWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	count := &atomic.Int64{}
	err := Worker(ctx, fun.SliceIterator([]fun.WaitFunc{
		func(context.Context) { count.Add(1) },
		func(context.Context) { count.Add(1) },
		func(context.Context) { count.Add(1) },
	}), Options{})
	assert.NotError(t, err)
	assert.Equal(t, count.Load(), 3)
}
