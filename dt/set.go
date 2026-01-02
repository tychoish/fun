package dt

import (
	"encoding/json"
	"iter"
	"maps"

	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/dt/stw"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// Set provides a generic set implementation with optional
// order tracking (via Order()). This implementation is not
// thread-safe. For a synchronized version, use adt.Set.
type Set[T comparable] struct {
	hash stw.Map[T, *Element[T]]
	list *List[T]
}

// MakeSet constructs a set and adds all items from the input
// sequence to set. These sets are ordered.
func MakeSet[T comparable](in iter.Seq[T]) *Set[T] {
	out := &Set[T]{}
	out.Order()
	out.Extend(in)
	return out
}

// Order enables order tracking for the Set. The method panics if there
// is more than one item in the map. If order tracking is enabled,
// this operation is a noop.
func (s *Set[T]) Order() {
	s.init()
	if s.list != nil {
		return
	}
	ft.Invariant(ers.When(len(s.hash) != 0, "cannot make an ordered set out of an un-ordered set that contain data"))

	s.list = &List[T]{}
}

// SortQuick sorts the elements in the set using
// sort.StableSort. Typically faster than SortMerge, but potentially
// more memory intensive for some types. If the set is not ordered, it
// will become ordered. Unlike the Order() method, you can use this
// method on a populated but unordered set.
func (s *Set[T]) SortQuick(lt cmp.LessThan[T]) { s.init(); s.setupOrdered(); s.list.SortQuick(lt) }

// SortMerge sorts the elements in an ordered set using a merge sort
// algorithm. If the set is not ordered, it will become
// ordered. Unlike the Order() method, you can use this method on a
// populated but unordered set.
func (s *Set[T]) SortMerge(lt cmp.LessThan[T]) { s.init(); s.setupOrdered(); s.list.SortMerge(lt) }

func (s *Set[T]) setupOrdered() {
	if s.list == nil {
		s.list = &List[T]{}
		for item := range s.hash {
			s.list.PushBack(item)
		}
	}
}

func (s *Set[T]) isOrdered() bool { return s.list != nil }
func (s *Set[T]) init()           { ft.CallWhen(s.hash == nil, s.initHash) }
func (s *Set[T]) initHash()       { s.hash = stw.Map[T, *Element[T]]{} }

// Len returns the number of items tracked in the set.
func (s *Set[T]) Len() int { return len(s.hash) }

// Check returns true if the item is in the set.
func (s *Set[T]) Check(in T) bool { return s.hash.Check(in) }

// Iterator returns a new-style native Go iterator for the items in the set. Provides items in
// iteration order if the set is ordered. If the Set is ordered, then the future produces items in
// the set's order.
func (s *Set[T]) Iterator() iter.Seq[T] {
	if s.list != nil {
		return s.list.IteratorFront()
	}
	return s.hash.Keys()
}

// Delete removes the item from the set, return true when the
// item had been in the Set, and returning false othewise.
func (s *Set[T]) Delete(in T) bool {
	s.init()

	defer delete(s.hash, in)

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
func (s *Set[T]) Add(in T) (ok bool) {
	s.init()
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

// Extend adds all items encountered in the stream to the set.
func (s *Set[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.add) }
func (s *Set[T]) add(in T)                { s.Add(in) }

// Equal tests two sets, returning true if the items in the sets have
// equal values. If the sets are ordered, order is considered.
func (s *Set[T]) Equal(other *Set[T]) bool {
	switch {
	case len(s.hash) != other.Len():
		return false
	case s.isOrdered() != other.isOrdered():
		return false
	case s.isOrdered():
		return irt.Equal(s.Iterator(), other.Iterator())
	default:
		return maps.Equal(s.hash, other.hash)
	}
}

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
		ft.ApplyMany(s.add, items)
	}
	return err
}
