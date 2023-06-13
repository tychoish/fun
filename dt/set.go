package dt

import (
	"context"
	"sync"
	stdatomic "sync/atomic"

	"github.com/tychoish/fun"
)

type atomic[T comparable] struct{ val stdatomic.Value }

func (a *atomic[T]) Get() T        { return a.resolve(a.val.Load()) }
func (a *atomic[T]) Set(in T) bool { return a.val.CompareAndSwap(nil, in) }

func (*atomic[T]) resolve(in any) (val T) {
	switch c := in.(type) {
	case T:
		val = c
	}
	return val

}

type Set[T comparable] struct {
	hash Map[T, *Element[T]]
	list *List[T]
	mtx  atomic[*sync.Mutex]
}

func (s *Set[T]) Synchronize() { s.mtx.Set(&sync.Mutex{}) }
func (s *Set[T]) Order() {
	defer s.with(s.lock())
	if s.list != nil {
		return
	}
	fun.Invariant(len(s.hash) == 0, "cannot make an ordered set out of an un-ordered set that contain data")
	s.list = &List[T]{}
}

func (s *Set[T]) WithLock(mtx *sync.Mutex) {
	fun.Invariant(mtx != nil, "mutexes must be non-nil")
	fun.Invariant(s.mtx.Set(mtx), "cannot override an existing mutex")
}

func (s *Set[T]) isOrdered() bool { return s.list != nil }

func (s *Set[T]) init()            { s.hash = Map[T, *Element[T]]{} }
func (*Set[T]) with(m *sync.Mutex) { fun.WhenCall(m != nil, m.Unlock) }
func (s *Set[T]) lock() *sync.Mutex {
	m := s.mtx.Get()
	fun.WhenCall(m != nil, m.Lock)
	fun.WhenCall(s.hash == nil, s.init)
	return m
}

func (s *Set[T]) Add(in T)                   { _ = s.AddCheck(in) }
func (s *Set[T]) Len() int                   { defer s.with(s.lock()); return len(s.hash) }
func (s *Set[T]) Check(in T) bool            { defer s.with(s.lock()); return s.hash.Check(in) }
func (s *Set[T]) Delete(in T)                { defer s.with(s.lock()); _ = s.delete(in) }
func (s *Set[T]) DeleteCheck(in T) bool      { defer s.with(s.lock()); return s.delete(in) }
func (s *Set[T]) Iterator() *fun.Iterator[T] { defer s.with(s.lock()); return s.unsafeIterator() }
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

func (s *Set[T]) Populate(iter *fun.Iterator[T]) {
	fun.InvariantMust(iter.Observe(context.Background(), s.Add))
}

func (s *Set[T]) delete(in T) bool {
	e, ok := s.hash.Load(in)
	if !ok {
		return false
	}

	fun.WhenDo(e != nil, e.Remove)
	delete(s.hash, in)
	return true
}

func (s *Set[T]) unsafeIterator() *fun.Iterator[T] {
	if s.list != nil {
		return s.list.Iterator()
	}
	return s.hash.Keys()
}

func (s *Set[T]) Producer() fun.Producer[T] {
	defer s.with(s.lock())

	if s.list != nil {
		return s.list.Producer()
	}

	return s.hash.Keys().Producer()
}

// Equal tests two sets, returning true if the items in the sets have
// equal values. If the iterators are ordered, order is
// considered.
func (s *Set[T]) Equal(other *Set[T]) bool {
	defer s.with(s.lock())

	if len(s.hash) != other.Len() || s.isOrdered() != other.isOrdered() {
		return false
	}

	ctx := context.Background()
	iter := s.unsafeIterator()

	if s.isOrdered() {
		otherIter := other.Iterator()
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

func (s *Set[T]) MarshalJSON() ([]byte, error) { return s.Iterator().MarshalJSON() }

func (s *Set[T]) UnmarshalJSON(in []byte) error {
	iter := Sliceify([]T{}).Iterator()
	iter.UnmarshalJSON(in)
	s.Populate(iter)
	return iter.Close()
}
