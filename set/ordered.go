package set

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
)

type orderedLLSet[T comparable] struct {
	set   fun.Map[T, *seq.Element[T]]
	elems seq.List[T]
}

// MakeNewOrdered constructs a "new" ordered set implementation. This
// is a more simple implementation based around a linked list, with
// identical semantics. Items are deleted from the list
// synchronously--the "old" implementation does a lazy deletion that
// batches--and this implementation may perform more predictably for
// use cases that do a large number of deletions.
//
// This implementation does not permit iteration with concurrent
// deletes, as is the case with the old ordered implementation.
func MakeNewOrdered[T comparable]() Set[T] {
	return &orderedLLSet[T]{set: fun.Map[T, *seq.Element[T]]{}}
}

// BuildOrdered creates an ordered set (new implementation) from
// the contents of the input iterator.
func BuildOrdered[T comparable](ctx context.Context, iter fun.Iterator[T]) Set[T] {
	set := MakeNewOrdered[T]()
	Populate(ctx, set, iter)
	return set
}

func (lls *orderedLLSet[T]) Add(it T) {
	if lls.set.Check(it) {
		return
	}

	lls.elems.PushBack(it)

	lls.set[it] = lls.elems.Back()
}

func (lls *orderedLLSet[T]) Iterator() fun.Iterator[T] { return seq.ListValues(lls.elems.Iterator()) }
func (lls *orderedLLSet[T]) Len() int                  { return lls.elems.Len() }
func (lls *orderedLLSet[T]) Check(it T) bool           { return lls.set.Check(it) }
func (lls *orderedLLSet[T]) Delete(it T) {
	e, ok := lls.set.Load(it)
	if !ok {
		return
	}

	e.Remove()
	delete(lls.set, it)
}

func (lls *orderedLLSet[T]) MarshalJSON() ([]byte, error) {
	return itertool.MarshalJSON(internal.BackgroundContext, lls.Iterator())
}

func (lls *orderedLLSet[T]) UnmarshalJSON(in []byte) error {
	iter := itertool.UnmarshalJSON[T](in)
	Populate[T](internal.BackgroundContext, lls, iter)
	return iter.Close()
}

type orderedSetItem[T comparable] struct {
	idx     int
	item    T
	deleted bool
}

type orderedSetImpl[T comparable] struct {
	elems        []orderedSetItem[T]
	set          fun.Map[T, orderedSetItem[T]]
	deletedCount int
}

// NewOrdered produces an order-preserving set
// implementation. Iteration order will reflect insertion order.
func NewOrdered[T comparable]() Set[T] {
	return MakeOrdered[T](0)
}

// MakeOrdered produces an order-preserving set implementation, with
// pre-allocated capacity to the specified size. Iteration will
// reflect insertion order.
func MakeOrdered[T comparable](size int) Set[T] {
	return &orderedSetImpl[T]{
		set:   fun.Mapify(make(map[T]orderedSetItem[T], size)),
		elems: make([]orderedSetItem[T], 0, size),
	}
}

func (s *orderedSetImpl[T]) lazyDelete() {
	if s.deletedCount < 64 {
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

func (s *orderedSetImpl[T]) Check(in T) bool { return s.set.Check(in) }
func (s *orderedSetImpl[T]) Len() int        { return len(s.set) }
func (s *orderedSetImpl[T]) Delete(it T) {
	val, ok := s.set[it]
	if !ok {
		return
	}
	s.elems[val.idx].deleted = true
	s.elems[val.idx].item = fun.ZeroOf[T]() // zero to release memory
	delete(s.set, val.item)
	s.deletedCount++

	s.lazyDelete()
}

func (s *orderedSetImpl[T]) Iterator() fun.Iterator[T] {
	s.lazyDelete()
	return &orderedSetIterImpl[T]{set: s, lastIdx: -1}
}

func (s *orderedSetImpl[T]) MarshalJSON() ([]byte, error) {
	return itertool.MarshalJSON(internal.BackgroundContext, s.Iterator())
}

func (s *orderedSetImpl[T]) UnmarshalJSON(in []byte) error {
	iter := itertool.UnmarshalJSON[T](in)
	Populate[T](internal.BackgroundContext, s, iter)
	return iter.Close()
}

type orderedSetIterImpl[T comparable] struct {
	set     *orderedSetImpl[T]
	lastIdx int
	value   *T
}

func (iter *orderedSetIterImpl[T]) Next(ctx context.Context) bool {
	for i := iter.lastIdx + 1; i < len(iter.set.elems); i++ {
		if ctx.Err() != nil || i >= len(iter.set.elems) {
			return false
		}

		if iter.set.elems[i].deleted {
			continue
		}

		iter.lastIdx = i
		iter.value = &iter.set.elems[i].item
		return true
	}
	return false
}

func (iter *orderedSetIterImpl[T]) Value() T     { return *iter.value }
func (iter *orderedSetIterImpl[T]) Close() error { return nil }
