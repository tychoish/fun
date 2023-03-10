// Package Set provides ordered and unordered set implementations for
// arbitrary comparable types.
package set

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
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
	// elements in the set.
	Iterator() fun.Iterator[T]
}

// MakeUnordered constructs a set object, pre-allocating the specified
// length. Iteration order is randomized.
func MakeUnordered[T comparable](len int) Set[T] { return make(mapSetImpl[T], len) }

// NewUnordered constructs a set object for the given type, without prealocation.
func NewUnordered[T comparable]() Set[T] { return MakeUnordered[T](0) }

// PopulateSet adds all elements in the iterator to the provided Set.
func PopulateSet[T comparable](ctx context.Context, set Set[T], iter fun.Iterator[T]) {
	fun.Observe(ctx, iter, set.Add)
}

// BuildUnordered produces a new unordered set from the elements in
// the iterator.
func BuildUnordered[T comparable](ctx context.Context, iter fun.Iterator[T]) Set[T] {
	set := NewUnordered[T]()
	PopulateSet(ctx, set, iter)
	return set
}

type mapSetImpl[T comparable] map[T]struct{}

func (s mapSetImpl[T]) Add(item T)        { s[item] = struct{}{} }
func (s mapSetImpl[T]) Len() int          { return len(s) }
func (s mapSetImpl[T]) Delete(item T)     { delete(s, item) }
func (s mapSetImpl[T]) Check(item T) bool { return checkInMap(item, s) }
func (s mapSetImpl[T]) Iterator() fun.Iterator[T] {
	pipe := make(chan T)

	iter := &internal.MapIterImpl[T]{
		ChannelIterImpl: internal.ChannelIterImpl[T]{Pipe: pipe},
	}
	iter.Ctx, iter.Closer = context.WithCancel(internal.BackgroundContext)
	iter.WG.Add(1)

	go func() {
		defer iter.WG.Done()
		defer close(pipe)

		for item := range s {
			select {
			case <-iter.Ctx.Done():
				return
			case pipe <- item:
				continue
			}
		}
	}()

	return iter
}

type syncSetImpl[T comparable] struct {
	mtx *sync.Mutex
	set Set[T]
}

// Synchronize wraps an existing set instance with a
// mutex. The underlying implementation provides an Unwrap method.
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

type syncIterImpl[T any] struct {
	// this is duplicated from itertool to avoid an import cycle
	mtx  sync.Locker
	iter fun.Iterator[T]
}

func (s syncSetImpl[T]) Iterator() fun.Iterator[T] {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return syncIterImpl[T]{mtx: s.mtx, iter: s.set.Iterator()}
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.iter }
func (iter syncIterImpl[T]) Next(ctx context.Context) bool {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Next(ctx)
}

func (iter syncIterImpl[T]) Close() error {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Close()
}

func (iter syncIterImpl[T]) Value() T {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Value()
}
