// Package shard provides a logically versioned map implementation backed by a collection of
// independently synchronized maps.
//
// In the current iteration the parallel methods/helpers are not implemented with (safe?) parallelism.
package shard

import (
	"fmt"
	"iter"
	"sync/atomic"

	"github.com/tychoish/fun/adt"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/stw"
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
		return stw.Map[K, *Versioned[V]]{}
	case MapTypeRWMutex:
		return &rmtxMap[K, *Versioned[V]]{d: stw.Map[K, *Versioned[V]]{}}
	case MapTypeMutex:
		return &mtxMap[K, *Versioned[V]]{d: stw.Map[K, *Versioned[V]]{}}
	default:
		panic(erc.Join(ers.ErrInvalidInput, fmt.Errorf("map<%d> is does not exist", impl)))
	}
}

func (*sh[K, V]) inner(vv *Versioned[V]) V          { return vv.Load() }
func (sh *sh[K, V]) read() vmap[K, V]               { return sh.data }
func (sh *sh[K, V]) write() vmap[K, V]              { sh.clock.Add(1); return sh.data }
func (sh *sh[K, V]) store(k K, v V)                 { sh.set(k, v) }
func (sh *sh[K, V]) load(k K) (V, bool)             { v, ok := sh.read().Load(k); return v.Load(), ok }
func (sh *sh[K, V]) keys() iter.Seq[K]              { return sh.read().Keys() }
func (sh *sh[K, V]) vvals() iter.Seq[*Versioned[V]] { return sh.read().Values() }
func (sh *sh[K, V]) values() iter.Seq[V]            { return to(sh.inner).Iterator(sh.vvals()) }

func (sh *sh[K, V]) set(k K, v V) bool {
	mp := sh.write()
	if val, ok := mp.Load(k); ok {
		val.Set(v)
		return mp.Set(k, val)
	}
	return mp.Set(k, Version(v))
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
