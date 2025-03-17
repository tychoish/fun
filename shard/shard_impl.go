package shard

import (
	"sync/atomic"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ers"
)

type sh[K comparable, V any] struct {
	data  vmap[K, V]
	clock atomic.Uint64
}

func (*sh[K, V]) makeVmap(impl MapImplementation) vmap[K, V] {
	switch impl {
	case MapImplementationSyncMap:
		return &adt.Map[K, *Versioned[V]]{}
	case MapImplementationStdlib:
		return dt.Map[K, *Versioned[V]]{}
	case MapImplementationRWMutex:
		return &mutexMap[K, *Versioned[V]]{}
	case MapImplementationMutex:
		return &mutexMap[K, *Versioned[V]]{}
	default:
		panic(ers.ErrInvalidInput)
	}

}

func (sh *sh[K, V]) read() vmap[K, V]                     { return sh.data }
func (sh *sh[K, V]) write() vmap[K, V]                    { sh.clock.Add(1); return sh.data }
func (sh *sh[K, V]) load(k K) (V, bool)                   { v, ok := sh.read().Load(k); return v.Load(), ok }
func (sh *sh[K, V]) keys() *fun.Iterator[K]               { return sh.read().Keys() }
func (sh *sh[K, V]) vvIter() *fun.Iterator[*Versioned[V]] { return sh.read().Values() }
func (sh *sh[K, V]) values() *fun.Iterator[V]             { return to(sh.unwrap).Process(sh.vvIter()) }
func (*sh[K, V]) unwrap(vv *Versioned[V]) V               { return vv.Load() }

func (sh *sh[K, V]) valuesp(n uint64) *fun.Iterator[V] {
	return to(sh.unwrap).ProcessParallel(sh.vvIter(), numWorkers(int(n)))
}

func (sh *sh[K, V]) store(k K, v V) {
	mp := sh.write()
	if val, ok := mp.Load(k); ok {
		val.Set(v)
		mp.Store(k, val)
	} else {
		mp.Store(k, Version(v))
	}
}

func (sh *sh[K, V]) fetch(k K) (out V, _uint64, _ uint64, _ bool) {
	mp := sh.read()
	for {
		shVersion := sh.clock.Load()
		d, ok := mp.Load(k)
		ov := d.Version()
		switch {
		case !ok:
			return out, 0, 0, false
		case shVersion == sh.clock.Load():
			value, version := d.Fetch()
			if ov == version {
				return value, version, shVersion, ok
			}

		}
	}
}
