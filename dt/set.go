package dt

import (
	"encoding/json"
	"iter"
	"maps"
	"sync"

	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

// Set provides a generic unordered set implementation. This implementation
// is not thread-safe. For a synchronized version, use adt.Set.
// For a set that maintains insertion order, use OrderedSet or adt.OrderedSet.
type Set[T comparable] struct {
	once sync.Once
	hash stw.Map[T, struct{}]
}

// MakeSet constructs an unordered set and adds all items from the input
// sequence to the set.
func MakeSet[T comparable](in iter.Seq[T]) *Set[T] {
	out := &Set[T]{}
	out.Extend(in)
	return out
}

func (s *Set[T]) init()   { s.once.Do(s.doInit) }
func (s *Set[T]) doInit() { s.hash = stw.Map[T, struct{}]{} }

// Len returns the number of items tracked in the set.
func (s *Set[T]) Len() int { s.init(); return len(s.hash) }

// Check returns true if the item is in the set.
func (s *Set[T]) Check(in T) bool { s.init(); return s.hash.Check(in) }

// Iterator returns a new-style native Go iterator for the items in the set.
// The iteration order is undefined and may vary between calls.
func (s *Set[T]) Iterator() iter.Seq[T] { s.init(); return s.hash.Keys() }

// Delete removes the item from the set, returning true when the
// item had been in the Set, and returning false otherwise.
func (s *Set[T]) Delete(in T) (existed bool) {
	s.init()

	if existed = s.hash.Check(in); existed {
		delete(s.hash, in)
	}

	return
}

// Add adds an item to the set and returns true if the item had
// been in the set before Add. In all cases when Add
// returns, the item is a member of the set.
func (s *Set[T]) Add(in T) (existed bool) {
	s.init()

	if existed = s.hash.Check(in); !existed {
		s.hash.Ensure(in)
	}

	return
}

// Extend adds all items from the iterator to the set.
func (s *Set[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.add) }
func (s *Set[T]) add(in T)                { s.Add(in) }

// Equal tests two sets, returning true if the items in the sets have
// equal values. Order is not considered.
func (s *Set[T]) Equal(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	return maps.Equal(s.hash, other.hash)
}

// MarshalJSON generates a JSON array of the items in the set.
// The order of items in the array is undefined.
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
		irt.Apply(irt.Slice(items), s.add)
	}
	return err
}
