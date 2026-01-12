package pubsub

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/fnx"
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

	front := &waitPushCase{Name: "Front", dq: erc.Must(NewDeque[int](DequeOptions{Capacity: 2}))}
	front.Push = front.dq.WaitPushFront
	front.Pop = front.dq.PopBack
	cases = append(cases, front)

	back := &waitPushCase{Name: "Back", dq: erc.Must(NewDeque[int](DequeOptions{Capacity: 2}))}
	back.Push = back.dq.WaitPushBack
	back.Pop = back.dq.PopFront
	cases = append(cases, back)

	return cases
}

type fixture[T any] struct {
	name   string
	add    func(T) error
	remove func() (T, bool)
	seq    func(context.Context) iter.Seq[T]
	close  func() error
	len    func() int
	elems  []T
}

func randomStringSlice(size int) []string {
	elems := make([]string, size)

	for i := 0; i < size; i++ {
		elems[i] = fmt.Sprintf("value=%d", i)
	}
	return elems
}

func generateDequeFixtures[T any](makeElems func(int) []T) []func() fixture[T] {
	return []func() fixture[T]{
		func() fixture[T] {
			cue := NewUnlimitedQueue[T]()

			return fixture[T]{
				name:   "QueueUnlimited",
				add:    cue.Push,
				remove: cue.Pop,
				seq:    cue.IteratorWait,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.tracker.len,
			}
		},
		func() fixture[T] {
			cue := erc.Must(NewQueue[T](QueueOptions{HardLimit: 100, SoftQuota: 60}))

			return fixture[T]{
				name:   "QueueLimited",
				add:    cue.Push,
				remove: cue.Pop,
				seq:    cue.IteratorWait,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.tracker.len,
			}
		},
		func() fixture[T] {
			cue := NewUnlimitedDeque[T]()

			return fixture[T]{
				name:   "DequePushBackPopFrontForward",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := NewUnlimitedDeque[T]()

			return fixture[T]{
				name:   "DequePushFrontPopBackForward",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := NewUnlimitedDeque[T]()

			return fixture[T]{
				name:   "DequePushBackPopFrontReverse",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := NewUnlimitedDeque[T]()

			return fixture[T]{
				name:   "DequePushFrontPopBackReverse",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				len:    cue.Len,
				close:  cue.Close,
			}
		},
		func() fixture[T] {
			cue := NewUnlimitedDeque[T]()

			return fixture[T]{
				name:   "DequePushBackPopFrontForward",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		// simple capacity
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushFrontPopBackForward",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushBackPopFrontReverse",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushFrontPopBackReverse",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				len:    cue.Len,
				close:  cue.Close,
			}
		},

		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushBackPopFrontForward",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},

		// bursty limited size
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushFrontPopBackForward",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushBackPopFrontReverse",
				add:    cue.PushBack,
				remove: cue.PopFront,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := erc.Must(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushFrontPopBackReverse",
				add:    cue.PushFront,
				remove: cue.PopBack,
				seq:    cue.IteratorBack,
				elems:  makeElems(50),
				len:    cue.Len,
				close:  cue.Close,
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

			set := map[T]bool{}
			for i := len(fix.elems); i > 0; i-- {
				out, ok := fix.remove()
				if !ok {
					t.Error("remove should not fail", i)
				}
				set[out] = true
			}
			if len(set) != len(fix.elems) {
				t.Fatal("did not see all expected results", set.Len(), len(fix.elems))
			}

			if fix.len() != 0 {
				t.Error("remove did not work")
			}
		})
		t.Run("Iterate", func(t *testing.T) {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Second)
			defer cancel()

			fix := f()
			for _, e := range fix.elems {
				if err := fix.add(e); err != nil {
					t.Fatal(err)
				}
			}

			assert.Equal(t, fix.len(), len(fix.elems))
			seen := 0
			iter := fix.seq(ctx)

			if err := fix.close(); err != nil {
				t.Fatal(err)
			}

			for value := range iter {
				seen++
				t.Logf("%d: %T", seen, value)
				assert.NotZero(t, value)
			}
			if seen != len(fix.elems) {
				t.Fatal("did not iterate far enough", seen, len(fix.elems))
			}
		})
	})
}

func TestDeque(t *testing.T) {
	t.Parallel()
	t.Run("String", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)
		for _, f := range generateDequeFixtures(randomStringSlice) {
			RunDequeTests(ctx, t, f)
		}
	})
	t.Run("Integer", func(t *testing.T) {
		t.Parallel()
		ctx := testt.Context(t)
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
					ctx := testt.Context(t)
					assert.True(t, tt.Push(ctx, 1) == nil)
					assert.True(t, tt.Push(ctx, 1) == nil)

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
					ctx := testt.Context(t)
					assert.True(t, tt.Push(ctx, 1) == nil)
					assert.True(t, tt.Push(ctx, 1) == nil)

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
					ctx := testt.Context(t)
					assert.True(t, tt.Push(ctx, 1) == nil)
					assert.True(t, tt.Push(ctx, 1) == nil)
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
			for idx, iter := range []iter.Seq[int]{
				dq.IteratorFront(t.Context()),
				dq.IteratorBack(t.Context()),
			} {
				t.Run(fmt.Sprint(idx), func(t *testing.T) {
					for range iter {
						t.Error("should not iterate", idx)
					}
				})
			}
		})
	})
	t.Run("WaitingBack", func(t *testing.T) {
		t.Parallel()
		ctx := testt.ContextWithTimeout(t, 500*time.Millisecond)
		dq, err := NewDeque[int](DequeOptions{Capacity: 2})
		if err != nil {
			t.Fatal(err)
		}
		startAt := time.Now()
		go func() {
			time.Sleep(10 * time.Millisecond)
			if perr := dq.PushBack(100); perr != nil {
				t.Error(err)
			}
		}()
		runtime.Gosched()
		out, err := dq.WaitPopBack(ctx)
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
		ctx := testt.ContextWithTimeout(t, 500*time.Millisecond)
		dq, err := NewDeque[int](DequeOptions{Capacity: 10})
		if err != nil {
			t.Fatal(err)
		}
		startAt := time.Now()
		go func() {
			time.Sleep(10 * time.Millisecond)
			if perr := dq.PushFront(100); perr != nil {
				t.Error(err)
			}
		}()
		time.Sleep(time.Millisecond)
		out, err := dq.WaitPopBack(ctx)
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

		if err := dq.Close(); err != nil {
			t.Fatal(err)
		}

		if dq.Len() != 2 {
			t.Fatal(2, "!=", dq.Len())
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
			ctx := testt.Context(t)

			if _, err := dq.WaitPopBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if _, err := dq.WaitPopBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}
		})
		t.Run("Iterators", func(t *testing.T) {
			for idx, iter := range []iter.Seq[int]{
				dq.IteratorFront(t.Context()),
				dq.IteratorBack(t.Context()),
			} {
				t.Run(fmt.Sprint(idx), func(t *testing.T) {
					seen := 0
					for range iter {
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
	t.Run("IteratorHandlesEmpty", func(t *testing.T) {
		t.Parallel()
		queue := NewUnlimitedDeque[int]()

		sa := time.Now()

		count := 0
		for range queue.IteratorFront(t.Context()) {
			count++
		}
		if count != 0 {
			t.Error("(first) should have reported item", time.Since(sa), queue.Len())
		}

		time.Sleep(2 * time.Millisecond)
		t.Log("push", queue.PushFront(31), queue.Len())

		// new itertor should fined item
		count = 0
		for range queue.IteratorFront(t.Context()) {
			count++
		}
		if count != 1 {
			t.Error("(second) should have reported item", time.Since(sa), queue.Len())
		}
	})
	t.Run("Future", func(t *testing.T) {
		t.Parallel()
		t.Run("BlockingEmpty", func(t *testing.T) {
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			assert.MinRuntime(t, 99*time.Millisecond,
				func() {
					dq := NewUnlimitedDeque[string]()
					for range dq.IteratorWaitFront(ctx) {
						continue
					}
				})
			assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		})
		t.Run("ReverseBlockingEmpty", func(t *testing.T) {
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			assert.MinRuntime(t, 100*time.Millisecond-(50*time.Nanosecond),
				func() {
					dq := NewUnlimitedDeque[string]()
					for range dq.IteratorWaitBack(ctx) {
						continue
					}
				})
			assert.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
		})
		t.Run("Production", func(t *testing.T) {
			ctx := testt.ContextWithTimeout(t, 100*time.Millisecond)
			dq := NewUnlimitedDeque[string]()
			assert.MaxRuntime(t, 128*time.Millisecond, func() {
				assert.MinRuntime(t, 16*time.Millisecond, func() {
					go func() {
						select {
						case <-ctx.Done():
							if errors.Is(ctx.Err(), context.DeadlineExceeded) {
								t.Error("should not reach timeout")
							}
						case <-time.After(10 * time.Millisecond):
							check.NotError(t, dq.PushBack("hello!"))
						}
					}()

					for val := range dq.IteratorWaitFront(ctx) {
						check.Equal(t, val, "hello!")
					}
				})
			})
		})
	})
}

func TestDequeIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Integers", func(t *testing.T) {
		const num = 100

		t.Parallel()

		queue := erc.Must(NewDeque[int64](DequeOptions{Unlimited: true}))
		ctx := testt.Context(t)
		counter := &atomic.Int64{}
		input := &atomic.Int64{}

		wg := &fnx.WaitGroup{}
		signal := fnx.Operation(wg.Wait).Worker().Signal(ctx)

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
		go func(iter iter.Seq[int64]) {
			defer wg.Done()
			runtime.Gosched()
			for range iter {
				counter.Add(1)
				runtime.Gosched()
			}
		}(queue.IteratorFront(ctx))

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
	t.Run("FutureConsumer", func(t *testing.T) {
		t.Parallel()
		ctx := testt.ContextWithTimeout(t, time.Second)
		queue := erc.Must(NewDeque[func()](DequeOptions{Unlimited: true}))
		sent := &atomic.Int64{}
		recv := &atomic.Int64{}

		wg := &fnx.WaitGroup{}
		const (
			factor = 2
			worker = 32
			num    = factor * worker
		)
		wwg := &fnx.WaitGroup{}
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
				go func(iter iter.Seq[func()]) {
					defer wwg.Done()

					for op := range iter {
						op()
						recv.Add(1)
					}
				}(queue.IteratorFront(ctx))
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

func TestDequeLIFO(t *testing.T) {
	t.Parallel()
	t.Run("BlocksOnEmptyDeque", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		dq := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		count := 0
		for range dq.IteratorWaitPopBack(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("ConsumesItems", func(t *testing.T) {
		dq := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Add items gradually (LIFO/FIFO - wait for new items between consumptions)
		go func() {
			for i := 1; i <= 3; i++ {
				time.Sleep(15 * time.Millisecond)
				check.NotError(t, dq.PushBack(i))
			}
		}()

		count := 0
		for range dq.IteratorWaitPopBack(ctx) {
			count++
		}

		// Should have consumed items added during the window
		assert.True(t, count >= 1)
	})

	t.Run("RemovesFromBack", func(t *testing.T) {
		dq := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Add both items together so they're both in the deque
		go func() {
			time.Sleep(10 * time.Millisecond)
			check.NotError(t, dq.PushBack("first"))
			check.NotError(t, dq.PushBack("second")) // Now "second" is at the back
		}()

		// Get first item - should be from back since LIFO pops from back
		var firstVal string
		for val := range dq.IteratorWaitPopBack(ctx) {
			if firstVal == "" {
				firstVal = val
			}
		}

		// Should consume "second" first (it's at the back)
		assert.Equal(t, firstVal, "second")
	})
}

func TestDequeFIFO(t *testing.T) {
	t.Parallel()
	t.Run("BlocksOnEmptyDeque", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		dq := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		count := 0
		for range dq.IteratorWaitPopFront(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("ConsumesItems", func(t *testing.T) {
		dq := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Add items gradually (LIFO/IteratorWaitPopFront wait for new items between consumptions)
		go func() {
			for i := 1; i <= 3; i++ {
				time.Sleep(15 * time.Millisecond)
				check.NotError(t, dq.PushBack(i))
			}
		}()

		count := 0
		for range dq.IteratorWaitPopFront(ctx) {
			count++
		}

		// Should have consumed items added during the window
		assert.True(t, count >= 1)
	})

	t.Run("RemovesFromFront", func(t *testing.T) {
		dq := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Add both items together so they're both in the deque
		go func() {
			time.Sleep(10 * time.Millisecond)
			check.NotError(t, dq.PushBack("first"))
			check.NotError(t, dq.PushBack("second")) // Now "second" is at the back
		}()

		// Get first item - should be from front since IteratorWaitPopFront pops from front
		var firstVal string
		for val := range dq.IteratorWaitPopFront(ctx) {
			if firstVal == "" {
				firstVal = val
			}
		}

		// Should consume "first" first (it's at the front)
		assert.Equal(t, firstVal, "first")
	})
}

func TestDequeDrain(t *testing.T) {
	t.Run("EmptyDequeDrainsImmediately", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		err := deque.Drain(ctx)
		assert.NotError(t, err)
		assert.Equal(t, 0, deque.Len())
	})

	t.Run("DrainsQueuedItems", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		// Add items
		for i := 0; i < 5; i++ {
			check.NotError(t, deque.PushBack(fmt.Sprintf("item-%d", i)))
		}

		assert.Equal(t, 5, deque.Len())

		// Start draining in background
		drainComplete := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainComplete <- err:
			case <-ctx.Done():
			}
		}()

		// Drain should be waiting
		time.Sleep(50 * time.Millisecond)
		select {
		case <-drainComplete:
			t.Fatal("drain should not have completed yet")
		default:
		}

		// Consume items
		for i := 0; i < 5; i++ {
			val, ok := deque.PopFront()
			check.True(t, ok)
			check.Equal(t, fmt.Sprintf("item-%d", i), val)
		}

		// Drain should complete
		err := <-drainComplete
		assert.NotError(t, err)
		assert.Equal(t, 0, deque.Len())
	})

	t.Run("PreventsNewAddsWhileDraining", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		// Add some items
		for i := 0; i < 3; i++ {
			check.NotError(t, deque.PushBack(fmt.Sprintf("item-%d", i)))
		}

		// Start draining
		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Give drain time to start
		time.Sleep(50 * time.Millisecond)

		// Try to add - should fail with ErrQueueDraining
		err := deque.PushFront("blocked-front")
		assert.ErrorIs(t, err, ErrQueueDraining)

		err = deque.PushBack("blocked-back")
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Consume all items
		for deque.Len() > 0 {
			_, ok := deque.PopFront()
			check.True(t, ok)
		}

		// Drain should complete
		err = <-drainErr
		assert.NotError(t, err)

		// After drain completes, should be able to add again
		check.NotError(t, deque.PushBack("new-item"))
	})

	t.Run("DequeNotClosedAfterDrain", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		// Add items
		for i := 0; i < 3; i++ {
			check.NotError(t, deque.PushBack(fmt.Sprintf("item-%d", i)))
		}

		// Start draining in background
		drainComplete := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainComplete <- err:
			case <-ctx.Done():
			}
		}()

		// Consume items
		for deque.Len() > 0 {
			time.Sleep(10 * time.Millisecond)
			_, ok := deque.PopFront()
			check.True(t, ok)
		}

		// Wait for drain to complete
		err := <-drainComplete
		assert.NotError(t, err)

		// Deque should not be closed - can still add items
		check.NotError(t, deque.PushBack("after-drain"))
		val, ok := deque.PopFront()
		check.True(t, ok)
		check.Equal(t, "after-drain", val)
	})

	t.Run("ContextCancellationDuringDrain", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		// Add items that won't be consumed
		for i := 0; i < 5; i++ {
			check.NotError(t, deque.PushBack(i))
		}

		drainCtx, drainCancel := context.WithCancel(ctx)

		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(drainCtx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Give drain time to start waiting
		time.Sleep(50 * time.Millisecond)

		// Cancel the drain context
		drainCancel()

		// Drain should return with error
		err := <-drainErr
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))

		// Items should still be in deque
		assert.Equal(t, 5, deque.Len())
	})

	t.Run("ConcurrentDrainAndConsume", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 100}))

		// Add many items
		for i := 0; i < 50; i++ {
			check.NotError(t, deque.PushBack(i))
		}

		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Consume items concurrently
		go func() {
			for deque.Len() > 0 {
				time.Sleep(5 * time.Millisecond)
				_, _ = deque.PopFront()
			}
		}()

		// Drain should complete once all items consumed
		err := <-drainErr
		assert.NotError(t, err)
		assert.Equal(t, 0, deque.Len())
	})

	t.Run("MultipleDrainCalls", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		// First drain on empty deque
		err := deque.Drain(ctx)
		assert.NotError(t, err)

		// Add items
		for i := 0; i < 3; i++ {
			check.NotError(t, deque.PushBack(i))
		}

		// Second drain
		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Consume items
		for deque.Len() > 0 {
			time.Sleep(10 * time.Millisecond)
			_, _ = deque.PopFront()
		}

		err = <-drainErr
		assert.NotError(t, err)

		// Third drain on empty deque
		err = deque.Drain(ctx)
		assert.NotError(t, err)
	})

	t.Run("DrainWithPopBack", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		// Add items
		for i := 0; i < 3; i++ {
			check.NotError(t, deque.PushBack(fmt.Sprintf("item-%d", i)))
		}

		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Use PopBack to consume
		for deque.Len() > 0 {
			val, ok := deque.PopBack()
			check.True(t, ok)
			check.True(t, val != "")
		}

		// Drain should complete
		err := <-drainErr
		assert.NotError(t, err)
		assert.Equal(t, 0, deque.Len())
	})

	t.Run("WaitPushReceivesDrainingErrorWhileWaiting", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		// Create deque with capacity of 2
		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 2}))

		// Fill the deque to capacity
		check.NotError(t, deque.PushBack(1))
		check.NotError(t, deque.PushBack(2))
		assert.Equal(t, 2, deque.Len())

		// Start a goroutine that tries to WaitPushBack (will block because deque is full)
		waitPushErr := make(chan error, 1)
		go func() {
			err := deque.WaitPushBack(ctx, 100)
			select {
			case waitPushErr <- err:
			case <-ctx.Done():
			}
		}()

		// Give WaitPushBack time to block
		time.Sleep(100 * time.Millisecond)

		// Verify WaitPushBack hasn't completed yet
		select {
		case <-waitPushErr:
			t.Fatal("WaitPushBack should still be waiting")
		default:
		}

		// Now start draining the deque
		drainErr := make(chan error, 1)
		go func() {
			err := deque.Drain(ctx)
			select {
			case drainErr <- err:
			case <-ctx.Done():
			}
		}()

		// Give drain time to start
		time.Sleep(50 * time.Millisecond)

		// The blocked WaitPushBack should now receive ErrQueueDraining
		err := <-waitPushErr
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Verify item was NOT added to the deque
		assert.Equal(t, 2, deque.Len())

		// Consume all items to allow drain to complete
		for deque.Len() > 0 {
			_, ok := deque.PopFront()
			check.True(t, ok)
		}

		// Drain should complete successfully
		err = <-drainErr
		assert.NotError(t, err)
		assert.Equal(t, 0, deque.Len())
	})
}

func TestDequeShutdown(t *testing.T) {
	t.Run("ShutdownDrainsAndCloses", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		// Add items
		for i := 0; i < 3; i++ {
			check.NotError(t, deque.PushBack(i))
		}

		// Shutdown in background
		shutdownDone := make(chan error, 1)
		go func() {
			err := deque.Shutdown(ctx)
			select {
			case shutdownDone <- err:
			case <-ctx.Done():
			}
		}()

		// Give shutdown time to start draining
		time.Sleep(50 * time.Millisecond)

		// Try to add - should fail with ErrQueueDraining
		err := deque.PushBack(100)
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Consume items
		for deque.Len() > 0 {
			_, ok := deque.PopFront()
			check.True(t, ok)
		}

		// Wait for shutdown to complete
		err = <-shutdownDone
		assert.NotError(t, err)

		// Deque should be closed now
		err = deque.PushBack(200)
		assert.ErrorIs(t, err, ErrQueueClosed)
	})

	t.Run("ShutdownEmptyDeque", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[string](DequeOptions{Capacity: 10}))

		// Shutdown empty deque
		err := deque.Shutdown(ctx)
		assert.NotError(t, err)

		// Should be closed
		err = deque.PushFront("item")
		assert.ErrorIs(t, err, ErrQueueClosed)
	})

	t.Run("ShutdownWithContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		// Add items that won't be consumed
		for i := 0; i < 5; i++ {
			check.NotError(t, deque.PushBack(i))
		}

		shutdownCtx, shutdownCancel := context.WithCancel(ctx)

		shutdownErr := make(chan error, 1)
		go func() {
			err := deque.Shutdown(shutdownCtx)
			select {
			case shutdownErr <- err:
			case <-ctx.Done():
			}
		}()

		// Give shutdown time to start
		time.Sleep(50 * time.Millisecond)

		// Cancel shutdown context
		shutdownCancel()

		// Shutdown should fail
		err := <-shutdownErr
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))

		// Items should still be in deque
		assert.Equal(t, 5, deque.Len())
	})

	t.Run("ShutdownBlocksWaitPush", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
		defer cancel()

		deque := erc.Must(NewDeque[int](DequeOptions{Capacity: 10}))

		// Add items to deque
		check.NotError(t, deque.PushBack(1))
		check.NotError(t, deque.PushBack(2))
		check.NotError(t, deque.PushBack(3))

		// Start shutdown
		shutdownDone := make(chan error, 1)
		go func() {
			err := deque.Shutdown(ctx)
			select {
			case shutdownDone <- err:
			case <-ctx.Done():
			}
		}()

		// Give shutdown time to start draining
		time.Sleep(50 * time.Millisecond)

		// Try WaitPushBack - should fail immediately with ErrQueueDraining
		err := deque.WaitPushBack(ctx, 100)
		assert.ErrorIs(t, err, ErrQueueDraining)

		// Consume remaining items
		for deque.Len() > 0 {
			_, _ = deque.PopFront()
		}

		// Wait for shutdown
		err = <-shutdownDone
		assert.NotError(t, err)
	})
}
