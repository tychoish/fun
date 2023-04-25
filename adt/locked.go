package adt

import (
	"fmt"
	"sync"
)

// Synchronized wraps an arbitrary type with a lock, and provides a
// functional interface for interacting with that type. In general
// Synchronize is ideal for container and interface types which are
// not safe for concurrent use, when either `adt.Map`, `adt.Atomic`
// are not appropriate.
type Synchronized[T any] struct {
	mtx sync.Mutex
	obj T
}

// withLock provides a terse way of getting and release a.
func (s *Synchronized[T]) withLock() func() { s.mtx.Lock(); return s.mtx.Unlock }

// NewSynchronized constructs a new synchronized object that wraps the
// input type.
func NewSynchronized[T any](in T) *Synchronized[T] { return &Synchronized[T]{obj: in} }

// With runs the input function within the lock, to mutate the object.
func (s *Synchronized[T]) With(in func(obj T)) { defer s.withLock()(); in(s.obj) }

// Set overrides the current value of the protected object. Use with
// caution.
func (s *Synchronized[T]) Set(in T) { defer s.withLock()(); s.obj = in }

// String implements fmt.Stringer using this type.
func (s *Synchronized[T]) String() string { return fmt.Sprint(s.Get()) }

// Get returns the underlying protected object. Use with caution.
func (s *Synchronized[T]) Get() T { defer s.withLock()(); return s.obj }

// Swap sets the locked value to the new value and returns the old.
func (s *Synchronized[T]) Swap(new T) (old T) {
	defer s.withLock()()
	item := s.obj
	s.obj = new
	old = item
	return
}
