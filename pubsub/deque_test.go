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

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/fnx"
	"github.com/tychoish/fun/risky"
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
	name   string
	add    func(T) error
	remove func() (T, bool)
	stream func(context.Context) iter.Seq[T]
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
				add:    cue.Add,
				remove: cue.Remove,
				stream: cue.IteratorWait,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.tracker.len,
			}
		},
		func() fixture[T] {
			cue := risky.Force(NewQueue[T](QueueOptions{HardLimit: 100, SoftQuota: 60}))

			return fixture[T]{
				name:   "QueueLimited",
				add:    cue.Add,
				remove: cue.Remove,
				stream: cue.IteratorWait,
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
				stream: cue.IteratorFront,
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
				stream: cue.IteratorFront,
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
				stream: cue.IteratorBack,
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
				stream: cue.IteratorBack,
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
				stream: cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		// simple capacity
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushFrontPopBackForward",
				add:    cue.PushFront,
				remove: cue.PopBack,
				stream: cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushBackPopFrontReverse",
				add:    cue.PushBack,
				remove: cue.PopFront,
				stream: cue.IteratorBack,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushFrontPopBackReverse",
				add:    cue.PushFront,
				remove: cue.PopBack,
				stream: cue.IteratorBack,
				elems:  makeElems(50),
				len:    cue.Len,
				close:  cue.Close,
			}
		},

		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{Capacity: 50}))

			return fixture[T]{
				name:   "DequeCapacityPushBackPopFrontForward",
				add:    cue.PushBack,
				remove: cue.PopFront,
				stream: cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},

		// bursty limited size
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushFrontPopBackForward",
				add:    cue.PushFront,
				remove: cue.PopBack,
				stream: cue.IteratorFront,
				close:  cue.Close,
				elems:  makeElems(50),
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushBackPopFrontReverse",
				add:    cue.PushBack,
				remove: cue.PopFront,
				stream: cue.IteratorBack,
				elems:  makeElems(50),
				close:  cue.Close,
				len:    cue.Len,
			}
		},
		func() fixture[T] {
			cue := risky.Force(NewDeque[T](DequeOptions{QueueOptions: &QueueOptions{HardLimit: 100, SoftQuota: 60}}))

			return fixture[T]{
				name:   "DequeBurstPushFrontPopBackReverse",
				add:    cue.PushFront,
				remove: cue.PopBack,
				stream: cue.IteratorBack,
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

			set := &dt.Set[T]{}
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
			iter := fix.stream(ctx)

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
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)

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
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)

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
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)
					fun.Invariant.Ok(tt.Push(ctx, 1) == nil)
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
		t.Run("Stream", func(t *testing.T) {
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

			if _, err := dq.WaitBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}

			if _, err := dq.WaitBack(ctx); !errors.Is(err, ErrQueueClosed) {
				t.Fatal(err)
			}
		})
		t.Run("Stream", func(t *testing.T) {
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
						t.Fatalf("stream had %d and saw %d", dq.Len(), seen)
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
	t.Run("StreamHandlesEmpty", func(t *testing.T) {
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
			assert.MinRuntime(t, 100*time.Millisecond-(10*time.Nanosecond),
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

		queue := risky.Force(NewDeque[int64](DequeOptions{Unlimited: true}))
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
		queue := risky.Force(NewDeque[func()](DequeOptions{Unlimited: true}))
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

func TestDequeIteratorPopFront(t *testing.T) {
	t.Run("PopExistingItems", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

		check.NotError(t, dq.PushBack("one"))
		check.NotError(t, dq.PushBack("two"))
		check.NotError(t, dq.PushBack("three"))

		values := make([]string, 0)
		for val := range dq.IteratorPopFront(ctx) {
			values = append(values, val)
		}

		assert.Equal(t, len(values), 3)
		assert.Equal(t, values[0], "one")
		assert.Equal(t, values[1], "two")
		assert.Equal(t, values[2], "three")
		assert.Equal(t, dq.Len(), 0)
	})

	t.Run("NonBlocking", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 5}))

		check.NotError(t, dq.PushBack(1))
		check.NotError(t, dq.PushBack(2))

		count := 0
		for range dq.IteratorPopFront(ctx) {
			count++
		}

		assert.Equal(t, count, 2)
		assert.Equal(t, dq.Len(), 0)
	})

	t.Run("EmptyDeque", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 5}))

		count := 0
		for range dq.IteratorPopFront(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("RemovesFromFront", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 10}))

		for i := 1; i <= 5; i++ {
			check.NotError(t, dq.PushBack(i))
		}

		values := make([]int, 0)
		for val := range dq.IteratorPopFront(ctx) {
			values = append(values, val)
			if len(values) >= 3 {
				break
			}
		}

		assert.Equal(t, len(values), 3)
		assert.Equal(t, values[0], 1)
		assert.Equal(t, values[1], 2)
		assert.Equal(t, values[2], 3)
		assert.Equal(t, dq.Len(), 2)

		val, ok := dq.PopFront()
		assert.True(t, ok)
		assert.Equal(t, val, 4)
	})
}

func TestDequeIteratorPopBack(t *testing.T) {
	t.Run("PopExistingItems", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

		check.NotError(t, dq.PushBack("one"))
		check.NotError(t, dq.PushBack("two"))
		check.NotError(t, dq.PushBack("three"))

		values := make([]string, 0)
		for val := range dq.IteratorPopBack(ctx) {
			values = append(values, val)
		}

		assert.Equal(t, len(values), 3)
		assert.Equal(t, values[0], "three")
		assert.Equal(t, values[1], "two")
		assert.Equal(t, values[2], "one")
		assert.Equal(t, dq.Len(), 0)
	})

	t.Run("NonBlocking", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 5}))

		check.NotError(t, dq.PushBack(1))
		check.NotError(t, dq.PushBack(2))

		count := 0
		for range dq.IteratorPopBack(ctx) {
			count++
		}

		assert.Equal(t, count, 2)
		assert.Equal(t, dq.Len(), 0)
	})

	t.Run("EmptyDeque", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 5}))

		count := 0
		for range dq.IteratorPopBack(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("RemovesFromBack", func(t *testing.T) {
		ctx := context.Background()
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 10}))

		for i := 1; i <= 5; i++ {
			check.NotError(t, dq.PushBack(i))
		}

		values := make([]int, 0)
		for val := range dq.IteratorPopBack(ctx) {
			values = append(values, val)
			if len(values) >= 3 {
				break
			}
		}

		assert.Equal(t, len(values), 3)
		assert.Equal(t, values[0], 5)
		assert.Equal(t, values[1], 4)
		assert.Equal(t, values[2], 3)
		assert.Equal(t, dq.Len(), 2)

		val, ok := dq.PopBack()
		assert.True(t, ok)
		assert.Equal(t, val, 2)
	})
}

func TestDequeLIFO(t *testing.T) {
	t.Run("BlocksOnEmptyDeque", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

		count := 0
		for range dq.LIFO(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("ConsumesItems", func(t *testing.T) {
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Add items gradually (LIFO/FIFO wait for new items between consumptions)
		go func() {
			for i := 1; i <= 3; i++ {
				time.Sleep(15 * time.Millisecond)
				check.NotError(t, dq.PushBack(i))
			}
		}()

		count := 0
		for range dq.LIFO(ctx) {
			count++
		}

		// Should have consumed items added during the window
		assert.True(t, count >= 1)
	})

	t.Run("RemovesFromBack", func(t *testing.T) {
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

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
		for val := range dq.LIFO(ctx) {
			if firstVal == "" {
				firstVal = val
			}
		}

		// Should consume "second" first (it's at the back)
		assert.Equal(t, firstVal, "second")
	})
}

func TestDequeFIFO(t *testing.T) {
	t.Run("BlocksOnEmptyDeque", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

		count := 0
		for range dq.FIFO(ctx) {
			count++
		}

		assert.Equal(t, count, 0)
	})

	t.Run("ConsumesItems", func(t *testing.T) {
		dq := risky.Force(NewDeque[int](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Add items gradually (LIFO/FIFO wait for new items between consumptions)
		go func() {
			for i := 1; i <= 3; i++ {
				time.Sleep(15 * time.Millisecond)
				check.NotError(t, dq.PushBack(i))
			}
		}()

		count := 0
		for range dq.FIFO(ctx) {
			count++
		}

		// Should have consumed items added during the window
		assert.True(t, count >= 1)
	})

	t.Run("RemovesFromFront", func(t *testing.T) {
		dq := risky.Force(NewDeque[string](DequeOptions{Capacity: 10}))

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Add both items together so they're both in the deque
		go func() {
			time.Sleep(10 * time.Millisecond)
			check.NotError(t, dq.PushBack("first"))
			check.NotError(t, dq.PushBack("second")) // Now "second" is at the back
		}()

		// Get first item - should be from front since FIFO pops from front
		var firstVal string
		for val := range dq.FIFO(ctx) {
			if firstVal == "" {
				firstVal = val
			}
		}

		// Should consume "first" first (it's at the front)
		assert.Equal(t, firstVal, "first")
	})
}
