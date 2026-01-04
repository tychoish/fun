package adt

import (
	"bytes"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/intish"
	"github.com/tychoish/fun/stw"
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
	typeIsPtr   bool
	hook        *Atomic[func(T) T]
	constructor *Atomic[func() T]
	pool        *sync.Pool
}

func (p *Pool[T]) init() { p.once.Do(p.doInit) }

func (p *Pool[T]) doInit() {
	p.hook = NewAtomic(ft.Noop[T])
	p.constructor = NewAtomic(ft.Zero[T])
	p.pool = &sync.Pool{New: func() any { return p.constructor.Get()() }}
	var zero T
	p.typeIsPtr = ft.IsPtr(zero)
}

// FinalizeSetup prevents future calls from setting the constructor or
// cleanup hooks. Once a pool's setup has been finalized it cannot be
// unset: future attempts to set the constructor or cleanup hook
// result in a panic and invariant violation. FinalizeSetup is safe to
// cull multiple times and from different go routines.
func (p *Pool[T]) FinalizeSetup() { p.init(); p.locked.Store(true) }

// SetCleanupHook sets a function to be called on every object
// renetering the pool. By default, the cleanup function is a noop,
// and if the input function is nil, it is not set.
func (p *Pool[T]) SetCleanupHook(in func(T) T) {
	p.init()
	erc.InvariantOk(ft.Not(p.locked.Load()), "SetCleaupHook", "after FinalizeSetup", ers.ErrImmutabilityViolation)
	ft.ApplyWhen(in != nil, p.hook.Set, in)
}

// SetConstructor overrides the default constructor (which makes an
// object with a Zero value by default) for Get/Make operations.
func (p *Pool[T]) SetConstructor(in func() T) {
	p.init()
	erc.InvariantOk(ft.Not(p.locked.Load()), "SetConstructor", "after FinalizeSetup", ers.ErrImmutabilityViolation)
	ft.ApplyWhen(in != nil, p.constructor.Set, in)
}

// Get returns an object from the pool or constructs a default object
// according to the constructor.
func (p *Pool[T]) Get() T { p.init(); return p.pool.Get().(T) }

// Put returns an object in the pool, calling the cleanuphook if
// set. Put *always* clears the object's finalizer before calling the
// cleanuphook or returning it to the pool.
func (p *Pool[T]) Put(in T) {
	p.init()

	if any(in) != nil {
		p.pool.Put(p.hook.Get()(in))
	}
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

	if p.typeIsPtr {
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
	_buf := MakeBufferPool(capacity, 64*1024)
	bufpool := &Pool[*bytes.Buffer]{}
	bufpool.SetCleanupHook(func(buf *bytes.Buffer) *bytes.Buffer { buf.Reset(); return buf })
	bufpool.SetConstructor(func() *bytes.Buffer { return bytes.NewBuffer(_buf.Make()) })
	bufpool.FinalizeSetup()
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
//
// The type of the pooled object is stw.Slice[byte], a simple type
// alias for Go's slice type with convenience methods for common slice
// operations. You can use these values interchangeably with vanilla
// byte slices, as you need and wish.
func MakeBufferPool(minVal, maxVal int) *Pool[stw.Slice[byte]] {
	minVal, maxVal = intish.Bounds(minVal, maxVal)
	erc.InvariantOk(maxVal > 0, "buffer pool capacity max cannot be zero", ers.ErrInvalidInput)
	bufpool := &Pool[stw.Slice[byte]]{}
	bufpool.SetCleanupHook(func(buf stw.Slice[byte]) stw.Slice[byte] {
		if cap(buf) > maxVal {
			return nil
		}
		return buf[:0]
	})
	bufpool.SetConstructor(func() stw.Slice[byte] { return stw.NewSlice(make([]byte, 0, minVal)) })
	bufpool.FinalizeSetup()

	return bufpool
}

// DefaultBufferPool creates a pool of byte slices with a maximum size
// of 64kb. All other slices are discarded. These are the same
// settings as used by the fmt package's buffer pool.
//
// The type of the pooled object is stw.Slice[byte], a simple type
// alias for Go's slice type with convenience methods for common slice
// operations. You can use these values interchangeably with vanilla
// byte slices, as you need and wish.
func DefaultBufferPool() *Pool[stw.Slice[byte]] { return MakeBufferPool(0, 64*1024) }
