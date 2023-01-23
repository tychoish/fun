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
	Iterator(context.Context) fun.Iterator[T]
}

// MakeUnordered constructs a set object, pre-allocating the specified
// length. Iteration order is randomized.
func MakeUnordered[T comparable](len int) Set[T] { return make(mapSetImpl[T], len) }

// NewUnordered constructs a set object for the given type, without prealocation.
func NewUnordered[T comparable]() Set[T] { return MakeUnordered[T](0) }

type mapSetImpl[T comparable] map[T]struct{}

func (s mapSetImpl[T]) Add(item T)        { s[item] = struct{}{} }
func (s mapSetImpl[T]) Len() int          { return len(s) }
func (s mapSetImpl[T]) Delete(item T)     { delete(s, item) }
func (s mapSetImpl[T]) Check(item T) bool { _, ok := s[item]; return ok }
func (s mapSetImpl[T]) Iterator(ctx context.Context) fun.Iterator[T] {
	pipe := make(chan T)

	iter := &internal.MapIterImpl[T]{
		ChannelIterImpl: internal.ChannelIterImpl[T]{Pipe: pipe},
	}
	ctx, iter.Closer = context.WithCancel(ctx)
	iter.WG.Add(1)

	go func() {
		defer iter.WG.Done()
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

type syncSetImpl[T comparable] struct {
	mtx *sync.RWMutex
	set Set[T]
}

// Synchronize wraps an existing set instance with a
// mutex. The underlying implementation provides an Unwrap method.
func Synchronize[T comparable](s Set[T]) Set[T] {
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

type syncIterImpl[T any] struct {
	// this is duplicated from itertool to avoid an import cycle
	mtx  *sync.RWMutex
	iter fun.Iterator[T]
}

func (iter syncIterImpl[T]) Unwrap() fun.Iterator[T] { return iter.iter }
func (iter syncIterImpl[T]) Next(ctx context.Context) bool {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Next(ctx)
}

func (iter syncIterImpl[T]) Close(ctx context.Context) error {
	iter.mtx.Lock()
	defer iter.mtx.Unlock()

	return iter.iter.Close(ctx)
}

func (iter syncIterImpl[T]) Value() T {
	iter.mtx.RLock()
	defer iter.mtx.RUnlock()

	return iter.iter.Value()
}

func (s syncSetImpl[T]) Iterator(ctx context.Context) fun.Iterator[T] {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	return syncIterImpl[T]{mtx: s.mtx, iter: s.set.Iterator(ctx)}
}
