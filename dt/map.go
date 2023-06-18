package dt

import (
	"context"

	"github.com/tychoish/fun"
)

// Map is just a generic type wrapper around a map, mostly for the
// purpose of being able to interact with Pair[K,V] objects and
// Iterators.
//
// All normal map operations are still accessible, these methods
// exist to provide accessible function objects for use in contexts
// where that may be useful and to improve the readability of some
// call sites, where default map access may be awkward.
type Map[K comparable, V any] map[K]V

// MapIterator converts a map into an iterator of dt.Pair objects. The
// iterator is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the iterator, cancel the context that you pass to the
// iterator OR call iterator.Close().
//
// To use this iterator the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other iterator processing tools
// (generators, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func MapIterator[K comparable, V any](in map[K]V) *fun.Iterator[Pair[K, V]] {
	return Mapify(in).Iterator()
}
func MapKeys[K comparable, V any](in map[K]V) *fun.Iterator[K]   { return Mapify(in).Keys() }
func MapValues[K comparable, V any](in map[K]V) *fun.Iterator[V] { return Mapify(in).Values() }

// Mapify provides a constructor that will produce a fun.Map without
// specifying types.
func Mapify[K comparable, V any](in map[K]V) Map[K, V] { return in }

// Check returns true if the value K is in the map.
func (m Map[K, V]) Check(key K) bool { _, ok := m[key]; return ok }

// Get returns the value from the map, and is the same thing as:
//
//	foo := mp[key]
//
// If the key is not present in the map, as with a normal map, this is
// the zero value for V.
func (m Map[K, V]) Get(key K) V { return m[key] }

// Load returns the value in the map for the key, and an "ok" value
// which is true if that item is present in the map.
func (m Map[K, V]) Load(key K) (V, bool) { v, ok := m[key]; return v, ok }

// SetDefault set's sets the provided key in the map to the zero value
// for the value type.
func (m Map[K, V]) SetDefault(key K) { var vl V; m[key] = vl }

// Pairs exports a map a Pairs object, which is an alias for a slice of
// Pair objects.
func (m Map[K, V]) Pairs() Pairs[K, V] { p := MakePairs[K, V](); p.ConsumeMap(m); return p }

// Add adds a key value pair directly to the map.
func (m Map[K, V]) Add(k K, v V) { m[k] = v }

// AddPair adds a Pair object to the map.
func (m Map[K, V]) AddPair(p Pair[K, V]) { m.Add(p.Key, p.Value) }

// Append adds a sequence of Pair objects to the map.
func (m Map[K, V]) Append(pairs ...Pair[K, V]) { m.Extend(pairs) }

// Len returns the length. It is equivalent to len(Map), but is
// provided for consistency.
func (m Map[K, V]) Len() int { return len(m) }

// Extend adds a sequence of Pairs to the map.
func (m Map[K, V]) Extend(pairs Pairs[K, V]) {
	for _, pair := range pairs {
		m.AddPair(pair)
	}
}

// ConsumeMap adds all the keys from the input map the map.
func (m Map[K, V]) ConsumeMap(in Map[K, V]) {
	for k, v := range in {
		m[k] = v
	}
}

// ConsumeSlice adds a slice of values to the map, using the provided
// function to generate the key for the value. Existing values in the
// map are overridden.
func (m Map[K, V]) ConsumeSlice(in []V, keyf func(V) K) {
	for idx := range in {
		value := in[idx]
		m[keyf(value)] = value
	}
}

// Consume adds items to the map from an iterator of Pair
// objects. Existing values for K are always overwritten.
func (m Map[K, V]) Consume(ctx context.Context, iter *fun.Iterator[Pair[K, V]]) {
	fun.InvariantMust(iter.Observe(ctx, func(in Pair[K, V]) { m.AddPair(in) }))
}

// ConsumeValues adds items to the map, using the function to generate
// the keys for the values.
//
// This operation will panic (with an ErrInvariantValidation) if the
// keyf panics.
func (m Map[K, V]) ConsumeValues(ctx context.Context, iter *fun.Iterator[V], keyf func(V) K) {
	fun.InvariantMust(iter.Observe(ctx, func(in V) { m[keyf(in)] = in }))
}

// Iterator converts a map into an iterator of dt.Pair objects. The
// iterator is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the iterator, cancel the context that you pass to the
// iterator OR call iterator.Close().
//
// To use this iterator the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other iterator processing tools
// (generators, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func (m Map[K, V]) Iterator() *fun.Iterator[Pair[K, V]] { return m.Producer().Iterator() }

// Keys provides an iterator over just the keys in the map.
func (m Map[K, V]) Keys() *fun.Iterator[K] { return m.ProducerKeys().Iterator() }

// Values provides an iterator over just the values in the map.
func (m Map[K, V]) Values() *fun.Iterator[V] { return m.ProducerValues().Iterator() }

func (m Map[K, V]) Producer() fun.Producer[Pair[K, V]] {
	pipe := fun.Blocking(make(chan Pair[K, V]))

	init := fun.Operation(func(ctx context.Context) {
		defer pipe.Close()
		send := pipe.Send()
		for k, v := range m {
			if !send.Check(ctx, MakePair(k, v)) {
				break
			}
		}
	}).Launch().Once()

	return pipe.Receive().Producer().PreHook(init)
}

func (m Map[K, V]) ProducerKeys() fun.Producer[K] {
	pipe := fun.Blocking(make(chan K))

	init := fun.Operation(func(ctx context.Context) {
		defer pipe.Close()
		send := pipe.Send()
		for k := range m {
			if !send.Check(ctx, k) {
				break
			}
		}
	}).Launch().Once()

	return pipe.Receive().Producer().PreHook(init)
}

func (m Map[K, V]) ProducerValues() fun.Producer[V] {
	pipe := fun.Blocking(make(chan V))

	init := fun.Operation(func(ctx context.Context) {
		defer pipe.Close()
		send := pipe.Send()
		for k := range m {
			if !send.Check(ctx, m[k]) {
				break
			}
		}
	}).Launch().Once()

	return pipe.Receive().Producer().PreHook(init)
}