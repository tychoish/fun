package itertool

import (
	"context"
	"sync"
	"testing"

	"github.com/tychoish/fun"
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
