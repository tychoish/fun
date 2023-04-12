package adt

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

func NewMapItem[K comparable, V any](k K, v V) MapItem[K, V] { return MapItem[K, V]{Key: k, Value: v} }

type MapItem[K comparable, V any] struct {
	Key   K
	Value V
}

// Map provides a wrapper around the standard library's sync.Map type
// with key/value types enforced by generics. Additional helpers
// (Append, Extend, Populate) support adding multiple items to the
// map, while Iterator and StoreFrom provide compatibility with
// iterators.
type Map[K comparable, V any] struct {
	// Default handles construction and pools objects in
	// the map for the Ensure and Get operations which must
	// construct zero-value items. No configuration or
	// construction is necessary; however, callers can modify the
	// default value constructed as needed.
	Default Pool[V]
	mp      sync.Map
}

func (mp *Map[K, V]) Delete(key K)                   { mp.mp.Delete(key) }
func (mp *Map[K, V]) Store(k K, v V)                 { mp.mp.Store(k, v) }
func (mp *Map[K, V]) Set(it MapItem[K, V])           { mp.Store(it.Key, it.Value) }
func (mp *Map[K, V]) Append(its ...MapItem[K, V])    { mp.Extend(its) }
func (mp *Map[K, V]) Ensure(key K)                   { mp.EnsureDefault(key, mp.Default.Make) }
func (mp *Map[K, V]) Contains(key K) bool            { _, ok := mp.mp.Load(key); return ok }
func (mp *Map[K, V]) Load(key K) (V, bool)           { return mp.safeCast(mp.mp.Load(key)) }
func (mp *Map[K, V]) EnsureStore(k K, v V) bool      { _, loaded := mp.mp.LoadOrStore(k, v); return !loaded }
func (mp *Map[K, V]) EnsureSet(i MapItem[K, V]) bool { return mp.EnsureStore(i.Key, i.Value) }

func (mp *Map[K, V]) safeCast(v any, ok bool) (V, bool) {
	if v == nil {
		return fun.ZeroOf[V](), false
	}
	return v.(V), ok
}

func (mp *Map[K, V]) Get(key K) V {
	new := mp.Default.Get()
	out, loaded := mp.mp.LoadOrStore(key, new)
	if !loaded {
		mp.Default.Put(new)
	}
	return out.(V)
}

func (mp *Map[K, V]) EnsureDefault(key K, constr func() V) V {
	out, _ := mp.mp.LoadOrStore(key, constr())
	return out.(V)
}

func (mp *Map[K, V]) Join(in *Map[K, V]) {
	in.Range(func(k K, v V) bool { mp.Store(k, v); return true })
}

func (mp *Map[K, V]) Populate(in map[K]V) {
	for k := range in {
		mp.Store(k, in[k])
	}
}

func (mp *Map[K, V]) Export() map[K]V {
	out := map[K]V{}
	mp.Range(func(k K, v V) bool { out[k] = v; return true })
	return out
}

func (mp *Map[K, V]) MarshalJSON() ([]byte, error) { return json.Marshal(mp.Export()) }

func (mp *Map[K, V]) UnmarshalJSON(in []byte) error {
	out := map[K]V{}
	if err := json.Unmarshal(in, &out); err != nil {
		return err
	}
	mp.Populate(out)
	return nil
}

func (mp *Map[K, V]) Extend(its []MapItem[K, V]) {
	for _, it := range its {
		mp.Store(it.Key, it.Value)
	}
}

func (mp *Map[K, V]) StoreFrom(ctx context.Context, iter fun.Iterator[MapItem[K, V]]) {
	fun.InvariantMust(fun.Observe(ctx, iter, func(it MapItem[K, V]) { mp.Store(it.Key, it.Value) }))
}

func (mp *Map[K, V]) Len() int {
	count := 0
	mp.Range(func(K, V) bool { count++; return true })

	return count
}

func (mp *Map[K, V]) Range(f func(K, V) bool) {
	mp.mp.Range(func(ak, av any) bool { return f(ak.(K), av.(V)) })
}

func (mp *Map[K, V]) Iterator() fun.Iterator[MapItem[K, V]] {
	iter := &internal.ChannelIterImpl[MapItem[K, V]]{}
	pipe := make(chan MapItem[K, V])
	iter.Pipe = pipe
	ctx, cancel := context.WithCancel(internal.BackgroundContext)
	iter.Closer = cancel
	iter.WG.Add(1)
	go func() {
		defer iter.WG.Done()
		defer close(pipe)
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

func (mp *Map[K, V]) Keys() fun.Iterator[K] {
	return fun.Transform(mp.Iterator(), func(in MapItem[K, V]) (K, error) { return in.Key, nil })
}

func (mp *Map[K, V]) Values() fun.Iterator[V] {
	return fun.Transform(mp.Iterator(), func(in MapItem[K, V]) (V, error) { return in.Value, nil })
}
