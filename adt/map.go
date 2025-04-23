package adt

import (
	"context"
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
)

// Map provides a wrapper around the standard library's sync.Map type
// with key/value types enforced by generics. Additional helpers
// support adding multiple items to the map, while Stream and
// StoreFrom provide compatibility with streams.
type Map[K comparable, V any] struct {
	// Default handles construction and pools objects in
	// the map for the Ensure and Get operations which must
	// construct zero-value items. No configuration or
	// construction is necessary; however, callers can modify the
	// default value constructed as needed.
	Default Pool[V]
	mp      sync.Map
}

// Delete removes a key--and its corresponding value--from the map, if
// it exists.
func (mp *Map[K, V]) Delete(key K) { mp.mp.Delete(key) }

// Store adds a key and value to the map, replacing any existing
// values as needed.
func (mp *Map[K, V]) Store(k K, v V) { mp.mp.Store(k, v) }

// Set adds a key and value to the map from a Pair.
func (mp *Map[K, V]) Set(it dt.Pair[K, V]) { mp.Store(it.Key, it.Value) }

// Ensure adds a key to the map if it does not already exist, using
// the default value. The default value, is taken from the pool, which
// has a configurable constructor if you want a different default
// value.
func (mp *Map[K, V]) Ensure(key K) { mp.EnsureDefault(key, mp.Default.Make) }

// Check returns true if the key exists in the map or false otherwise.
func (mp *Map[K, V]) Check(key K) bool { return ft.IsOk(mp.mp.Load(key)) }

// Load retrieves the value from the map. The semantics are the same
// as for maps in go: if the value does not exist it always returns
// the zero value for the type, while the second value indicates if
// the key was present in the map.
func (mp *Map[K, V]) Load(key K) (V, bool) { return mp.safeCast(mp.mp.Load(key)) }

// Ensure store takes a value and returns true if the value was stored
// in the map.
func (mp *Map[K, V]) EnsureStore(k K, v V) bool { _, loaded := mp.mp.LoadOrStore(k, v); return !loaded }

// EnsureSet has the same semantics as EnsureStore, but takes a dt.Pair
// object.
func (mp *Map[K, V]) EnsureSet(i dt.Pair[K, V]) bool { return mp.EnsureStore(i.Key, i.Value) }

func (mp *Map[K, V]) safeCast(v any, ok bool) (out V, _ bool) {
	if v == nil {
		return out, false
	}
	return v.(V), ok
}

// Get retrieves the value from the map at the given value. If the key
// is not present in the map a default value is created and added to
// the map.
func (mp *Map[K, V]) Get(key K) V {
	defaultValue := mp.Default.Get()
	out, loaded := mp.mp.LoadOrStore(key, defaultValue)
	if !loaded {
		mp.Default.Put(defaultValue)
	}
	return out.(V)
}

// EnsureDefault is similar to EnsureStore and Ensure, but provides
// the default value as a function that produces a value rather than
// the value directly. The returned value is *always* the value of the
// key, which is either the value from the map or the value produced
// by the function.
//
// The constructor function is *always* called, even when the key
// exists in the map. Unlike Get and Ensure which have similar
// semantics and roles, the value produced by function does not
// participate in the default object pool.
func (mp *Map[K, V]) EnsureDefault(key K, constr func() V) V {
	out, _ := mp.mp.LoadOrStore(key, constr())
	return out.(V)
}

// MarshalJSON produces a JSON form of the map, using a Range function
// to iterate through the values in the map. Range functions do not
// reflect a specific snapshot of the map if the map is being modified
// while being marshaled: keys will only appear at most once but order
// or which version of a value is not defined.
func (mp *Map[K, V]) MarshalJSON() ([]byte, error) {
	out := map[K]V{}
	mp.Range(func(k K, v V) bool { out[k] = v; return true })
	return json.Marshal(out)
}

// UnmarshalJSON takes a json sequence and adds the values to the
// map. This does not remove or reset the values in the map, and other
// operations may interleave during this operation.
func (mp *Map[K, V]) UnmarshalJSON(in []byte) error {
	out := map[K]V{}
	if err := json.Unmarshal(in, &out); err != nil {
		return err
	}

	for k := range out {
		mp.Store(k, out[k])
	}

	return nil
}

// Len counts and reports on the number of items in the map. This is
// provided by iterating and counting the values in the map, and has
// O(n) performance.
//
// Len uses a range function and therefore does not reflect a specific
// snapshot of the map at any time if keys are being deleted while Len
// is running. Len will never report a number that is larger than the
// total number of items in the map while Len is running, but the
// number of items in the map may be smaller at the beginning and/or
// the end than reported.
func (mp *Map[K, V]) Len() int {
	count := 0
	mp.mp.Range(func(any, any) bool { count++; return true })

	return count
}

// Range provides a method for iterating over the values in the map,
// with a similar API as the standard library's sync.Map. The function
// is called once on every key in the map. When the range function
// returns false the iteration stops.
//
// Range functions do not reflect a specific snapshot of the map if
// the map is being modified while being marshaled: keys will only
// appear at most once but order or which version of a value is not
// defined.
func (mp *Map[K, V]) Range(fn func(K, V) bool) { mp.Iterator()(fn) }

// Stream returns a stream that produces a sequence of pair
// objects.
//
// This operation relies on a the underlying Range stream, and
// advances lazily through the Range operation as callers advance the
// stream. Be aware that this produces a stream that does not
// reflect any particular atomic of the underlying map.
func (mp *Map[K, V]) Stream() *fun.Stream[dt.Pair[K, V]] { return makeMapStream(mp, dt.MakePair) }

// Iterator returns a native go iterator for a fun/dt.Map object.
func (mp *Map[K, V]) Iterator() iter.Seq2[K, V] {
	return func(fn func(K, V) bool) {
		mp.mp.Range(func(ak, av any) bool { return fn(ak.(K), av.(V)) })
	}
}

// Keys returns a stream that renders all of the keys in the map.
//
// This operation relies on a the underlying Range stream, and
// advances lazily through the Range operation as callers advance the
// stream. Be aware that this produces a stream that does not reflect
// any particular atomic of the underlying map.
func (mp *Map[K, V]) Keys() *fun.Stream[K] { return makeMapStream(mp, func(k K, _ V) K { return k }) }

// Values returns a stream that renders all of the values in the
// map.
//
// This operation relies on a the underlying Range iterator, and
// advances lazily through the Range operation as callers advance the
// stream. Be aware that this produces a stream that does not
// reflect any particular atomic of the underlying map.
func (mp *Map[K, V]) Values() *fun.Stream[V] { return makeMapStream(mp, func(_ K, v V) V { return v }) }

func makeMapStream[K comparable, V any, O any](
	mp *Map[K, V],
	rf func(K, V) O,
) *fun.Stream[O] {
	pipe := fun.Blocking(make(chan O))
	return pipe.Generator().PreHook(fun.Operation(func(ctx context.Context) {
		send := pipe.Send()
		mp.Range(func(key K, value V) bool {
			return send.Check(ctx, rf(key, value))
		})
	}).PostHook(pipe.Close).Go().Once()).Stream()
}
