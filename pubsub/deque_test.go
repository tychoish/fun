package pubsub

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/set"
)

type fixture[T any] struct {
	name     string
	add      func(T) error
	remove   func() (T, bool)
	iterator func() fun.Iterator[T]
	close    func() error
	len      func() int
	elems    []T
}

func randomStringSlice(size int) []string {
	elems := make([]string, 50)

	for i := 0; i < 50; i++ {
		elems[i] = fmt.Sprintf("value=%d", i)
	}
	return elems
}

func generateDequeFixtures[T any](makeElems func(int) []T) []func() fixture[T] {
	return []func() fixture[T]{
		func() fixture[T] {
			cue := NewUnlimitedQueue[T]()

			return fixture[T]{
				name:     "QueueUnlimited",
				add:      cue.Add,
				remove:   cue.Remove,
				iterator: cue.Iterator,
				elems:    makeElems(50),
				close:    cue.Close,
				len:      cue.tracker.len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewQueue[T](QueueOptions{HardLimit: 100, SoftQuota: 60}))

			return fixture[T]{
				name:     "QueueLimited",
				add:      cue.Add,
				remove:   cue.Remove,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.tracker.len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Unlimited: true}))

			return fixture[T]{
				name:     "DequePushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Unlimited: true}))

			return fixture[T]{
				name:     "DequePushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Unlimited: true}))

			return fixture[T]{
				name:     "DequePushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				close:    cue.Close,
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Unlimited: true}))

			return fixture[T]{
				name:     "DequePushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				len:      cue.Len,
				close:    cue.Close,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Unlimited: true}))

			return fixture[T]{
				name:     "DequePushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},
		// simple capacity
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:     "DequeCapacityPushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:     "DequeCapacityPushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				close:    cue.Close,
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:     "DequeCapacityPushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				len:      cue.Len,
				close:    cue.Close,
			}
		},

		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:     "DequeCapacityPushBackPopFrontForward",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},

		// bursty limited size
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:     "DequeBurstPushFrontPopBackForward",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.Iterator,
				close:    cue.Close,
				elems:    makeElems(50),
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:     "DequeBurstPushBackPopFrontReverse",
				add:      cue.PushBack,
				remove:   cue.PopFront,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				close:    cue.Close,
				len:      cue.Len,
			}
		},
		func() fixture[T] {
			cue := fun.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:     "DequeBurstPushFrontPopBackReverse",
				add:      cue.PushFront,
				remove:   cue.PopBack,
				iterator: cue.IteratorReverse,
				elems:    makeElems(50),
				len:      cue.Len,
				close:    cue.Close,
			}
		},
	}

}

func RunDequeTests[T comparable](ctx context.Context, t *testing.T, f func() fixture[T]) {
	fix := f()
	t.Run(fix.name, func(t *testing.T) {
		f := f
		t.Parallel()
		t.Run("AddRemove", func(t *testing.T) {
			fix := f()
			for _, e := range fix.elems {
				if err := fix.add(e); err != nil {
					t.Fatal(err)
				}
			}

			if fix.len() != len(fix.elems) {
				t.Fatal("add did not work")
			}

			set := set.NewUnordered[T]()
			for i := len(fix.elems); i > 0; i-- {
				out, ok := fix.remove()
				if !ok {
					t.Error("remove should not fail", i)
				}
				set.Add(out)
			}
			if set.Len() != len(fix.elems) {
				t.Fatal("did not see all expected results", set.Len(), len(fix.elems))
			}

			if fix.len() != 0 {
				t.Error("remove did not work")
			}
		})
		t.Run("Iterate", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			fix := f()
			for _, e := range fix.elems {
				if err := fix.add(e); err != nil {
					t.Fatal(err)
				}
			}
			seen := 0
			iter := fix.iterator()

			if err := fix.close(); err != nil {
				t.Fatal(err)
			}

			for iter.Next(ctx) {
				seen++
				if iter.Value() == *new(T) {
					t.Fatal("problem at", seen)
				}
			}
			if seen != len(fix.elems) {
				t.Fatal("did not iterate far enough", seen, len(fix.elems))
			}

			if err := ctx.Err(); err != nil {
				t.Error("shouldn't cancel", err)
			}
			if err := iter.Close(ctx); err != nil {
				t.Fatal(err)
			}
		})

	})

}

func TestDeque(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	t.Run("String", func(t *testing.T) {
		t.Parallel()
		for _, f := range generateDequeFixtures(randomStringSlice) {
			RunDequeTests(ctx, t, f)
		}
	})
	t.Run("Integer", func(t *testing.T) {
		t.Parallel()
		for _, f := range generateDequeFixtures(randomIntSlice) {
			RunDequeTests(ctx, t, f)
		}
	})
	t.Run("Config", func(t *testing.T) {
		t.Run("InvalidQueueOptions", func(t *testing.T) {
			conf := DequeOptions{
				QueueOptions: &QueueOptions{HardLimit: 4, SoftQuota: 5},
			}
			if err := conf.Validate(); err == nil {
				t.Fatal()
			}
			if _, err := NewDeque[string](conf); err == nil {
				t.Fatal()
			}
		})
		t.Run("NegativeCapacity", func(t *testing.T) {
			conf := DequeOptions{
				Capacity: -1,
			}
			if err := conf.Validate(); err == nil {
				t.Fatal()
			}
			if _, err := NewDeque[string](conf); err == nil {
				t.Fatal()
			}
		})
		t.Run("Zero", func(t *testing.T) {
			conf := DequeOptions{}
			if err := conf.Validate(); err == nil {
				t.Fatal()
			}
			if _, err := NewDeque[string](conf); err == nil {
				t.Fatal()
			}
		})
		t.Run("ConflictingOptions", func(t *testing.T) {
			conf := DequeOptions{
				Capacity:  100,
				Unlimited: true,
			}
			if err := conf.Validate(); err == nil {
				t.Fatal()
			}
			if _, err := NewDeque[string](conf); err == nil {
				t.Fatal()
			}
		})
		t.Run("TrivallyCorrect", func(t *testing.T) {
			for idx, err := range []error{
				(&DequeOptions{Capacity: 1}).Validate(),
				(&DequeOptions{Capacity: math.MaxInt}).Validate(),
				(&DequeOptions{Unlimited: true}).Validate(),
				(&DequeOptions{QueueOptions: &QueueOptions{SoftQuota: 400, HardLimit: 1000}}).Validate(),
			} {
				if err != nil {
					t.Fatal(idx, err)
				}
			}
		})
	})
	t.Run("ForcePush", func(t *testing.T) {
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			if i <= 50 {
				if err := dq.ForcePushBack(i); err != nil {
					t.Fatal(err)
				}
			} else if err := dq.ForcePushFront(i); err != nil {
				t.Fatal(err)
			}
			if i < 9 && dq.Len() != i+1 {
				t.Fatal("got the wrong length", dq.Len())
			}
			if i >= 9 && dq.Len() != 10 {
				t.Fatal("exceded capacity", dq.Len())
			}
		}
		val, ok := dq.PopFront()
		if !ok || val != 99 {
			t.Error(val)
		}
	})
	t.Run("Push", func(t *testing.T) {
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 100; i++ {
			if i < 10 {
				if err := dq.PushBack(i); err != nil {
					t.Fatal(err)
				}
			} else if err := dq.PushFront(i); err == nil {
				t.Fatal("shold not add item to full list", err)
			}
			if i < 9 && dq.Len() != i+1 {
				t.Fatal("got the wrong length", dq.Len())
			}
			if i >= 9 && dq.Len() != 10 {
				t.Fatal("exceded capacity", dq.Len())
			}
		}
		val, ok := dq.PopBack()
		if !ok || val != 9 {
			t.Error(val)
		}
	})
	t.Run("Empty", func(t *testing.T) {
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := dq.PopBack(); ok {
			t.Error("should not pop empty list")
		}
		if _, ok := dq.PopFront(); ok {
			t.Error("should not pop empty list")
		}
		t.Run("Iterator", func(t *testing.T) {
			for idx, iter := range []fun.Iterator[int]{
				dq.Iterator(),
				dq.IteratorReverse(),
			} {
				t.Run(fmt.Sprint(idx), func(t *testing.T) {
					if iter.Next(ctx) {
						t.Error("should not iterate", idx)
					}
				})
			}
		})
	})
	t.Run("WaitingBack", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		startAt := time.Now()
		go func() {
			time.Sleep(10 * time.Millisecond)
			if err := dq.PushBack(100); err != nil {
				t.Error(err)
			}
		}()
		time.Sleep(time.Millisecond)
		out, err := dq.WaitBack(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(startAt) < 10*time.Millisecond {
			t.Error("did not sleep")
		}
		if out != 100 {
			t.Error("100 !=", out)
		}
	})
	t.Run("WaitingFront", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		startAt := time.Now()
		go func() {
			time.Sleep(10 * time.Millisecond)
			if err := dq.PushFront(100); err != nil {
				t.Error(err)
			}
		}()
		time.Sleep(time.Millisecond)
		out, err := dq.WaitBack(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if time.Since(startAt) < 10*time.Millisecond {
			t.Error("did not sleep")
		}
		if out != 100 {
			t.Error("100 !=", out)
		}
	})
	t.Run("BlockingIteratorForward", func(t *testing.T) {
		t.Parallel()
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 5; i++ {
			if err := dq.PushFront(i); err != nil {
				t.Fatal(err)
			}
		}
		startAt := time.Now()
		iter := dq.IteratorBlocking()
		seen := 0
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		t.Cleanup(cancel)
		for iter.Next(ctx) {
			seen++
			if time.Since(startAt) > 250*time.Millisecond {
				t.Error("took to long to iterate", time.Since(startAt))
			}
			t.Log(time.Since(startAt))
		}
		if time.Since(startAt) <= 500*time.Millisecond {
			t.Error("should have waited for the context to timeout", time.Since(startAt))
		}
		if ctx.Err() == nil {
			t.Error("context should have canceled")
		}
		if seen != 5 {
			t.Error("should have seen all items")
		}
	})
	t.Run("BlockingIteratorReverse", func(t *testing.T) {
		t.Parallel()
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}

		for i := 0; i < 5; i++ {
			if err := dq.PushBack(i); err != nil {
				t.Fatal(err)
			}
		}
		startAt := time.Now()
		iter := dq.IteratorBlockingReverse()
		seen := 0
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		t.Cleanup(cancel)
		for iter.Next(ctx) {
			seen++
			if time.Since(startAt) > 250*time.Millisecond {
				t.Error("took to long to iterate", time.Since(startAt))
			}
			t.Log(time.Since(startAt))
		}
		if time.Since(startAt) <= 500*time.Millisecond {
			t.Error("should have waited for the context to timeout", time.Since(startAt))
		}
		if ctx.Err() == nil {
			t.Error("context should have canceled")
		}
		if seen != 5 {
			t.Error("should have seen all items")
		}
	})

	t.Run("Closed", func(t *testing.T) {
		dq, err := NewDeque[int](DequeOptions{Capacity: 40})
		if err != nil {
			t.Fatal(err)
		}

		if err := dq.PushBack(100); err != nil {
			t.Fatal(err)
		}
		if err := dq.PushFront(100); err != nil {
			t.Fatal(err)
		}
		if l := dq.Len(); l != 2 {
			t.Fatal(l)
		}

		dq.Close()

		if l := dq.Len(); l != 2 {
			t.Fatal(l)
		}

		t.Run("Push", func(t *testing.T) {
			if err := dq.PushBack(42); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if err := dq.PushFront(42); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if err := dq.ForcePushBack(42); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if err := dq.ForcePushFront(42); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}
		})

		t.Run("Wait", func(t *testing.T) {
			if _, err := dq.WaitBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if _, err := dq.WaitBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}
		})
		t.Run("Iterator", func(t *testing.T) {
			for idx, iter := range []fun.Iterator[int]{
				dq.Iterator(),
				dq.IteratorReverse(),
				dq.IteratorBlocking(),
				dq.IteratorBlockingReverse(),
			} {
				t.Run(fmt.Sprint(idx), func(t *testing.T) {
					seen := 0
					for iter.Next(ctx) {
						seen++
					}
					if seen != 2 {
						t.Fatalf("iterator had %d and saw %d", dq.Len(), seen)
					}
				})
			}
		})
		t.Run("Internal", func(t *testing.T) {
			if _, ok := dq.pop(dq.root); ok {
				t.Fatal("can't pop the root")
			}
		})
	})
}
