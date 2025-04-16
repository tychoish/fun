package shard

import (
	"fmt"
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

func (*sh[K, V]) makeVmap(impl MapType) vmap[K, V] {
	switch impl {
	case MapTypeSync:
		return &adt.Map[K, *Versioned[V]]{}
	case MapTypeStdlib:
		return dt.Map[K, *Versioned[V]]{}
	case MapTypeRWMutex:
		return &rmtxMap[K, *Versioned[V]]{d: dt.Map[K, *Versioned[V]]{}}
	case MapTypeMutex:
		return &mtxMap[K, *Versioned[V]]{d: dt.Map[K, *Versioned[V]]{}}
	default:
		panic(ers.Join(ers.ErrInvalidInput, fmt.Errorf("map<%d> is does not exist", impl)))
	}
}

func (sh *sh[K, V]) read() vmap[K, V]                  { return sh.data }
func (sh *sh[K, V]) write() vmap[K, V]                 { sh.clock.Add(1); return sh.data }
func (sh *sh[K, V]) load(k K) (V, bool)                { v, ok := sh.read().Load(k); return v.Load(), ok }
func (sh *sh[K, V]) keys() *fun.Stream[K]              { return sh.read().Keys() }
func (sh *sh[K, V]) vvals() *fun.Stream[*Versioned[V]] { return sh.read().Values() }
func (sh *sh[K, V]) values() *fun.Stream[V]            { return to(sh.inner).ReadAll(sh.vvals()) }
func (*sh[K, V]) inner(vv *Versioned[V]) V             { return vv.Load() }
func (sh *sh[K, V]) valsp(n int) *fun.Stream[V] {
	return to(sh.inner).Parallel(sh.vvals(), poolOpts(n))
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
