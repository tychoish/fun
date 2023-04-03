package adt

import (
	"runtime"
	"sync"

	"github.com/tychoish/fun"
)

// Pool is an ergonomic wrapper around sync.Pool that provides some
// additional functionality: generic type interface, a default clean
// up hook to modify (optionally) the object before returning it to
// the pool.
//
// Additionally, the Make() method attaches an object finalizer to the
// object produced by the pool that returns the object to the pool
// rather than garbage collect it. This is likely to be less efficient
// than return the objects to the pool using defer functions, but may
// be more ergonomic in some situations.
//
// Pool can be used with default construction; however, you should set
// the Constructor before calling Get() or Make() on the pool.
type Pool[T any] struct {
	Constructor fun.Atomic[func() T]
	once        sync.Once
	hook        *fun.Atomic[func(T) T]
	pool        *sync.Pool
}

func (p *Pool[T]) init() {
	p.once.Do(func() {
		p.hook = fun.NewAtomic(func(in T) T { return in })
		p.pool = &sync.Pool{
			New: func() any {
				return p.Constructor.Get()()
			},
		}
	})
}

// SetCleanupHook sets a function to be called on every object
// renetering the pool. By default, the cleanup function is a noop.
func (p *Pool[T]) SetCleanupHook(in func(T) T) {
	p.init()
	if in == nil {
		return
	}
	p.hook.Set(in)
}

// Get returns an object from the pool or constructs a default object
// according to the constructor.
func (p *Pool[T]) Get() T { p.init(); return p.pool.Get().(T) }

// Put returns an object in the pool, calling the cleanuphook if
// set. Put *always* clears the object's finalizer before calling the
// cleanuphook or returning it to the pool.
func (p *Pool[T]) Put(in T) { p.init(); p.pool.Put(p.hook.Get()(in)) }

// Make gets an object out of the sync.Pool, and attaches a finalizer
// that returns the item to the pool when the object would be garbage
// collected.
//
// Finalizer hooks are not automatically cleared by the Put()
// operation, so objects retrieved with Make should not be passed
// manually to Put().
func (p *Pool[T]) Make() T { p.init(); o := p.pool.Get().(T); runtime.SetFinalizer(o, p.Put); return o }
