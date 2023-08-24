package adt

import (
	"bytes"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
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
	locked      atomic.Bool
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

// Lock sets the pool to locked. Once set it cannot be unset: future
// attempts to set the constructor or cleanup hook result in a panic
// and invariant violation.
func (p *Pool[T]) Lock() { p.locked.Store(true) }

// SetCleanupHook sets a function to be called on every object
// renetering the pool. By default, the cleanup function is a noop.
func (p *Pool[T]) SetCleanupHook(in func(T) T) {
	p.init()
	fun.Invariant.IsFalse(p.locked.Load(), "cannot modify hook of locked pool")
	if in != nil {
		p.hook.Set(in)
	}
}

// SetConstructor overrides the default constructor (which makes an
// object with a Zero value by default) for Get/Make operations.
func (p *Pool[T]) SetConstructor(in func() T) {
	p.init()
	fun.Invariant.IsFalse(p.locked.Load(), "cannot modify constructor of locked pool")
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
func (p *Pool[T]) Put(in T) {
	p.init()
	if ft.IsNil(in) {
		return
	}
	p.pool.Put(p.hook.Get()(in))
}

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

// MakeBytesBufferPool configures a pool of *bytes.Buffers. Buffers
// are always reset before reentering the pool, and are pre-allocated
// with the specified capacity. Negative initial capcity values are
// ignored.
func MakeBytesBufferPool(capacity int) *Pool[*bytes.Buffer] {
	bufpool := &Pool[*bytes.Buffer]{}
	bufpool.SetCleanupHook(func(buf *bytes.Buffer) *bytes.Buffer { buf.Reset(); return buf })
	bufpool.SetConstructor(func() *bytes.Buffer { return bytes.NewBuffer(make([]byte, 0, intish.Max(0, capacity))) })
	bufpool.Lock()
	return bufpool
}

// MakeBufferPool constructs a pool of byte slices. New slices are
// allocated with the specified minimum capacity, and are always
// resliced to be 0 length before reentering the pool. Slices that are
// larger than the specified maximum do not reenter the pool.
//
// Min/Max values less than 0 are ignored, and the highest value is
// always used as the max the lowest as the min, regardless of
// position. MakeBufferPool panics with an invariant violation if the
// max capacity value is zero.
func MakeBufferPool(min, max int) *Pool[[]byte] {
	min, max = intish.Bounds(min, max)
	fun.Invariant.OK(max > 0, "buffer pool capacity max cannot be zero")
	bufpool := &Pool[[]byte]{}
	bufpool.SetCleanupHook(func(buf []byte) []byte {
		if cap(buf) > max {
			return nil
		}
		return buf[:0]
	})
	bufpool.SetConstructor(func() []byte { return make([]byte, 0, min) })
	bufpool.Lock()

	return bufpool
}

// DefaultBufferPool creates a pool of byte slices with a maximum size
// of 64kb. All other slices are discarded. These are the same
// settings as used by the fmt package's buffer pool.
func DefaultBufferPool() *Pool[[]byte] { return MakeBufferPool(0, 64*1024) }
