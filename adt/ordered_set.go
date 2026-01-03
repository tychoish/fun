package adt

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// OrderedSet provides a thread-safe generic set implementation that
// always maintains insertion order. All operations are synchronized.
type OrderedSet[T comparable] struct {
	mtx  sync.Mutex
	once sync.Once
	hash *Map[T, *dt.Element[T]]
	list *dt.List[T]
}

// MakeOrderedSet constructs an ordered set and adds all items from the
// input sequence in order.
func MakeOrderedSet[T comparable](seq iter.Seq[T]) *OrderedSet[T] {
	out := &OrderedSet[T]{}
	out.Extend(seq)
	return out
}

// SortQuick sorts the elements in the set using
// a stable sort. Typically faster than SortMerge,
// but potentially more memory intensive for some types.
func (s *OrderedSet[T]) SortQuick(lt cmp.LessThan[T]) { defer s.with(s.lock()); s.list.SortQuick(lt) }

// SortMerge sorts the elements in the set using a merge sort
// algorithm.
func (s *OrderedSet[T]) SortMerge(lt cmp.LessThan[T]) { defer s.with(s.lock()); s.list.SortMerge(lt) }

func (s *OrderedSet[T]) init() { s.once.Do(s.doInit) }
func (s *OrderedSet[T]) doInit() {
	s.hash = &Map[T, *dt.Element[T]]{}
	s.list = &dt.List[T]{}
}
func (*OrderedSet[T]) with(mtx *sync.Mutex) { mtx.Unlock() }
func (s *OrderedSet[T]) lock() *sync.Mutex  { s.mtx.Lock(); s.init(); return &s.mtx }

// Len returns the number of items tracked in the set.
func (s *OrderedSet[T]) Len() int { defer s.with(s.lock()); return s.hash.Len() }

// Check returns true if the item is in the set.
func (s *OrderedSet[T]) Check(in T) bool { defer s.with(s.lock()); return s.hash.Check(in) }

// Iterator returns a new-style native Go iterator for the items in the set
// in insertion order.
func (s *OrderedSet[T]) Iterator() iter.Seq[T] {
	s.init()
	return irt.WithMutex(s.list.IteratorFront(), &s.mtx)
}

// Delete removes the item from the set, returning true when the item
// existed in the Set and false otherwise.
func (s *OrderedSet[T]) Delete(in T) bool {
	defer s.with(s.lock())
	defer s.hash.Delete(in)

	e, ok := s.hash.Load(in)
	if !ok {
		return false
	}

	ft.DoWhen(e != nil, e.Remove)
	return true
}

// Add adds an item to the set and returns true if the item had
// been in the set before Add. In all cases when Add
// returns, the item is a member of the set.
func (s *OrderedSet[T]) Add(in T) (ok bool) {
	defer s.with(s.lock())

	if ok = s.hash.Check(in); ok {
		return
	}

	elem := dt.NewElement(in)
	s.list.Back().Append(elem)
	s.hash.Store(in, elem)

	return
}

// Extend adds all items encountered in the stream to the set.
func (s *OrderedSet[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.add) }
func (s *OrderedSet[T]) add(in T)                { s.Add(in) }

// MarshalJSON generates a JSON array of the items in the set in insertion order.
func (s *OrderedSet[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(irt.Collect(s.Iterator(), 0, s.Len()))
}

// UnmarshalJSON reads input JSON data, constructs an array in memory
// and then adds items from the array to existing set. Items that are
// in the set when UnmarshalJSON begins are not modified.
func (s *OrderedSet[T]) UnmarshalJSON(in []byte) error {
	var items []T
	err := json.Unmarshal(in, &items)
	if err == nil {
		irt.Apply(irt.Slice(items), s.add)
	}
	return err
}
