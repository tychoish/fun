package adt

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun/irt"
)

// Set provides a thread-safe generic unordered set implementation.
// All operations are synchronized. For a set that maintains insertion
// order, use OrderedSet.
type Set[T comparable] struct {
	once sync.Once
	hash *Map[T, struct{}]
}

// MakeSet constructs an unordered set and adds all items from the input
// sequence.
func MakeSet[T comparable](seq iter.Seq[T]) *Set[T] {
	out := &Set[T]{}
	out.Extend(seq)
	return out
}

func (s *Set[T]) mp() *Map[T, struct{}] { s.once.Do(s.doInit); return s.hash }
func (s *Set[T]) doInit()               { s.hash = &Map[T, struct{}]{} }

// Len returns the number of items tracked in the set.
func (s *Set[T]) Len() int { return s.mp().Len() }

// Check returns true if the item is in the set.
func (s *Set[T]) Check(in T) bool { return s.mp().Check(in) }

// Iterator returns a new-style native Go iterator for the items in the set.
// The iteration order is undefined and may vary between calls.
//
// When Synchronized, the lock is NOT held when the iterator is advanced.
func (s *Set[T]) Iterator() iter.Seq[T] { return s.mp().Keys() }

// Delete removes the item from the set, returning true when the item
// existed in the Set and false otherwise.
func (s *Set[T]) Delete(in T) (ok bool) { _, ok = s.mp().mp.LoadAndDelete(in); return }

// Add adds an item to the set and returns true if the item had
// been in the set before Add. In all cases when Add
// returns, the item is a member of the set.
func (s *Set[T]) Add(in T) (ok bool) { _, ok = s.mp().mp.LoadOrStore(in, struct{}{}); return }

// Extend adds all items encountered in the stream to the set.
func (s *Set[T]) Extend(iter iter.Seq[T]) { irt.Apply(iter, s.add) }
func (s *Set[T]) add(in T)                { s.Add(in) }

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
