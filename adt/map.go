package adt

import (
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun/dt"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// OrderedMap is an alias of the `dt` package's ordered map
// implementation, which is (incidentally as an implementation detail)
// safe for concurrent use.
type OrderedMap[K comparable, V any] = dt.OrderedMap[K, V]

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
func (mp *Map[K, V]) Len() (count int) {
	mp.mp.Range(func(any, any) bool { count++; return true })
	return
}

// Check returns true if the key exists in the map or false otherwise.
func (mp *Map[K, V]) Check(key K) bool { return ft.IsOk(mp.mp.Load(key)) }

// Delete removes a key--and its corresponding value--from the map, if
// it exists.
func (mp *Map[K, V]) Delete(key K) { mp.mp.Delete(key) }

// Store adds a key and value to the map, replacing any existing
// values as needed.
func (mp *Map[K, V]) Store(k K, v V) { mp.mp.Store(k, v) }

// Load retrieves the value from the map. The semantics are the same
// as for maps in go: if the value does not exist it always returns
// the zero value for the type, while the second value indicates if
// the key was present in the map.
func (mp *Map[K, V]) Load(key K) (V, bool) { return mp.safeCast(mp.mp.Load(key)) }

// Get returns the value from the map. If the key is not present in the map,
// this returns the zero value for V.
func (mp *Map[K, V]) Get(key K) V { return ft.IgnoreSecond(mp.Load(key)) }

// Set adds the value to the map, overriding any existing value. The return reports if the key
// existed in the map before the operation.
func (mp *Map[K, V]) Set(key K, value V) bool { return ft.IgnoreFirst(mp.mp.Swap(key, value)) }

func (mp *Map[K, V]) safeCast(v any, ok bool) (out V, _ bool) {
	if v == nil {
		return out, false
	}
	return v.(V), ok
}

// Ensure is similar to EnsureStore but provides
// the default value as a function that produces a value rather than
// the value directly. The returned value is *always* the value of the
// key, which is either the value from the map or the value produced
// by the function.
//
// The constructor function is *always* called, even when the key
// exists in the map. Unlike Get and Ensure which have similar
// semantics and roles, the value produced by function does not
// participate in the default object pool.
func (mp *Map[K, V]) Ensure(key K) bool {
	defaultValue := mp.Default.Get()
	_, loaded := mp.mp.LoadOrStore(key, defaultValue)
	if !loaded {
		mp.Default.Put(defaultValue)
	}
	return loaded
}

// Extend adds values from the input sequence to the current map.  If
// keys in the input sequence exist in the map already, they're
// overridden. There is no isolation. While the operation is entirely
// thread safe (or at least as safe as the input iterator is), adding
// individual key/value pairs to the map may interleave with other
// operations.
func (mp *Map[K, V]) Extend(seq iter.Seq2[K, V]) { irt.Apply2(seq, mp.Store) }

// MarshalJSON produces a JSON form of the map, using a Range function
// to iterate through the values in the map. Range functions do not
// reflect a specific snapshot of the map if the map is being modified
// while being marshaled: keys will only appear at most once but order
// or which version of a value is not defined.
func (mp *Map[K, V]) MarshalJSON() ([]byte, error) { return json.Marshal(irt.Collect2(mp.Iterator())) }

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

// Iterator returns a native go iterator for a fun/dt.Map object.
func (mp *Map[K, V]) Iterator() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) { mp.mp.Range(func(ak, av any) bool { return yield(ak.(K), av.(V)) }) }
}

// Keys returns a stream that renders all of the keys in the map.
//
// This operation relies on a the underlying Range stream, and
// advances lazily through the Range operation as callers advance the
// stream. Be aware that this produces a stream that does not reflect
// any particular atomic of the underlying map.
func (mp *Map[K, V]) Keys() iter.Seq[K] { return irt.First(mp.Iterator()) }

// Values returns a stream that renders all of the values in the
// map.
//
// This operation relies on a the underlying Range iterator, and
// advances lazily through the Range operation as callers advance the
// stream. Be aware that this produces a stream that does not
// reflect any particular atomic of the underlying map.
func (mp *Map[K, V]) Values() iter.Seq[V] { return irt.Second(mp.Iterator()) }
