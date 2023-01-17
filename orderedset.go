package fun

import (
	"context"
)

type orderedSetItem[T comparable] struct {
	idx     int
	item    T
	deleted bool
}

type orderedSetImpl[T comparable] struct {
	elems        []orderedSetItem[T]
	set          map[T]orderedSetItem[T]
	deletedCount int
}

// NewOrderedSet produces an order-preserving set implementation.
func NewOrderedSet[T comparable]() Set[T] {
	return MakeOrderedSet[T](0)
}

// MakeOrderedSet produces an order-preserving set implementation,
// with pre-allocated capacity to the specified size.
func MakeOrderedSet[T comparable](size int) Set[T] {
	return &orderedSetImpl[T]{
		set:   make(map[T]orderedSetItem[T], size),
		elems: make([]orderedSetItem[T], 0, size),
	}
}

func (s *orderedSetImpl[T]) lazyDelete() {
	if s.deletedCount < len(s.set)*2 {
		return
	}

	newElems := make([]orderedSetItem[T], len(s.set))
	newIdx := 0
	for oldIdx := range s.elems {
		newItem := s.elems[oldIdx]
		if newItem.deleted {
			continue
		}
		newItem.idx = newIdx
		newElems[newIdx] = newItem
		s.set[newItem.item] = newItem
		newIdx++
	}
	s.elems = newElems
	s.deletedCount = 0
}

func (s *orderedSetImpl[T]) Add(in T) {
	if _, ok := s.set[in]; ok {
		return
	}
	val := orderedSetItem[T]{item: in, idx: len(s.elems)}

	s.set[in] = val
	s.elems = append(s.elems, val)
}

func (s *orderedSetImpl[T]) Check(in T) bool { _, ok := s.set[in]; return ok }
func (s *orderedSetImpl[T]) Len() int        { return len(s.set) }
func (s *orderedSetImpl[T]) Delete(it T) {
	val, ok := s.set[it]
	if !ok {
		return
	}
	s.elems[val.idx].deleted = true
	s.elems[val.idx].item = *new(T) // zero to release memory
	delete(s.set, val.item)
	s.deletedCount++

	s.lazyDelete()
}
func (s *orderedSetImpl[T]) Iterator(ctx context.Context) Iterator[T] {
	s.lazyDelete()
	return &orderedSetIterImpl[T]{set: s, lastIdx: -1}
}

type orderedSetIterImpl[T comparable] struct {
	set     *orderedSetImpl[T]
	lastIdx int
	value   *T
}

func (iter *orderedSetIterImpl[T]) Next(ctx context.Context) bool {
	for i := iter.lastIdx + 1; i < len(iter.set.elems); i++ {
		if ctx.Err() != nil {
			return false
		}
		if iter.set.elems[i].deleted {
			continue
		}
		if i >= len(iter.set.elems) {
			return false
		}
		iter.lastIdx = i
		iter.value = &iter.set.elems[i].item
		return true
	}
	return false
}

func (iter *orderedSetIterImpl[T]) Value() T                      { return *iter.value }
func (iter *orderedSetIterImpl[T]) Close(_ context.Context) error { return nil }
