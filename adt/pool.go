package adt

import (
	"reflect"
	"runtime"
	"sync"
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
	once        sync.Once
	hook        *Atomic[func(T) T]
	constructor *Atomic[func() T]
	pool        *sync.Pool
}

func (p *Pool[T]) init() { p.once.Do(p.doInit) }

func (p *Pool[T]) doInit() {
	p.hook = NewAtomic(func(in T) T { return in })
	p.constructor = NewAtomic(func() (out T) { return out })
	p.pool = &sync.Pool{New: func() any { return p.constructor.Get()() }}
}

// SetCleanupHook sets a function to be called on every object
// renetering the pool. By default, the cleanup function is a noop.
func (p *Pool[T]) SetCleanupHook(in func(T) T) {
	p.init()
	if in != nil {
		p.hook.Set(in)
	}
}

// SetConstructor overrides the default constructor (which makes an
// object with a Zero value by default) for Get/Make operations.
func (p *Pool[T]) SetConstructor(in func() T) {
	p.init()
	if in != nil {
		p.constructor.Set(in)
	}
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
func (p *Pool[T]) Make() T {
	p.init()
	o := p.pool.Get().(T)
	if reflect.ValueOf(o).Kind() == reflect.Pointer {
		runtime.SetFinalizer(o, p.Put)
	} else {
		runtime.SetFinalizer(&o, func(in *T) { p.Put(*in) })
	}

	return o
}
