package dt

import stdlibAtomic "sync/atomic"

type atomic[T comparable] struct{ val stdlibAtomic.Value }

func (a *atomic[T]) Get() T { return a.resolve(a.val.Load()) }
func (a *atomic[T]) Set(in T) bool {
	return a.val.CompareAndSwap(nil, in) || a.val.CompareAndSwap(in, in)
}

func (*atomic[T]) resolve(in any) (val T) {
	switch c := in.(type) {
	case T:
		val = c
	}
	return val
}
