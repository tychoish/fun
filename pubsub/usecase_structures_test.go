package pubsub

import (
	"context"
	"errors"
	"iter"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

func TestUseCaseContainerCapacityBoundaries(t *testing.T) {
	for name, mk := range ucBoxes(t, 1) {
		t.Run(name+"/One", func(t *testing.T) {
			b := mk()
			if err := b.Push(1); err != nil {
				t.Fatal(err)
			}
			if err := b.Push(2); !errors.Is(err, ErrQueueFull) {
				t.Fatalf("second push into capacity 1: %v", err)
			}
			if v, ok := b.Pop(); !ok || v != 1 {
				t.Fatalf("got %d, %v", v, ok)
			}
			if _, ok := b.Pop(); ok {
				t.Fatal("pop of an empty container succeeded")
			}
			if err := b.Push(3); err != nil {
				t.Fatalf("push after making room: %v", err)
			}
		})
	}
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name+"/Unbounded", func(t *testing.T) {
			b := mk()
			const n = 20000
			for i := range n {
				if err := b.Push(i); err != nil {
					t.Fatal(err)
				}
			}
			if b.Len() != n {
				t.Fatalf("len %d", b.Len())
			}
			for i := range n {
				if v, ok := b.Pop(); !ok || v != i {
					t.Fatalf("pop %d: got %d, %v", i, v, ok)
				}
			}
		})
	}
	t.Run("InvalidQueueOptions", func(t *testing.T) {
		for _, opts := range []QueueOptions{{HardLimit: 0}, {HardLimit: -1}, {HardLimit: 2, SoftQuota: 3}, {HardLimit: 2, BurstCredit: -1}} {
			if q, err := NewQueue[int](opts); err == nil || q != nil {
				t.Fatalf("%+v accepted", opts)
			}
		}
	})
	t.Run("InvalidDequeOptions", func(t *testing.T) {
		for _, opts := range []DequeOptions{
			{Capacity: -1},
			{Unlimited: true, Capacity: 1},
			{Capacity: 1, QueueOptions: &QueueOptions{HardLimit: 1}},
			{QueueOptions: &QueueOptions{HardLimit: 0}},
		} {
			if dq, err := NewDeque[int](opts); err == nil || dq != nil {
				t.Fatalf("%+v accepted", opts)
			}
		}
	})
}

func TestUseCaseContainerBlockedPushersAllLand(t *testing.T) {
	for name, mk := range ucBoxes(t, 1) {
		t.Run(name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			const producers, per = 4, 25
			var wg sync.WaitGroup
			for p := range producers {
				wg.Go(func() {
					for i := range per {
						if err := b.WaitPush(t.Context(), p*1000+i); err != nil {
							t.Errorf("producer %d: %v", p, err)
							return
						}
					}
				})
			}
			last := make([]int, producers)
			for i := range last {
				last[i] = -1
			}
			for range producers * per {
				v, err := b.WaitPop(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				p, i := v/1000, v%1000
				if i != last[p]+1 {
					t.Fatalf("producer %d: item %d after %d (per-producer order lost)", p, i, last[p])
				}
				last[p] = i
			}
			ucGuard(t, "producers", wg.Wait)
			if b.Len() != 0 {
				t.Fatalf("len %d", b.Len())
			}
		})
	}
}

func TestUseCaseContainerCompetingConsumersSeeEachItemOnce(t *testing.T) {
	for name, mk := range ucBoxes(t, 0) {
		t.Run(name, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			b := mk()
			const consumers, items = 5, 500
			var mu sync.Mutex
			var seen []int
			var wg sync.WaitGroup
			for range consumers {
				wg.Go(func() {
					for v := range b.Popping(t.Context()) {
						mu.Lock()
						seen = append(seen, v)
						mu.Unlock()
					}
				})
			}
			for i := range items {
				if err := b.Push(i); err != nil {
					t.Fatal(err)
				}
			}
			// Shutdown drains, then closes, which ends every iterator.
			ucGuard(t, "Shutdown", func() {
				if err := b.Shutdown(t.Context()); err != nil {
					t.Error(err)
				}
			})
			ucGuard(t, "consumers", wg.Wait)
			slices.Sort(seen)
			if len(seen) != items {
				t.Fatalf("saw %d items, want %d", len(seen), items)
			}
			for i, v := range seen {
				if v != i {
					t.Fatalf("item %d missing or duplicated", i)
				}
			}
		})
	}
}

func TestUseCaseContainerFIFOWithConcurrentProducerConsumer(t *testing.T) {
	for name, mk := range ucBoxes(t, 3) {
		t.Run(name, func(t *testing.T) {
			b := mk()
			const n = 2000
			go func() {
				for i := range n {
					if err := b.WaitPush(t.Context(), i); err != nil {
						return
					}
				}
			}()
			next := 0
			for v := range b.Popping(t.Context()) {
				if v != next {
					t.Fatalf("got %d, want %d", v, next)
				}
				if next++; next == n {
					break
				}
			}
			if next != n {
				t.Fatalf("received %d items", next)
			}
		})
	}
}

// ucYieldGuard drives seq, stopping after breakAfter items, and
// records any yield that happens after the consumer returned false.
func ucYieldGuard[T any](seq iter.Seq[T], breakAfter int) (count, violations int) {
	stopped := false
	seq(func(T) bool {
		if stopped {
			violations++
		}
		count++
		if count >= breakAfter {
			stopped = true
			return false
		}
		return true
	})
	return count, violations
}

func TestUseCaseContainerIteratorsEarlyBreakEveryPosition(t *testing.T) {
	const items = 4
	fill := func(t *testing.T, push func(int) error) {
		t.Helper()
		for i := range items {
			if err := push(i); err != nil {
				t.Fatal(err)
			}
		}
	}
	type iterCase struct {
		name        string
		destructive bool
		build       func(t *testing.T) (seq iter.Seq[int], length func() int)
	}
	cases := []iterCase{
		{"Queue.IteratorWait", false, func(t *testing.T) (iter.Seq[int], func() int) {
			q := NewUnlimitedQueue[int]()
			fill(t, q.Push)
			return q.IteratorWait(t.Context()), q.Len
		}},
		{"Queue.IteratorWaitPop", true, func(t *testing.T) (iter.Seq[int], func() int) {
			q := NewUnlimitedQueue[int]()
			fill(t, q.Push)
			return q.IteratorWaitPop(t.Context()), q.Len
		}},
		{"Queue.Iterator", false, func(t *testing.T) (iter.Seq[int], func() int) {
			q := NewUnlimitedQueue[int]()
			fill(t, q.Push)
			return q.Iterator(), q.Len
		}},
	}
	dq := func(name string, destructive bool, mk func(*Deque[int], context.Context) iter.Seq[int]) iterCase {
		return iterCase{"Deque." + name, destructive, func(t *testing.T) (iter.Seq[int], func() int) {
			d := &Deque[int]{}
			fill(t, d.PushBack)
			return mk(d, t.Context()), d.Len
		}}
	}
	cases = append(cases,
		dq("IteratorFront", false, (*Deque[int]).IteratorFront),
		dq("IteratorBack", false, (*Deque[int]).IteratorBack),
		dq("IteratorWaitFront", false, (*Deque[int]).IteratorWaitFront),
		dq("IteratorWaitBack", false, (*Deque[int]).IteratorWaitBack),
		dq("IteratorWaitPopFront", true, (*Deque[int]).IteratorWaitPopFront),
		dq("IteratorWaitPopBack", true, (*Deque[int]).IteratorWaitPopBack),
	)

	for _, tc := range cases {
		for pos := 1; pos <= items; pos++ {
			t.Run(tc.name+"/BreakAt"+strconv.Itoa(pos), func(t *testing.T) {
				seq, length := tc.build(t)
				var count, violations int
				ucGuard(t, tc.name, func() { count, violations = ucYieldGuard(seq, pos) })
				if violations != 0 {
					t.Fatalf("%d yields after the consumer stopped", violations)
				}
				if count != pos {
					t.Fatalf("saw %d items, want %d", count, pos)
				}
				want := items
				if tc.destructive {
					want = items - pos
				}
				if length() != want {
					t.Fatalf("length %d, want %d", length(), want)
				}
			})
		}
	}
}

func TestUseCaseQueueIteratorReiteration(t *testing.T) {
	t.Skip("known bug: Queue.Iterator is built on irt.WithMutex, which is single-use, so a second range over the same iter.Seq yields nothing")
	q := NewUnlimitedQueue[int]()
	for i := range 3 {
		_ = q.Push(i)
	}
	seq := q.Iterator()
	first := slices.Collect(seq)
	second := slices.Collect(seq)
	if !slices.Equal(first, []int{0, 1, 2}) || !slices.Equal(first, second) {
		t.Fatalf("Iterator is not re-iterable: %v then %v", first, second)
	}
}

func TestUseCaseQueueIteratorNeverRangedDoesNotLeak(t *testing.T) {
	t.Skip("known bug: Queue.Iterator eagerly calls iter.Pull (via irt.WithMutex); an iterator that is created but never ranged leaks a goroutine")
	q := NewUnlimitedQueue[int]()
	defer testt.NoGoroutineLeak(t, 5*time.Second)()
	for range 5 {
		_ = q.Iterator()
	}
}

// TestUseCaseDequeIteratorIsAResumableCursor pins that a Deque
// iterator keeps its position across ranges over the same iter.Seq:
// breaking out and ranging again resumes, and a finished iterator
// stays finished. A fresh call to the iterator method starts over.
func TestUseCaseDequeIteratorIsAResumableCursor(t *testing.T) {
	dq := &Deque[int]{}
	for i := range 3 {
		_ = dq.PushBack(i)
	}
	front := dq.IteratorFront(t.Context())
	for v := range front {
		if v != 0 {
			t.Fatalf("first item %d", v)
		}
		break
	}
	if rest := slices.Collect(front); !slices.Equal(rest, []int{1, 2}) {
		t.Fatalf("resumed iteration yielded %v", rest)
	}
	if again := slices.Collect(front); len(again) != 0 {
		t.Fatalf("exhausted iterator yielded %v", again)
	}
	if fresh := slices.Collect(dq.IteratorFront(t.Context())); !slices.Equal(fresh, []int{0, 1, 2}) {
		t.Fatalf("fresh iterator yielded %v", fresh)
	}
}

func TestUseCaseZeroValueContainers(t *testing.T) {
	t.Run("Queue", func(t *testing.T) {
		q := &Queue[int]{}
		if q.Len() != 0 {
			t.Fatal("non-empty zero queue")
		}
		if err := q.Push(1); err != nil {
			t.Fatal(err)
		}
		if v, ok := q.Pop(); !ok || v != 1 {
			t.Fatalf("got %d, %v", v, ok)
		}
	})
	t.Run("QueueIteratorOnFreshValue", func(t *testing.T) {
		t.Skip("known bug: Queue.Iterator on a zero-value Queue dereferences the nil front sentinel (init is never run)")
		q := &Queue[int]{}
		for range q.Iterator() {
			t.Fatal("zero queue yielded")
		}
	})
	t.Run("Deque", func(t *testing.T) {
		dq := &Deque[int]{}
		if err := dq.PushFront(1); err != nil {
			t.Fatal(err)
		}
		if err := dq.PushBack(2); err != nil {
			t.Fatal(err)
		}
		if got := slices.Collect(dq.IteratorFront(t.Context())); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
		if got := slices.Collect(dq.IteratorBack(t.Context())); !slices.Equal(got, []int{2, 1}) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestUseCaseDequeOrderingBothEnds(t *testing.T) {
	const n = 50
	t.Run("PushFrontPopFrontIsLIFO", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := range n {
			_ = dq.PushFront(i)
		}
		for i := n - 1; i >= 0; i-- {
			if v, ok := dq.PopFront(); !ok || v != i {
				t.Fatalf("got %d, %v want %d", v, ok, i)
			}
		}
	})
	t.Run("PushBackPopBackIsLIFO", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := range n {
			_ = dq.PushBack(i)
		}
		for i := n - 1; i >= 0; i-- {
			if v, ok := dq.PopBack(); !ok || v != i {
				t.Fatalf("got %d, %v want %d", v, ok, i)
			}
		}
	})
	t.Run("PushFrontPopBackIsFIFO", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := range n {
			_ = dq.PushFront(i)
		}
		for i := range n {
			if v, ok := dq.PopBack(); !ok || v != i {
				t.Fatalf("got %d, %v want %d", v, ok, i)
			}
		}
	})
	t.Run("PopBackIteratorIsLIFO", func(t *testing.T) {
		dq := &Deque[int]{}
		for i := range 5 {
			_ = dq.PushBack(i)
		}
		var got []int
		for v := range dq.IteratorWaitPopBack(t.Context()) {
			got = append(got, v)
			if len(got) == 5 {
				break
			}
		}
		if !slices.Equal(got, []int{4, 3, 2, 1, 0}) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestUseCaseDequeTwoWaitersTwoPushes(t *testing.T) {
	for _, side := range []string{"Front", "Back"} {
		t.Run(side, func(t *testing.T) {
			defer testt.NoGoroutineLeak(t, 10*time.Second)()
			dq := &Deque[int]{}
			pop := dq.WaitPopFront
			if side == "Back" {
				pop = dq.WaitPopBack
			}
			type res struct {
				v   int
				err error
			}
			a := ucAsync(func() res { v, err := pop(t.Context()); return res{v, err} })
			b := ucAsync(func() res { v, err := pop(t.Context()); return res{v, err} })
			_ = dq.PushBack(1)
			_ = dq.PushBack(2)
			ra, rb := ucRecv(t, a, "waiter a"), ucRecv(t, b, "waiter b")
			if ra.err != nil || rb.err != nil {
				t.Fatalf("errors: %v %v", ra.err, rb.err)
			}
			got := []int{ra.v, rb.v}
			slices.Sort(got)
			if !slices.Equal(got, []int{1, 2}) {
				t.Fatalf("got %v", got)
			}
			if dq.Len() != 0 {
				t.Fatalf("len %d", dq.Len())
			}
		})
	}
}

func TestUseCaseForcePushBoundaries(t *testing.T) {
	dq, err := NewDeque[int](DequeOptions{Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		if err := dq.ForcePushBack(i); err != nil {
			t.Fatal(err)
		}
		if dq.Len() != 1 {
			t.Fatalf("len %d after force push %d", dq.Len(), i)
		}
	}
	if v, _ := dq.PopFront(); v != 4 {
		t.Fatalf("capacity-1 deque retained %d, want newest", v)
	}
	_ = dq.Close()
	if err := dq.ForcePushFront(1); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("ForcePushFront on closed: %v", err)
	}
	if err := dq.ForcePushBack(1); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("ForcePushBack on closed: %v", err)
	}
}
