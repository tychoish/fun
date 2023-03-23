package adt

import (
	"runtime"
	"sync"

	"github.com/tychoish/fun"
)

type Pool[T any] struct {
	Constructor fun.Atomic[func() T]
	hook        *fun.Atomic[func(T) T]
	pool        *sync.Pool
}

func (p *Pool[T]) init() {
	if p.pool == nil {
		p.pool = &sync.Pool{
			New: func() any {
				return p.Constructor.Get()()
			},
		}
	}
	if p.hook == nil {
		p.hook = fun.NewAtomic(func(in T) T { return in })
	}
}

func (p *Pool[T]) SetCleanupHook(in func(T) T) { p.init(); p.hook.Set(in) }
func (p *Pool[T]) Get() T                      { p.init(); return p.pool.Get().(T) }
func (p *Pool[T]) Put(in T)                    { p.init(); p.pool.Put(p.hook.Get()(in)) }

func (p *Pool[T]) MagicGet() T {
	p.init()
	out := p.pool.Get().(T)
	runtime.SetFinalizer(out, p.Put)
	return out
}
