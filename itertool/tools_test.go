package itertool

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun"
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

				return set.Iterator(ctx)
			},
		},
		{
			Name: "OrderedSetIterator",
			Constructor: func(elems []T) fun.Iterator[T] {
				set := set.MakeOrdered[T](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator(ctx)
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

			wg := &sync.WaitGroup{}
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

			fun.Wait(ctx, wg)

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

		wg := &sync.WaitGroup{}
		for _, iter := range splits {
			wg.Add(1)

			go func(it fun.Iterator[string]) {
				defer wg.Done()

				for it.Next(ctx) {
					set.Add(it.Value())
				}
			}(iter)
		}

		fun.Wait(ctx, wg)

		if set.Len() != 100 {
			t.Error("did not iterate enough")

		}

	})
}

func TestTools(t *testing.T) {
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
	t.Run("MapWorkerSendingBlocking", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pipe := make(chan string, 1)
		output := make(chan int)
		catcher := &erc.Collector{}
		wg := &sync.WaitGroup{}
		pipe <- t.Name()
		wg.Add(1)
		go mapWorker(
			ctx,
			catcher,
			wg,
			Options{},
			MapperFunction(func(ctx context.Context, in string) (int, error) { return 53, nil }),
			func() {},
			pipe,
			output,
		)
		time.Sleep(10 * time.Millisecond)
		cancel()
		wg.Wait()

		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

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
	t.Run("MapSkipSomeValues", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		strs := makeIntSlice(100)
		output := Map(ctx, Options{NumWorkers: 5}, Slice(strs),
			func(ctx context.Context, in int) (int, bool, error) {
				if in%2 == 0 {
					return math.MaxInt, false, nil
				}
				return in, true, nil
			})
		vals, err := CollectSlice(ctx, output)
		if err != nil {
			t.Fatal(err)
		}

		if len(vals) != 50 {
			t.Error("unexpected values", len(vals), vals)
		}
		for idx, v := range vals {
			if v == math.MaxInt {
				t.Error(idx)
			}
		}
	})
}

func TestParallelForEach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("Basic", func(t *testing.T) {
		for i := 0; i <= 8; i++ {
			t.Run(fmt.Sprintf("Threads%d", i), func(t *testing.T) {
				elems := makeIntSlice(200)
				seen := set.Synchronize(set.MakeNewOrdered[int]())
				err := ParallelForEach(ctx,
					Slice(elems),
					Options{NumWorkers: i},
					func(ctx context.Context, in int) error {
						seen.Add(in)
						return nil
					},
				)
				if err != nil {
					t.Fatal(err)
				}
				out, err := CollectSlice(ctx, seen.Iterator(ctx))
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
					t.Error("should  all with 1 worker", matches, len(out))
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
			Options{
				NumWorkers:      3,
				ContinueOnPanic: true,
			},
			func(ctx context.Context, in int) error {
				seen.Add(in)
				if in >= 100 {
					panic("error")
				}
				return nil
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
		if len(errs) != 100 {
			t.Error(len(errs))
		}
	})
	t.Run("AbortOnPanic", func(t *testing.T) {
		seen := set.Synchronize(set.MakeNewOrdered[int]())
		err := ParallelForEach(ctx,
			Slice(makeIntSlice(10)),
			Options{
				NumWorkers:      10,
				ContinueOnPanic: false,
			},
			func(ctx context.Context, in int) error {
				if in == 4 {
					seen.Add(in)
				}
				if in == 8 {
					panic("gotcha")
				}

				<-ctx.Done()
				return nil
			},
		)
		if err == nil {
			t.Fatal("should not have errored", err)
		}
		if seen.Len() != 1 {
			t.Error("should have only seen one", seen.Len())

		}
		var es *erc.Stack
		if !errors.As(err, &es) {
			t.Fatal(err)
		}
		errs := fun.Must(CollectSlice(ctx, es.Iterator()))
		if len(errs) != 1 {
			t.Error(len(errs))
		}
	})
	t.Run("CancelAndPanic", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := ParallelForEach(ctx,
			Slice(makeIntSlice(10)),
			Options{
				NumWorkers:      4,
				ContinueOnPanic: false,
			},
			func(ctx context.Context, in int) error {
				if in == 8 {
					cancel()
					panic("gotcha")
				}
				return nil
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
			Options{
				NumWorkers:      4,
				ContinueOnError: true,
			},
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
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
			Options{
				NumWorkers:      2,
				ContinueOnError: false,
			},
			func(ctx context.Context, in int) error {
				return fmt.Errorf("errored=%d", in)
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
	t.Run("MapCheckedFunction", func(t *testing.T) {
		out := Map(
			ctx,
			Options{},
			Slice(makeIntSlice(100)),
			CheckedFunction(func(ctx context.Context, num int) (int, bool) {
				if num%2 == 0 {
					return math.MaxInt, false
				}

				return num, true
			}),
		)

		vals, err := CollectSlice(ctx, out)
		if err != nil {
			t.Fatal(err)

		}
		if len(vals) != 50 {
			t.Fatal(len(vals), vals)
		}
	})
	t.Run("CollectErrors", func(t *testing.T) {
		ec := &erc.Collector{}
		pf := CollectErrors(ec, func(ctx context.Context, in string) (int, error) {
			if in == "hi" {
				return 400, errors.New(in)
			}
			return 42, nil
		})
		out, ok, err := pf(ctx, "hello")
		if err != nil || !ok || ec.HasErrors() {
			t.Error("should not error", ok, err, ec.Resolve())
		}
		out, ok, err = pf(ctx, "hi")
		if err != nil {
			t.Error("collect errors shouldn't transmit errors", err)
		}
		if ok {
			t.Error("should out be ok, for error values")
		}
		if !ec.HasErrors() {
			t.Error("should collect errors")
		}
		if err := ec.Resolve(); err == nil {
			t.Error("should have resolve error")
		}
		if out != 0 {
			t.Error("wrapper should not propgate value in error case")
		}
	})

}
