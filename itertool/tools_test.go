package itertool

import (
	"context"
	"testing"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/set"
)

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

	builders := []FixtureIteratorConstuctors[string]{
		{
			Name: "SliceIterator",
			Constructor: func(elems []string) fun.Iterator[string] {
				return Slice(elems)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []string) fun.Iterator[string] {
				vals := make(chan string, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return Channel(vals)
			},
		},
		{
			Name: "SetIterator",
			Constructor: func(elems []string) fun.Iterator[string] {
				set := set.MakeUnordered[string](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator(ctx)
			},
		},
		{
			Name: "OrderedSetIterator",
			Constructor: func(elems []string) fun.Iterator[string] {
				set := set.MakeOrdered[string](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator(ctx)
			},
		},
		// "QueueIterator": func(elems string) fun.Iterator[string] {
		// 	cue, err := NewQueue[string](QueueOptions{
		// 		SoftQuota: len(elements.Elements),
		// 		HardLimit: 2 * len(elements.Elements),
		// 	})
		// 	if err != nil {
		// 		t.Fatal(err)
		// 	}

		// 	for idx := range elements.Elements {
		// 		cue.Add(elements.Elements[idx])
		// 	}
		// 	_ = cue.Close()
		// 	return cue.Iterator()
		// },
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
		RunIteratorImplementationTests(ctx, t, elems, builders, filters)
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorStringAlgoTests(ctx, t, elems, builders, filters)
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

	builders := []FixtureIteratorConstuctors[int]{
		{
			Name: "SliceIterator",
			Constructor: func(elems []int) fun.Iterator[int] {
				return Slice(elems)
			},
		},
		{
			Name: "ChannelIterator",
			Constructor: func(elems []int) fun.Iterator[int] {
				vals := make(chan int, len(elems))
				for idx := range elems {
					vals <- elems[idx]
				}
				close(vals)
				return Channel(vals)
			},
		},
		{
			Name: "SetIterator",
			Constructor: func(elems []int) fun.Iterator[int] {
				set := set.MakeUnordered[int](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator(ctx)
			},
		},
		{
			Name: "OrderedSetIterator",
			Constructor: func(elems []int) fun.Iterator[int] {
				set := set.MakeOrdered[int](len(elems))
				for idx := range elems {
					set.Add(elems[idx])
				}

				return set.Iterator(ctx)
			},
		},
		// "QueueIterator": func(elems []int) fun.Iterator[int] {
		// 	cue, err := NewQueue[int](QueueOptions{
		// 		SoftQuota: len(elements.Elements),
		// 		HardLimit: 2 * len(elements.Elements),
		// 	})
		// 	if err != nil {
		// 		t.Fatal(err)
		// 	}

		// 	for idx := range elements.Elements {
		// 		cue.Add(elements.Elements[idx])
		// 	}
		// 	_ = cue.Close()
		// 	return cue.Iterator()
		// },
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
		RunIteratorImplementationTests(ctx, t, elems, builders, filters)
	})

	t.Run("Aggregations", func(t *testing.T) {
		RunIteratorIntegerAlgoTests(ctx, t, elems, builders, filters)
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
