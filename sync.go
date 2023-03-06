package fun

import (
	"context"
	"sync"
	"sync/atomic"
)

// Wait returns when the all waiting items are done, *or* the context
// is canceled. This operation will leak a go routine if the WaitGroup
// never returns and the context is canceled.
//
// fun.Wait(wg) is equivalent to fun.WaitGroup(wg)(ctx)
func Wait(ctx context.Context, wg *sync.WaitGroup) { WaitGroup(wg)(ctx) }

// Atomic is a very simple atomic Get/Set operation, providing a
// generic type-safe implementation wrapping
// sync/atomic.Value.
type Atomic[T any] struct{ atomic.Value }

// NewAtomic creates a new Atomic Get/Set value with the initial value
// already set.
func NewAtomic[T any](initial T) *Atomic[T] { a := &Atomic[T]{}; a.Set(initial); return a }

// Set atomically sets the value of the Atomic.
func (a *Atomic[T]) Set(in T) { a.Value.Store(in) }

// Get resolves the atomic value, returning the zero value of the type
// T if the value is unset.
func (a *Atomic[T]) Get() T { return ZeroWhenNil[T](a.Value.Load()) }
