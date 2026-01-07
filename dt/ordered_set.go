package dt

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

// OrderedSet provides a generic set implementation that always
// maintains insertion order. This implementation is not thread-safe.
// For a synchronized version, use adt.OrderedSet.
type OrderedSet[T comparable] struct {
	once sync.Once
	hash stw.Map[T, *Element[T]]
	list *List[T]
}

// MakeOrderedSet constructs an ordered set and adds all items from the
// input sequence to the set in order.
func MakeOrderedSet[T comparable](in iter.Seq[T]) *OrderedSet[T] {
	out := &OrderedSet[T]{}
	out.Extend(in)
	return out
}

// SortQuick sorts the elements in the set using
// sort.StableSort. Typically faster than SortMerge, but potentially
// more memory intensive for some types.
func (s *OrderedSet[T]) SortQuick(cf func(T, T) int) { s.init(); s.list.SortQuick(cf) }

// SortMerge sorts the elements in the set using a merge sort
// algorithm.
func (s *OrderedSet[T]) SortMerge(cf func(T, T) int) { s.init(); s.list.SortMerge(cf) }

func (s *OrderedSet[T]) init() { s.once.Do(s.doInit) }
func (s *OrderedSet[T]) doInit() {
	s.hash = stw.Map[T, *Element[T]]{}
	s.list = &List[T]{}
}

// Len returns the number of items tracked in the set.
func (s *OrderedSet[T]) Len() int { s.init(); return len(s.hash) }

// Check returns true if the item is in the set.
func (s *OrderedSet[T]) Check(in T) bool { s.init(); return s.hash.Check(in) }

// Iterator returns a new-style native Go iterator for the items in the set
// in insertion order.
func (s *OrderedSet[T]) Iterator() iter.Seq[T] { s.init(); return s.list.IteratorFront() }

// Delete removes the item from the set, returning true when the
// item had been in the Set, and returning false otherwise.
func (s *OrderedSet[T]) Delete(in T) bool {
	s.init()

	defer delete(s.hash, in)

	e, ok := s.hash.Load(in)
	if !ok {
		return false
	}

	if e != nil {
		e.Remove()
	}

	return true
}

// Add adds an item to the set and returns true if the item had
// been in the set before Add. In all cases when Add
// returns, the item is a member of the set.
func (s *OrderedSet[T]) Add(in T) (ok bool) {
	s.init()

	ok = s.hash.Check(in)
	if ok {
		return
	}

	elem := NewElement(in)
	s.list.Back().Append(elem)
	s.hash.Set(in, elem)

	return
}

// Extend adds all items encountered in the stream to the set.
func (s *OrderedSet[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.add) }
func (s *OrderedSet[T]) add(in T)                { s.Add(in) }

// Equal tests two sets, returning true if the items in the sets have
// equal values in the same order.
func (s *OrderedSet[T]) Equal(other *OrderedSet[T]) bool {
	if len(s.hash) != other.Len() {
		return false
	}
	return irt.Equal(s.Iterator(), other.Iterator())
}

// MarshalJSON generates a JSON array of the items in the set in insertion order.
func (s *OrderedSet[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(irt.Collect(s.Iterator(), 0, s.Len()))
}

// UnmarshalJSON reads input JSON data, constructs an array in memory
// and then adds items from the array to existing set in order. Items that are
// in the set when UnmarshalJSON begins are not modified.
func (s *OrderedSet[T]) UnmarshalJSON(in []byte) error {
	var items []T
	err := json.Unmarshal(in, &items)
	if err == nil {
		irt.Apply(irt.Slice(items), s.add)
	}
	return err
}
