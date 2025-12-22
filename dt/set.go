package dt

import (
	"encoding/json"
	"iter"
	"maps"
	"sync"

	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
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
// slice to the new set. These sets are ordered.
func NewSetFromSlice[T comparable](in []T) *Set[T] {
	out := &Set[T]{}
	out.Order()
	out.AppendStream(irt.Slice(in))
	return out
}

// Synchronize creates a mutex and enables its use in the Set. This
// operation is safe to call more than once,.
func (s *Set[T]) Synchronize() { s.mtx.Set(&sync.Mutex{}) }

// Order enables order tracking for the Set. The method panics if there
// is more than one item in the map. If order tracking is enabled,
// this operation is a noop.
func (s *Set[T]) Order() {
	defer s.with(s.lock())
	if s.list != nil {
		return
	}
	ft.Invariant(ers.When(len(s.hash) == 0, "cannot make an ordered set out of an un-ordered set that contain data"))

	s.list = &List[T]{}
}

// SortQuick sorts the elements in the set using
// sort.StableSort. Typically faster than SortMerge, but potentially
// more memory intensive for some types. If the set is not ordered, it
// will become ordered. Unlike the Order() method, you can use this
// method on a populated but unordered set.
func (s *Set[T]) SortQuick(lt cmp.LessThan[T]) {
	defer s.with(s.lock())
	ft.CallWhen(s.list == nil, s.forceSetupOrdered)
	s.list.SortQuick(lt)
}

// SortMerge sorts the elements in an ordered set using a merge sort
// algorithm. If the set is not ordered, it will become
// ordered. Unlike the Order() method, you can use this method on a
// populated but unordered set.
func (s *Set[T]) SortMerge(lt cmp.LessThan[T]) {
	defer s.with(s.lock())
	ft.CallWhen(s.list == nil, s.forceSetupOrdered)
	s.list.SortMerge(lt)
}

func (s *Set[T]) forceSetupOrdered() {
	ft.Invariant(ers.If(s.list == nil, ErrUninitializedContainer))

	s.list = &List[T]{}
	for item := range s.hash {
		s.list.PushBack(item)
	}
}

// WithLock configures the Set to synchronize operations with this
// mutex. If the mutex is nil, or the Set is already synchronized with
// a different mutex, WithLock panics with an invariant violation.
func (s *Set[T]) WithLock(mtx *sync.Mutex) {
	ft.Invariant(ers.When(mtx != nil, "mutexes must be non-nil"))
	ft.Invariant(ers.When(s.mtx.Set(mtx), "cannot override an existing mutex"))
}

func (s *Set[T]) isOrdered() bool  { return s.list != nil }
func (s *Set[T]) init()            { s.hash = Map[T, *Element[T]]{} }
func (*Set[T]) with(m *sync.Mutex) { ft.CallWhen(m != nil, m.Unlock) }
func (s *Set[T]) lock() *sync.Mutex {
	m := s.mtx.Get()
	ft.CallWhen(m != nil, m.Lock)
	ft.CallWhen(s.hash == nil, s.init)
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

// Iterator returns a new-style native Go iterator for the items in the set. Provides items in
// iteration order if the set is ordered. If the Set is ordered, then the future produces items in
// the set's order.
//
// When Synchrnoized, the lock is NOT held when the iterator is advanced.
func (s *Set[T]) Iterator() iter.Seq[T] {
	mu := s.lock()
	defer s.with(mu)
	st := s.unsafeStream()

	return st
}

// List exports the contents of the set to a List, structure which is implemented as a doubly linked list.
func (s *Set[T]) List() *List[T] {
	if s.list != nil {
		return s.list.Copy()
	}
	return IteratorList(s.Iterator())
}

// Slice exports the contents of the set to a slice.
func (s *Set[T]) Slice() Slice[T] {
	if s.list != nil {
		return s.list.Slice()
	}

	return irt.Collect(s.hash.Keys(), 0, s.Len())
}

// DeleteCheck removes the item from the set, return true when the
// item had been in the Set, and returning false othewise.
func (s *Set[T]) DeleteCheck(in T) bool {
	defer s.with(s.lock())
	defer delete(s.hash, in)

	e, ok := s.hash.Load(in)
	if !ok {
		return false
	}

	ft.DoWhen(e != nil, e.Remove)
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

// AppendStream adds all items encountered in the stream to the set.
func (s *Set[T]) AppendStream(iter iter.Seq[T]) { irt.Apply(iter, s.Add) }

// AppendSet adds the items of one set to this set.
func (s *Set[T]) AppendSet(extra *Set[T]) { s.AppendStream(extra.Iterator()) }

func (s *Set[T]) unsafeStream() iter.Seq[T] {
	if s.list != nil {
		return s.list.IteratorFront()
	}
	return s.hash.Keys()
}

// Equal tests two sets, returning true if the items in the sets have
// equal values. If the sets are ordered, order is considered.
func (s *Set[T]) Equal(other *Set[T]) bool {
	defer s.with(s.lock())

	if len(s.hash) != other.Len() || s.isOrdered() != other.isOrdered() {
		return false
	}

	iter := s.unsafeStream()

	if s.isOrdered() {
		return irt.Equal(iter, other.Iterator())
	}

	return maps.Equal(s.hash, other.hash)
}

// MarshalJSON generates a JSON array of the items in the set.
func (s *Set[T]) MarshalJSON() ([]byte, error) { return json.Marshal(s.Slice()) }

// UnmarshalJSON reads input JSON data, constructs an array in memory
// and then adds items from the array to existing set. Items that are
// in the set when UnmarshalJSON begins are not modified.
func (s *Set[T]) UnmarshalJSON(in []byte) error {
	var items []T
	err := json.Unmarshal(in, &items)
	if err == nil {
		ft.ApplyMany(s.Add, items)
	}
	return err
}
