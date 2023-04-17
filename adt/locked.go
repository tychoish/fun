package adt

import "sync"

// Synchronized wraps an arbitrary type with a lock, and provides a
// functional interface for interacting with that type. In general
// Synchronize is ideal for container types which are not safe for
// concurrent use, when either `adt.Map`, `adt.Atomic` are not
// appropriate.
type Synchronized[T any] struct {
	mtx sync.Mutex
	obj T
}

// NewSynchronized constructs a new synchronized object that wraps the
// input type.
func NewSynchronized[T any](in T) *Synchronized[T] { return &Synchronized[T]{obj: in} }

// With runs the input function within the lock, to mutate the object.
func (s *Synchronized[T]) With(in func(obj T)) { defer s.withLock()(); in(s.obj) }

// Set overrides the current value of the protected object. Use with
// caution.
func (s *Synchronized[T]) Set(in T) { defer s.withLock()(); s.obj = in }

// withLock provides a terse
func (s *Synchronized[T]) withLock() func() { s.mtx.Lock(); return s.mtx.Unlock }
