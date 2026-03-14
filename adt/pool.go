package adt

import (
	"bytes"
	"sync"
	"sync/atomic"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/stw"
)

// Pool is an ergonomic wrapper around sync.Pool that provides a
// generic type interface and an optional cleanup hook to modify
// objects before returning them to the pool.
//
// Pool can be used with default construction; however, you should set
// the Constructor before calling Get() on the pool.
type Pool[T any] struct {
	once        sync.Once
	locked      atomic.Bool
	hook        *Atomic[func(T) T]
	constructor *Atomic[func() T]
	pool        *sync.Pool
}

func (p *Pool[T]) init() { p.once.Do(p.doInit) }

func (*Pool[T]) zero() (zero T) { return }
func (*Pool[T]) noop(in T) T    { return in }
func (p *Pool[T]) doInit() {
	p.hook = NewAtomic(p.noop)
	p.constructor = NewAtomic(p.zero)
	p.pool = &sync.Pool{New: func() any { return p.constructor.Get()() }}
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
	erc.InvariantOk(!p.locked.Load(), "SetCleaupHook", "after FinalizeSetup", ers.ErrImmutabilityViolation)
	if in != nil {
		p.hook.Set(in)
	}
}

// SetConstructor overrides the default constructor (which returns a
// zero value by default) for Get operations.
func (p *Pool[T]) SetConstructor(in func() T) {
	p.init()
	erc.InvariantOk(!p.locked.Load(), "SetConstructor", "after FinalizeSetup", ers.ErrImmutabilityViolation)
	if in != nil {
		p.constructor.Set(in)
	}
}

// Get returns an object from the pool or constructs a default object
// according to the constructor.
func (p *Pool[T]) Get() T { p.init(); return p.pool.Get().(T) }

// Put returns an object in the pool, calling the cleanup hook if set.
func (p *Pool[T]) Put(in T) {
	p.init()

	if any(in) != nil {
		p.pool.Put(p.hook.Get()(in))
	}
}

// MakeBytesBufferPool configures a pool of *bytes.Buffers. Buffers
// are always reset before reentering the pool, and are pre-allocated
// with the specified capacity. Negative initial capcity values are
// ignored.
func MakeBytesBufferPool(capacity int) *Pool[*bytes.Buffer] {
	_buf := MakeBufferPool(capacity, 64*1024)
	bufpool := &Pool[*bytes.Buffer]{}
	bufpool.SetCleanupHook(func(buf *bytes.Buffer) *bytes.Buffer { buf.Reset(); return buf })
	bufpool.SetConstructor(func() *bytes.Buffer { return bytes.NewBuffer(_buf.Get()) })
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
	minVal, maxVal = stw.Bounds(minVal, maxVal)
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
