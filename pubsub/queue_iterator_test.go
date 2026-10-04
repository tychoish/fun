package pubsub

import (
	"iter"
	"sync"
	"testing"
	"time"

	"github.com/tychoish/fun/testt"
)

// These tests pin down three bugs found in Queue.Iterator (built, prior
// to the fix, on irt.WithMutex, which wraps a single-use, eagerly
// started iter.Pull):
//
//  1. Ranging the same iter.Seq returned by Iterator() a second time
//     yielded nothing instead of iterating again.
//  2. Calling Iterator() without ever ranging it leaked the parked
//     iter.Pull goroutine.
//  3. Calling Iterator() on a zero-value (never constructed) Queue
//     panicked on a nil front sentinel instead of running the lazy
//     init().

func TestQueueIteratorReusableAcrossRanges(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	if err := q.Push(1); err != nil {
		t.Fatal(err)
	}
	if err := q.Push(2); err != nil {
		t.Fatal(err)
	}

	it := q.Iterator()

	var first, second []int
	for v := range it {
		first = append(first, v)
	}
	for v := range it {
		second = append(second, v)
	}

	if len(first) != 2 || first[0] != 1 || first[1] != 2 {
		t.Fatalf("first range: got %v, want [1 2]", first)
	}
	if len(second) != 2 || second[0] != 1 || second[1] != 2 {
		t.Fatalf("second range: got %v, want [1 2]; iterator is not reusable", second)
	}
}

func TestQueueIteratorUnrangedDoesNotLeakGoroutine(t *testing.T) {
	defer testt.NoGoroutineLeak(t, 5*time.Second)()

	q := NewUnlimitedQueue[int]()
	_ = q.Push(1)

	// Create several iterators and never range over them.
	for range 5 {
		_ = q.Iterator()
	}
}

func TestQueueIteratorZeroValueQueue(t *testing.T) {
	var q Queue[int]

	// Must not panic, and must yield nothing.
	var got []int
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Iterator() on zero-value Queue panicked: %v", r)
			}
		}()
		for v := range q.Iterator() {
			got = append(got, v)
		}
	}()
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}

	// After a Push, a new Iterator() call should work normally.
	if err := q.Push(42); err != nil {
		t.Fatal(err)
	}
	got = nil
	for v := range q.Iterator() {
		got = append(got, v)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("got %v, want [42]", got)
	}
}

func TestQueueIteratorStopsWhenQueueClosedMidIteration(t *testing.T) {
	q := NewUnlimitedQueue[int]()
	for i := range 3 {
		if err := q.Push(i); err != nil {
			t.Fatal(err)
		}
	}

	next, stop := iter.Pull(q.Iterator())
	defer stop()

	v, ok := next()
	if !ok || v != 0 {
		t.Fatalf("got (%d, %v), want (0, true)", v, ok)
	}

	if err := q.Close(); err != nil {
		t.Fatal(err)
	}

	if _, ok := next(); ok {
		t.Fatal("iterator should stop once the queue is closed")
	}
}

// TestQueueIteratorPopConcurrentNeverDuplicatesOrReorders pins the
// behavior of a single, long-lived Iterator() cursor (a
// non-destructive, live, non-blocking iterator) while items are
// concurrently popped out from under it: the cursor must never yield
// an item twice, and items it does yield must come out in increasing
// push order. This mirrors TestDequeIteratorSurvivesPopOfCurrent /
// TestQueueIteratorWaitSurvivesPop, but exercises the non-blocking
// Iterator (stepped manually via iter.Pull, for test purposes only)
// against a concurrently-popping goroutine instead of interleaving the
// two sequentially.
func TestQueueIteratorPopConcurrentNeverDuplicatesOrReorders(t *testing.T) {
	const items = 200
	q := NewUnlimitedQueue[int]()

	for i := range items {
		if err := q.Push(i); err != nil {
			t.Fatal(err)
		}
	}

	next, stop := iter.Pull(q.Iterator())
	defer stop()

	var wg sync.WaitGroup
	popped := make([]int, 0, items)
	var mu sync.Mutex
	wg.Go(func() {
		for range items {
			v, ok := q.Pop()
			if !ok {
				return
			}
			mu.Lock()
			popped = append(popped, v)
			mu.Unlock()
		}
	})

	seen := make(map[int]int)
	prev := -1
	for {
		v, ok := next()
		if !ok {
			break
		}
		if v <= prev {
			t.Fatalf("iterator went backwards or repeated: %d after %d", v, prev)
		}
		prev = v
		seen[v]++
	}

	wg.Wait()

	for v, n := range seen {
		if n > 1 {
			t.Fatalf("item %d observed %d times by the iterator cursor", v, n)
		}
	}
	if len(popped) != items {
		t.Fatalf("popped %d items, want %d", len(popped), items)
	}
	for i, v := range popped {
		if v != i {
			t.Fatalf("pop order broken at %d: got %d", i, v)
		}
	}
}
