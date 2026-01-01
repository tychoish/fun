package adt

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/dt/stw"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// Set provides a thread-safe generic set implementation with optional
// order tracking (via Order()). All operations are synchronized.
type Set[T comparable] struct {
	mtx  sync.Mutex
	once sync.Once
	hash stw.Map[T, *dt.Element[T]]
	list *dt.List[T]
}

// MakeSet constructs a map and adds all items from the input
// sequence. These sets are ordered.
func MakeSet[T comparable](seq iter.Seq[T]) *Set[T] {
	out := &Set[T]{}
	out.Order()
	out.Extend(seq)
	return out
}

// Order enables order tracking for the Set. The method panics if there
// is more than one item in the map. If order tracking is enabled,
// this operation is a noop.
func (s *Set[T]) Order() {
	defer s.with(s.lock())
	s.init()

	if s.list != nil {
		return
	}

	ft.Invariant(ers.When(s.hash.Len() != 0, "cannot make an ordered set out of an un-ordered set that contain data"))

	s.list = &dt.List[T]{}
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
	s.list = &dt.List[T]{}
	for item := range s.hash.Iterator() {
		s.list.PushBack(item)
	}
}

func (s *Set[T]) init()              { s.once.Do(s.doInit) }
func (s *Set[T]) doInit()            { s.hash = stw.Map[T, *dt.Element[T]]{} }
func (*Set[T]) with(mtx *sync.Mutex) { mtx.Unlock() }
func (s *Set[T]) lock() *sync.Mutex  { s.mtx.Lock(); return &s.mtx }

// Len returns the number of items tracked in the set.
func (s *Set[T]) Len() int { defer s.with(s.lock()); return s.hash.Len() }

// Check returns true if the item is in the set.
func (s *Set[T]) Check(in T) bool { defer s.with(s.lock()); return s.hash.Check(in) }

// Add attempts to add the item while holding to the mutex, and is a noop otherwise.
func (s *Set[T]) Add(in T) { _ = s.AddCheck(in) }

// Delete attempts to remove the item from the set.
func (s *Set[T]) Delete(in T) { _ = s.DeleteCheck(in) }

// Iterator returns a new-style native Go iterator for the items in the set. Provides items in
// iteration order if the set is ordered. If the Set is ordered, then the future produces items in
// the set's order.
//
// When Synchrnoized, the lock is NOT held when the iterator is advanced.
func (s *Set[T]) Iterator() iter.Seq[T] {
	s.init()
	if s.list != nil {
		return s.list.IteratorFront()
	}
	return s.hash.Keys()
}

// DeleteCheck removes the item from the set, return true when the
// item had been in the Set, and returning false othewise.
func (s *Set[T]) DeleteCheck(in T) bool {
	defer s.with(s.lock())
	defer s.hash.Delete(in)
	s.init()

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
	s.init()

	ok = s.hash.Check(in)
	if ok {
		return
	}

	if s.list == nil {
		s.hash.SetDefault(in)
		return
	}

	elem := dt.NewElement(in)
	s.list.Back().Append(elem)
	s.hash.Add(in, elem)

	return
}

// Extend adds all items encountered in the stream to the set.
func (s *Set[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.Add) }

// MarshalJSON generates a JSON array of the items in the set.
func (s *Set[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(irt.Collect(s.Iterator(), 0, s.Len()))
}

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
