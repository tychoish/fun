package adt

import (
	"fmt"
	"sync"

	"github.com/tychoish/fun/fn"
)

// AccessorsWithLock takes a getter/setter pair and configures both
// with a shared mutex: all read/write operations are fully
// synchronized with regards to eachother.
func AccessorsWithLock[T any](getter fn.Future[T], setter fn.Handler[T]) (fn.Future[T], fn.Handler[T]) {
	lock := &sync.Mutex{}
	return getter.WithLock(lock), setter.WithLock(lock)
}

// AccessorsWithReadLock takes a getter/setter pair and configures
// them with a rw-mutex: the getter (Future) uses the read lock, while
// the setter is write-locked.
func AccessorsWithReadLock[T any](getter fn.Future[T], setter fn.Handler[T]) (fn.Future[T], fn.Handler[T]) {
	lock := &sync.RWMutex{}
	return getter.WithLocker(lock.RLocker()), setter.WithLocker(lock)
}

// Synchronized wraps an arbitrary type with a lock, and provides a
// functional interface for interacting with that type. In general
// Synchronize is ideal for container and interface types which are
// not safe for concurrent use, when either `adt.Map`, `adt.Atomic`
// are not appropriate.
type Synchronized[T any] struct {
	mtx sync.Mutex
	obj T
}

// Lock takes the lock, locks it, and then returns it.
//
// This, in combination With() makes it possible to have a single
// statement for managing a mutex in a defer, given the evaluation
// time of defer arguments, as in:
//
//	mtx := &sync.Mutex{}
//	defer adt.With(adt.Lock(mtx))
func Lock(mtx *sync.Mutex) *sync.Mutex { mtx.Lock(); return mtx }

// With takes a lock as an argument and then releases the lock when it
// executes.
//
// This, in combination with Lock makes it possible to have a single
// statement for managing a mutex in a defer, given the evaluation
// time of defer arguments, as in:
//
//	mtx := &sync.Mutex{}
//	defer adt.With(adt.Lock(mtx))
func With(mtx *sync.Mutex) { mtx.Unlock() }

func WithR(m *sync.RWMutex)               { m.RUnlock() }
func LockR(m *sync.RWMutex) *sync.RWMutex { m.RLock(); return m }
func WithW(m *sync.RWMutex)               { m.Unlock() }
func LockW(m *sync.RWMutex) *sync.RWMutex { m.Lock(); return m }

// NewSynchronized constructs a new synchronized object that wraps the
// input type.
func NewSynchronized[T any](in T) *Synchronized[T] { return &Synchronized[T]{obj: in} }

// With runs the input function within the lock, to mutate the object.
func (s *Synchronized[T]) With(in func(obj T)) { s.Using(func() { in(s.obj) }) }

// Set overrides the current value of the protected object. Use with
// caution.
func (s *Synchronized[T]) Set(in T)   { s.Store(in) }
func (s *Synchronized[T]) Store(in T) { s.Using(func() { s.obj = in }) }

// String implements fmt.Stringer using this type.
func (s *Synchronized[T]) String() string { return fmt.Sprint(s.Get()) }

// Get returns the underlying protected object. Use with caution.
func (s *Synchronized[T]) Get() T  { return s.Load() }
func (s *Synchronized[T]) Load() T { defer With(Lock(&s.mtx)); return s.obj }

// Using runs the provided operation while holding the lock, but
// without providing access to the locked value.
func (s *Synchronized[T]) Using(op func()) { defer With(Lock(&s.mtx)); op() }

// Swap sets the locked value to the new value and returns the old.
func (s *Synchronized[T]) Swap(newVal T) (old T) {
	s.Using(func() { old = s.obj; s.obj = newVal })
	return
}
