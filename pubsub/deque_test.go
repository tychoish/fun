package pubsub

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/risky"
	"github.com/tychoish/fun/set"
	"github.com/tychoish/fun/testt"
)

type waitPushCase struct {
	Name string
	Push func(context.Context, int) error
	Pop  func() (int, bool)
	dq   *Deque[int]
}

func (tt *waitPushCase) check(t *testing.T) {
	t.Helper()
	if tt.dq.Len() != 0 {
		t.Error(tt.dq.Len())
	}
}

func makeWaitPushCases() []*waitPushCase {
	var cases []*waitPushCase

	front := &waitPushCase{Name: "Front", dq: risky.Force(NewDeque[int](DequeOptions{Capacity: 2}))}
	front.Push = front.dq.WaitPushFront
	front.Pop = front.dq.PopBack
	cases = append(cases, front)

	back := &waitPushCase{Name: "Back", dq: risky.Force(NewDeque[int](DequeOptions{Capacity: 2}))}
	back.Push = back.dq.WaitPushBack
	back.Pop = back.dq.PopFront
	cases = append(cases, back)

	return cases
}

type fixture[T any] struct {
	name     string
	add      func(T) error
	remove   func() (T, bool)
	iterator func() *fun.Iterator[T]
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
			cue := risky.Force(NewQueue[T](QueueOptions{HardLimit: 100, SoftQuota: 60}))

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
			cue := NewUnlimitedDeque[T]()

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
			cue := NewUnlimitedDeque[T]()

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
			cue := NewUnlimitedDeque[T]()

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
			cue := NewUnlimitedDeque[T]()

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
			cue := NewUnlimitedDeque[T]()

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
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

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
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

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
			t.Parallel()
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
			t.Parallel()
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			fix := f()
			for _, e := range fix.elems {
				if err := fix.add(e); err != nil {
					t.Fatal(err)
				}
			}

			assert.Equal(t, fix.len(), len(fix.elems))
			seen := 0
			iter := fix.iterator()

			if err := fix.close(); err != nil {
				t.Fatal(err)
			}

			for iter.Next(ctx) {
				seen++
				t.Logf("%d: %T", seen, iter.Value())
				assert.NotZero(t, iter.Value())
			}
			if seen != len(fix.elems) {
				t.Fatal("did not iterate far enough", seen, len(fix.elems))
			}

			if err := ctx.Err(); err != nil {
				t.Error("shouldn't cancel", err)
			}
			if err := iter.Close(); err != nil {
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
		t.Parallel()
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
			if err := conf.Validate(); err != nil {
				t.Fatal()
			}
			if _, err := NewDeque[string](conf); err != nil {
				t.Fatal()
			}
		})
		t.Run("Zero", func(t *testing.T) {
			conf := DequeOptions{}
			if err := conf.Validate(); err != nil {
				t.Fatal("validate", err)
			}
			if _, err := NewDeque[string](conf); err != nil {
				t.Fatal("create", err)
			}
		})
		t.Run("ConflictingOptionsUnlimited", func(t *testing.T) {
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
		t.Run("ConflictingOptionsQueue", func(t *testing.T) {
			conf := DequeOptions{
				Capacity:     100,
				QueueOptions: &QueueOptions{HardLimit: 50, SoftQuota: 24},
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
	t.Run("WaitPush", func(t *testing.T) {
		t.Parallel()
		t.Run("ContextCanceled", func(t *testing.T) {
			for _, tt := range makeWaitPushCases() {
				t.Run(tt.Name, func(t *testing.T) {
					tt.check(t)
					fun.Invariant(tt.Push(ctx, 1) == nil)
					fun.Invariant(tt.Push(ctx, 1) == nil)

					canceled, trigger := context.WithCancel(context.Background())
					trigger()

					err := tt.Push(canceled, 4)
					if err == nil {
						t.Fatal("should be error")
					}
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				})
			}
		})
		t.Run("Closed", func(t *testing.T) {
			for _, tt := range makeWaitPushCases() {
				t.Run(tt.Name, func(t *testing.T) {
					tt.check(t)
					fun.Invariant(tt.Push(ctx, 1) == nil)
					fun.Invariant(tt.Push(ctx, 1) == nil)

					if err := tt.dq.Close(); err != nil {
						t.Fatal(err)
					}

					err := tt.Push(ctx, 4)
					if err == nil {
						t.Fatal("should be error")
					}
					if !errors.Is(err, ErrQueueClosed) {
						t.Fatal(err)
					}
				})
			}
		})
		t.Run("RealWait", func(t *testing.T) {
			t.Parallel()
			for _, tt := range makeWaitPushCases() {
				t.Run(tt.Name, func(t *testing.T) {
					tt.check(t)
					fun.Invariant(tt.Push(ctx, 1) == nil)
					fun.Invariant(tt.Push(ctx, 1) == nil)
					start := time.Now()
					sig := make(chan struct{})
					var end time.Time
					go func() {
						defer close(sig)
						defer func() { end = time.Now() }()
						if err := tt.Push(ctx, 42); err != nil {
							t.Error(err)
						}
					}()
					time.Sleep(100 * time.Millisecond)
					sig2 := make(chan struct{})
					go func() {
						defer close(sig2)
						one, ok := tt.Pop()
						if !ok {
							t.Error("should have popped")
						}
						if one != 1 {
							t.Error(one)
						}
						one, ok = tt.Pop()
						if !ok {
							t.Error("should have popped")
						}
						if one != 1 {
							t.Error(one)
						}
					}()

					time.Sleep(100 * time.Millisecond)
					select {
					case <-sig2:
					case <-time.After(10 * time.Millisecond):
						t.Error("should not have timed out")
					}
					select {
					case <-time.After(10 * time.Millisecond):
						t.Error("should not have timed out")
					case <-sig:
						out, ok := tt.Pop()
						if !ok {
							t.Error("should have popped")
						}
						if out != 42 {
							t.Error(out)
						}
						if time.Since(end) < 20*time.Millisecond {
							t.Error(time.Since(end))
						}
						if time.Since(start)-time.Since(end) < 100*time.Millisecond {
							t.Error(time.Since(end) - time.Since(start))

						}
						return
					}
				})
			}
		})
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
			for idx, iter := range []fun.Iterable[int]{
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
		t.Parallel()
		ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		dq, err := NewDeque[int](DequeOptions{Capacity: 2})
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
		runtime.Gosched()
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
		t.Parallel()
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

	t.Run("Closed", func(t *testing.T) {
		t.Parallel()
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
		if dq.Len() != 2 {
			t.Fatal(dq.Len())
		}

		dq.Close()

		if dq.Len() != 2 {
			t.Fatal(dq.Len())
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
			for idx, iter := range []fun.Iterable[int]{
				dq.Iterator(),
				dq.IteratorReverse(),
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
		t.Run("IteratorClosed", func(t *testing.T) {
			for idx, iter := range []fun.Iterable[int]{
				dq.Iterator(),
				dq.IteratorReverse(),
			} {
				t.Run(fmt.Sprint(idx), func(t *testing.T) {
					seen := 0
					if err := iter.Close(); err != nil {
						t.Fatal(err)
					}
					for iter.Next(ctx) {
						seen++
					}
					if seen != 0 {
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
	t.Run("IteratorHandlesEmpty", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		queue := NewUnlimitedDeque[int]()

		toctx, toccancel := context.WithTimeout(ctx, 4*time.Millisecond)
		defer toccancel()
		sa := time.Now()
		iter := queue.Iterator()
		if iter.Next(toctx) {
			t.Error("should have reported false early", time.Since(sa))
		}
		// iterator is empty, becomes closed

		time.Sleep(2 * time.Millisecond)
		t.Log(queue.PushFront(31), queue.Len())

		// new itertor should fined item
		iter = queue.Iterator()
		if !iter.Next(ctx) {
			t.Error("should have reported item", time.Since(sa), iter.Value(), queue.Len())
		}
		t.Log(iter.Close(), queue.Len())
	})
	t.Run("Producer", func(t *testing.T) {
		t.Run("BlockingEmpty", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			dq := NewUnlimitedDeque[string]()
			_, err := dq.ProducerBlocking().Run(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		})
		t.Run("ReverseBlockingEmpty", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			dq := NewUnlimitedDeque[string]()
			_, err := dq.ProducerReverseBlocking().Run(ctx)
			assert.Error(t, err)
			assert.ErrorIs(t, err, context.DeadlineExceeded)
		})
		t.Run("Production", func(t *testing.T) {
			t.Parallel()
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			dq := NewUnlimitedDeque[string]()
			start := time.Now()
			go func() {
				select {
				case <-ctx.Done():
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						t.Error("should not reach timeout")
					}
				case <-time.After(10 * time.Millisecond):
					dq.PushBack("hello!")
				}
			}()

			val, err := dq.ProducerBlocking().Run(ctx)
			check.NotError(t, err)
			check.Equal(t, val, "hello!")

			dur := time.Since(start)

			if dur < 10*time.Millisecond || dur > 25*time.Millisecond {
				t.Error("duration out of bounds", dur)
			}
		})
	})
}

func TestDequeIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Integers", func(t *testing.T) {
		const num = 100

		t.Parallel()

		queue := risky.Force(NewDeque[int64](DequeOptions{Unlimited: true}))
		ctx := testt.Context(t)
		counter := &atomic.Int64{}
		input := &atomic.Int64{}

		wg := &fun.WaitGroup{}
		signal := fun.Operation(wg.Wait).Worker().Signal(ctx)

		for i := 0; i < num; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				check.NotError(t, queue.PushBack(1))
				input.Add(1)
			}()
			if i&5 == 0 {
				runtime.Gosched()
			}
		}
		time.Sleep(100 * time.Millisecond)
		runtime.Gosched()
		wg.Add(1)
		go func(iter fun.Iterable[int64]) {
			defer wg.Done()
			runtime.Gosched()
			for iter.Next(ctx) {
				counter.Add(1)
				runtime.Gosched()
			}
		}(queue.Iterator())

		ticker := testt.Ticker(t, 50*time.Millisecond)
		timeout := testt.Timer(t, 1000*time.Millisecond)

	WAIT:
		for {
			select {
			case <-signal:
				t.Log("wait group returned", input.Load(), counter.Load(), wg.Num())
				break WAIT
			case <-ticker.C:
				cur := input.Load()
				assert.True(t, cur <= num)
				seen := counter.Load()
				assert.True(t, seen <= num)
				if cur == num && seen == num {
					assert.NotError(t, queue.Close())
					break WAIT
				}
			case <-timeout.C:
				assert.NotError(t, queue.Close())
				if wg.Num() == 0 {
					t.Log("hit timeout, but workers returned")
					break WAIT
				}
				t.Error("should complete input before timeout", wg.Num())
			}
		}

		wg.Wait(ctx)

		check.Equal(t, 100, input.Load())
		check.Equal(t, 100, counter.Load())
		testt.Logf(t, "counter=%d, input=%d", counter.Load(), input.Load())
	})
	t.Run("ProducerConsumer", func(t *testing.T) {
		t.Parallel()
		ctx := testt.ContextWithTimeout(t, time.Second)
		queue := risky.Force(NewDeque[func()](DequeOptions{Unlimited: true}))
		sent := &atomic.Int64{}
		recv := &atomic.Int64{}

		wg := &fun.WaitGroup{}
		const (
			factor = 2
			worker = 32
			num    = factor * worker
		)
		wwg := &fun.WaitGroup{}
		for i := 0; i < factor; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < num; i++ {
					assert.NotError(t, queue.PushBack(func() { sent.Add(1) }))
					time.Sleep(time.Millisecond)
				}
				time.Sleep(10 * time.Millisecond)
			}()
			for i := 0; i < worker; i++ {
				wwg.Add(1)
				go func(iter fun.Iterable[func()]) {
					defer wwg.Done()

					for iter.Next(ctx) {
						iter.Value()()
						recv.Add(1)
					}
				}(queue.Iterator())
			}
		}

		wg.Wait(ctx)
		assert.NotError(t, ctx.Err())
		assert.NotError(t, queue.Close())
		wwg.Wait(ctx)
		assert.NotError(t, ctx.Err())
		assert.Equal(t, sent.Load(), recv.Load())
	})
}
