package adt

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/internal"
)

type MapItem[K comparable, V any] struct {
	Key   K
	Value V
}

// SyncMap provides a wrapper around the standard library's sync.Map
// type with key/value types enforced by generics. Additional
// helpers (Append, Extend, Populate) support
type SyncMap[K comparable, V any] struct {
	MakeDefault Pool[V]
	mp          sync.Map
}

func MakeSyncMap[K comparable, V any](in map[K]V) *SyncMap[K, V] {
	mp := &SyncMap[K, V]{}
	mp.Populate(in)
	return mp
}

func (mp *SyncMap[K, V]) Delete(key K)                { mp.mp.Delete(key) }
func (mp *SyncMap[K, V]) Load(key K) (V, bool)        { v, ok := mp.mp.Load(key); return v.(V), ok }
func (mp *SyncMap[K, V]) Store(k K, v V)              { mp.mp.Store(k, v) }
func (mp *SyncMap[K, V]) Swap(k K, v V) (any, bool)   { p, ok := mp.mp.Swap(k, v); return p.(V), ok }
func (mp *SyncMap[K, V]) Set(it MapItem[K, V])        { mp.Store(it.Key, it.Value) }
func (mp *SyncMap[K, V]) Append(its ...MapItem[K, V]) { mp.Extend(its) }

func (mp *SyncMap[K, V]) LoadAndDelete(k K) (V, bool) {
	v, ok := mp.mp.LoadAndDelete(k)
	return v.(V), ok
}

func (mp *SyncMap[K, V]) LoadOrStore(key K, val V) (V, bool) {
	v, ok := mp.mp.LoadOrStore(key, val)
	return v.(V), ok
}

func (mp *SyncMap[K, V]) Get(key K) V {
	new := mp.MakeDefault.Get()
	out, loaded := mp.LoadOrStore(key, new)
	if !loaded {
		mp.MakeDefault.Put(new)
	}
	return out
}

func (mp *SyncMap[K, V]) Join(in *SyncMap[K, V]) {
	mp.Range(func(k K, v V) bool { mp.Store(k, v); return true })
}

func (mp *SyncMap[K, V]) Populate(in map[K]V) {
	for k := range in {
		mp.Store(k, in[k])
	}
}

func (mp *SyncMap[K, V]) Extend(its []MapItem[K, V]) {
	for _, it := range its {
		mp.Store(it.Key, it.Value)
	}
}

func (mp *SyncMap[K, V]) StoreFrom(ctx context.Context, iter fun.Iterator[MapItem[K, V]]) {
	fun.Observe(ctx, iter, mp.Set)
}

func (mp *SyncMap[K, V]) Range(f func(K, V) bool) {
	mp.mp.Range(func(ak, av any) bool { return f(ak.(K), av.(V)) })
}

func (mp *SyncMap[K, V]) Iterator() fun.Iterator[MapItem[K, V]] {
	iter := &internal.MapIterImpl[MapItem[K, V]]{}
	pipe := make(chan MapItem[K, V])
	ctx, cancel := context.WithCancel(internal.BackgroundContext)
	iter.Closer = cancel
	ec := &erc.Collector{}
	iter.WG.Add(1)
	go func() {
		defer iter.WG.Done()
		defer close(pipe)
		defer func() { iter.Error = ec.Resolve() }()
		defer erc.Recover(ec)
		mp.Range(func(key K, value V) bool {
			select {
			case <-ctx.Done():
				return false
			case pipe <- MapItem[K, V]{Key: key, Value: value}:
				return true
			}
		})
	}()
	return iter
}