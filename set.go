package fun

import (
	"context"
	"sync"
)

// Set describes a basic set interface, and fun provdies a
// straightforward implementation backed by a `map[T]struct{}`, but
// other implementations are possible.
type Set[T comparable] interface {
	// Add unconditionally adds an item to the set.
	Add(T)
	// Len Returns the number of items in the set
	Len() int
	// Delete removes an item from the set.
	Delete(T)
	// Check returns true if the item is in the set.
	Check(T) bool
	// Iterator produces an Iterator implementation for the
	// elements in the set. There are no ordering guarantees.
	Iterator(context.Context) Iterator[T]
}

// MakeSet constructs a set object, pre-allocating the specified
// length.
func MakeSet[T comparable](len int) Set[T] { return make(mapSetImpl[T], len) }

// NewSet constructs a set object for the given type, without prealocation.
func NewSet[T comparable]() Set[T] { return MakeSet[T](0) }

type mapSetImpl[T comparable] map[T]struct{}

func (s mapSetImpl[T]) Add(item T)        { s[item] = struct{}{} }
func (s mapSetImpl[T]) Len() int          { return len(s) }
func (s mapSetImpl[T]) Delete(item T)     { delete(s, item) }
func (s mapSetImpl[T]) Check(item T) bool { _, ok := s[item]; return ok }
func (s mapSetImpl[T]) Iterator(ctx context.Context) Iterator[T] {
	pipe := make(chan T)

	iter := &mapIterImpl[T]{
		channelIterImpl: channelIterImpl[T]{pipe: pipe},
	}
	ctx, iter.closer = context.WithCancel(ctx)
	iter.wg.Add(1)

	go func() {
		defer iter.wg.Done()
		defer close(pipe)

		for item := range s {
			select {
			case <-ctx.Done():
				return
			case pipe <- item:
				continue
			}
		}
	}()

	return iter
}

// Pair represents a key-value pair.
type Pair[K comparable, V comparable] struct {
	Key   K
	Value V
}

// Pairs implements a collection of key-value pairs.
type Pairs[K comparable, V comparable] []Pair[K, V]

// Add adds a new value to the underlying slice.
func (p *Pairs[K, V]) Add(k K, v V) { *p = p.Append(Pair[K, V]{Key: k, Value: v}) }

// Append, mirroring the semantics of the built in append() function
// adds one or more Pair items to a Pairs slice, and returns the new
// slice without changing the value of the original slice:
//
//	p = p.Append(pair, pare, pear)
func (p Pairs[K, V]) Append(new ...Pair[K, V]) Pairs[K, V] { return append(p, new...) }

// MakePairs converts a map type into a slice of Pair types
// that can be usable in a set.
func MakePairs[K comparable, V comparable](in map[K]V) Pairs[K, V] {
	out := make([]Pair[K, V], 0, len(in))
	for k, v := range in {
		out = append(out, Pair[K, V]{Key: k, Value: v})
	}
	return out
}

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p Pairs[K, V]) Map() map[K]V {
	out := make(map[K]V, len(p))
	for idx := range p {
		if _, ok := out[p[idx].Key]; ok {
			continue
		}
		out[p[idx].Key] = p[idx].Value
	}
	return out
}

// Set converts a Pairs object into a set.
func (p Pairs[K, V]) Set() Set[Pair[K, V]] {
	set := MakeSet[Pair[K, V]](len(p))

	for idx := range p {
		set.Add(p[idx])
	}

	return set
}

// OrderedSet produces an order-preserving set based on the Pairs.
func (p Pairs[K, V]) OrderedSet() Set[Pair[K, V]] {
	set := MakeOrderedSet[Pair[K, V]](len(p))

	for idx := range p {
		if set.Check(p[idx]) {
			continue
		}
		set.Add(p[idx])
	}

	return set
}

type syncSetImpl[T comparable] struct {
	mtx *sync.RWMutex
	set Set[T]
}

// MakeSynchronizedSet wraps an existing set instance with a
// mutex. The underlying implementation provides an Unwrap method.
func MakeSynchronizedSet[T comparable](s Set[T]) Set[T] {
	return syncSetImpl[T]{
		set: s,
		mtx: &sync.RWMutex{},
	}
}

func (s syncSetImpl[T]) Unwrap() Set[T] {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.set
}

func (s syncSetImpl[T]) Add(in T) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.set.Add(in)
}

func (s syncSetImpl[T]) Len() int {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.set.Len()
}

func (s syncSetImpl[T]) Check(item T) bool {
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	return s.set.Check(item)
}

func (s syncSetImpl[T]) Delete(in T) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.set.Delete(in)
}

func (s syncSetImpl[T]) Iterator(ctx context.Context) Iterator[T] {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return syncIterImpl[T]{
		iter: s.set.Iterator(ctx),
		mtx:  s.mtx,
	}
}
