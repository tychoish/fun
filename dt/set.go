package dt

import (
	"context"
	"iter"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ft"
)

// Set provides a flexible generic set implementation, with optional
// safety for concurrent use (via Synchronize() and WithLock() methods,)
// and optional order tracking (via Order().)
type Set[T comparable] struct {
	hash Map[T, *Element[T]]
	list *List[T]
	mtx  atomic[*sync.Mutex]
}

// NewSetFromSlice constructs a map and adds all items from the input
// slice to the new set.
func NewSetFromSlice[T comparable](in []T) *Set[T] {
	out := &Set[T]{}
	out.Populate(fun.SliceStream(in))
	return out
}

// NewSetFromMap constructs a Set of pairs derived from the input
// map. The resulting set is constrained by both the keys and the
// values, and so would permit duplicate "keys" from the perspective
// of the map, and may not therefore roundtrip.
func NewSetFromMap[K, V comparable](in map[K]V) *Set[Pair[K, V]] {
	out := &Set[Pair[K, V]]{}
	out.Populate(MapStream(in))
	return out
}

// Synchronize creates a mutex and enables its use in the Set. This
// operation is safe to call more than once,
func (s *Set[T]) Synchronize() { s.mtx.Set(&sync.Mutex{}) }

// Order enables order tracking for the Set. The method panics if there
// is more than one item in the map. If order tracking is enabled,
// this operation is a noop.
func (s *Set[T]) Order() {
	defer s.with(s.lock())
	if s.list != nil {
		return
	}
	fun.Invariant.Ok(len(s.hash) == 0, "cannot make an ordered set out of an un-ordered set that contain data")
	s.list = &List[T]{}
}

// SortQuick sorts the elements in the set using
// sort.StableSort. Typically faster than SortMerge, but potentially
// more memory intensive for some types. If the set is not ordered, it
// will become ordered. Unlike the Order() method, you can use this
// method on a populated but unordered set.
func (s *Set[T]) SortQuick(lt cmp.LessThan[T]) {
	defer s.with(s.lock())
	ft.WhenCall(s.list == nil, s.forceSetupOrdered)
	s.list.SortQuick(lt)
}

// SortMerge sorts the elements in an ordered set using a merge sort
// algorithm. If the set is not ordered, it will become
// ordered. Unlike the Order() method, you can use this method on a
// populated but unordered set.
func (s *Set[T]) SortMerge(lt cmp.LessThan[T]) {
	defer s.with(s.lock())
	ft.WhenCall(s.list == nil, s.forceSetupOrdered)
	s.list.SortMerge(lt)
}

func (s *Set[T]) forceSetupOrdered() {
	fun.Invariant.Ok(s.list == nil)
	s.list = &List[T]{}
	for item := range s.hash {
		s.list.PushBack(item)
	}
}

// WithLock configures the Set to synchronize operations with this
// mutex. If the mutex is nil, or the Set is already synchronized with
// a different mutex, WithLock panics with an invariant violation.
func (s *Set[T]) WithLock(mtx *sync.Mutex) {
	fun.Invariant.Ok(mtx != nil, "mutexes must be non-nil")
	fun.Invariant.Ok(s.mtx.Set(mtx), "cannot override an existing mutex")
}

func (s *Set[T]) isOrdered() bool { return s.list != nil }

func (s *Set[T]) init()            { s.hash = Map[T, *Element[T]]{} }
func (*Set[T]) with(m *sync.Mutex) { ft.WhenCall(m != nil, m.Unlock) }
func (s *Set[T]) lock() *sync.Mutex {
	m := s.mtx.Get()
	ft.WhenCall(m != nil, m.Lock)
	ft.WhenCall(s.hash == nil, s.init)
	return m
}

// Add attempts to add the item to the mutex, and is a noop otherwise.
func (s *Set[T]) Add(in T) { _ = s.AddCheck(in) }

// Len returns the number of items tracked in the set.
func (s *Set[T]) Len() int { defer s.with(s.lock()); return len(s.hash) }

// Check returns true if the item is in the set.
func (s *Set[T]) Check(in T) bool { defer s.with(s.lock()); return s.hash.Check(in) }

// Delete attempts to remove the item from the set.
func (s *Set[T]) Delete(in T) { _ = s.DeleteCheck(in) }

// Stream provides a way to iterate over the items in the
// set. Provides items in iteration order if the set is ordered.
func (s *Set[T]) Stream() *fun.Stream[T] { return s.Generator().Stream() }

// Iterator returns a new-style native Go iterator for the items in the set.
func (s *Set[T]) Iterator() iter.Seq[T] { return s.Stream().Iterator(context.Background()) }

// DeleteCheck removes the item from the set, return true when the
// item had been in the Set, and returning false othewise
func (s *Set[T]) DeleteCheck(in T) bool {
	defer s.with(s.lock())
	defer delete(s.hash, in)

	e, ok := s.hash.Load(in)
	if !ok {
		return false
	}

	ft.WhenDo(e != nil, e.Remove)
	return true
}

// AddCheck adds an item to the set and returns true if the item had
// been in the set before AddCheck. In all cases when AddCheck
// returns, the item is a member of the set.
func (s *Set[T]) AddCheck(in T) (ok bool) {
	defer s.with(s.lock())
	ok = s.hash.Check(in)
	if ok {
		return
	}

	if s.list == nil {
		s.hash.SetDefault(in)
		return
	}

	elem := NewElement(in)
	s.list.Back().Append(elem)
	s.hash.Add(in, elem)

	return
}

// Populate adds all items encountered in the stream to the set.
func (s *Set[T]) Populate(iter *fun.Stream[T]) { iter.ReadAll(s.Add).Ignore().Wait() }

// Extend adds the items of one set to this set.
func (s *Set[T]) Extend(extra *Set[T]) { s.Populate(extra.Stream()) }

func (s *Set[T]) unsafeStream() *fun.Stream[T] {
	if s.list != nil {
		return s.list.StreamFront()
	}
	return s.hash.Keys()
}

// Generator will produce each item from set on successive calls. If
// the Set is ordered, then the generator produces items in the set's
// order. If the Set is synchronize, then the Generator always holds
// the Set's lock when called.
func (s *Set[T]) Generator() (out fun.Generator[T]) {
	defer s.with(s.lock())
	defer func() { mu := s.mtx.Get(); ft.WhenDo(mu != nil, func() fun.Generator[T] { return out.WithLock(mu) }) }()

	if s.list != nil {
		return s.list.GeneratorFront()
	}

	return s.hash.GeneratorKeys()
}

// Equal tests two sets, returning true if the items in the sets have
// equal values. If the sets are ordered, order is considered.
func (s *Set[T]) Equal(other *Set[T]) bool {
	defer s.with(s.lock())

	if len(s.hash) != other.Len() || s.isOrdered() != other.isOrdered() {
		return false
	}

	ctx := context.Background()
	iter := s.unsafeStream()

	if s.isOrdered() {
		otherIter := other.Stream()
		for iter.Next(ctx) && otherIter.Next(ctx) {
			if iter.Value() != otherIter.Value() {
				return false
			}
		}

		return iter.Close() == nil && otherIter.Close() == nil
	}

	for iter.Next(ctx) {
		if !other.Check(iter.Value()) {
			return false
		}
	}

	return iter.Close() == nil
}

// MarshalJSON generates a JSON array of the items in the set.
func (s *Set[T]) MarshalJSON() ([]byte, error) { return s.Stream().MarshalJSON() }

// UnmarshalJSON reads input JSON data, constructs an array in memory
// and then adds items from the array to existing set. Items that are
// in the set when UnmarshalJSON begins are not modified.
func (s *Set[T]) UnmarshalJSON(in []byte) error {
	iter := NewSlice([]T{}).Stream()
	iter.AddError(iter.UnmarshalJSON(in))
	s.Populate(iter)
	return iter.Close()
}
