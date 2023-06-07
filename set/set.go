// Package Set provides ordered and unordered set implementations for
// arbitrary comparable types.
package set

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/itertool"
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
	// Iterator produces an Iterator over the
	// elements in the set.
	Iterator() *fun.Iterator[T]
	Producer() fun.Producer[T]
}

// MakeUnordered constructs a set object, pre-allocating the specified
// length. Iteration order is randomized.
func MakeUnordered[T comparable](len int) Set[T] { return make(mapSetImpl[T], len) }

// NewUnordered constructs a set object for the given type, without prealocation.
func NewUnordered[T comparable]() Set[T] { return MakeUnordered[T](0) }

// Populate adds all elements in the iterator to the provided Set.
func Populate[T comparable](ctx context.Context, set Set[T], iter *fun.Iterator[T]) {
	fun.InvariantMust(iter.Observe(ctx, set.Add))
}

// BuildUnordered produces a new unordered set from the elements in
// the iterator.
func BuildUnordered[T comparable](ctx context.Context, iter *fun.Iterator[T]) Set[T] {
	set := NewUnordered[T]()
	Populate(ctx, set, iter)
	return set
}

// BuildOrderedFromPairs produces an unordered set from a sequence of pairs.
func BuildOrderedFromPairs[K, V comparable](pairs fun.Pairs[K, V]) Set[fun.Pair[K, V]] {
	return BuildOrdered(internal.BackgroundContext, fun.Sliceify(pairs).Iterator())
}

// BuildUnorderedFromPairs produces an order-preserving set based on a
// sequence of Pairs.
func BuildUnorderedFromPairs[K, V comparable](pairs fun.Pairs[K, V]) Set[fun.Pair[K, V]] {
	return BuildUnordered(internal.BackgroundContext, fun.Sliceify(pairs).Iterator())
}

type mapSetImpl[T comparable] fun.Map[T, struct{}]

func (s mapSetImpl[T]) Producer() fun.Producer[T] { return fun.Map[T, struct{}](s).Keys().Producer() }
func (s mapSetImpl[T]) Add(item T)                { s[item] = struct{}{} }
func (s mapSetImpl[T]) Len() int                  { return len(s) }
func (s mapSetImpl[T]) Delete(item T)             { delete(s, item) }
func (s mapSetImpl[T]) Check(item T) bool         { _, ok := s[item]; return ok }
func (s mapSetImpl[T]) Iterator() *fun.Iterator[T] {
	pipe := make(chan T)

	setup := fun.Operation(func(ctx context.Context) {
		defer close(pipe)
		for item := range s {
			if !fun.Blocking(pipe).Send().Check(ctx, item) {
				return
			}
		}
	}).Launch().Once()

	return fun.Generator(func(ctx context.Context) (T, error) {
		setup(ctx)
		return fun.Blocking(pipe).Receive().Read(ctx)
	})
}

func (s mapSetImpl[T]) MarshalJSON() ([]byte, error) {
	return itertool.MarshalJSON(internal.BackgroundContext, s.Iterator())
}

func (s mapSetImpl[T]) UnmarshalJSON(in []byte) error {
	iter := itertool.UnmarshalJSON[T](in)
	Populate[T](internal.BackgroundContext, s, iter)
	return iter.Close()
}

type syncSetImpl[T comparable] struct {
	mtx *sync.Mutex
	set Set[T]
}

// Synchronize wraps an existing set instance with a mutex. The
// underlying implementation provides an Unwrap method. Additionally
// the iterator implementation uses the adt package's synchronized
// iterator, which is handled specially by the `fun.IterateOne`
// function and a number of tools which use it.
func Synchronize[T comparable](s Set[T]) Set[T] {
	return syncSetImpl[T]{
		set: s,
		mtx: &sync.Mutex{},
	}
}

func (s syncSetImpl[T]) Unwrap() Set[T] {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.set
}

func (s syncSetImpl[T]) Add(in T) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.set.Add(in)
}

func (s syncSetImpl[T]) Len() int {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.set.Len()
}

func (s syncSetImpl[T]) Check(item T) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.set.Check(item)
}

func (s syncSetImpl[T]) Delete(in T) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.set.Delete(in)
}

func (s syncSetImpl[T]) Iterator() *fun.Iterator[T] { return s.Producer().Iterator() }

func (s syncSetImpl[T]) Producer() fun.Producer[T] {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.set.Producer().WithLock(s.mtx)
}

func (s syncSetImpl[T]) MarshalJSON() ([]byte, error) {
	return itertool.MarshalJSON(internal.BackgroundContext, s.Iterator())
}

func (s syncSetImpl[T]) UnmarshalJSON(in []byte) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return itertool.UnmarshalJSON[T](in).Observe(internal.BackgroundContext, s.set.Add)
}
