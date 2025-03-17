package dt

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
)

// ErrUninitializedContainer is the content of the panic produced when you
// attempt to perform an operation on an uninitialized sequence.
const ErrUninitializedContainer ers.Error = ers.Error("uninitialized container")

// Map is just a generic type wrapper around a map, mostly for the
// purpose of being able to interact with Pair[K,V] objects and
// Iterators.
//
// All normal map operations are still accessible, these methods
// exist to provide accessible function objects for use in contexts
// where that may be useful and to improve the readability of some
// call sites, where default map access may be awkward.
type Map[K comparable, V any] map[K]V

// DefaultMap takes a map value and returns it if it's non-nil. If the
// map is nil, it constructs and returns a new map, with the
// (optionally specified length.
func DefaultMap[K comparable, V any](input map[K]V, args ...int) map[K]V {
	if input != nil {
		return input
	}
	switch len(args) {
	case 0:
		return map[K]V{}
	case 1:
		return make(map[K]V, args[0])
	default:
		panic(ers.Wrap(ers.ErrInvariantViolation, "cannot specify >2 arguments to make() for a map"))
	}
}

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
	return NewMap(in).Iterator()
}

// MapKeys takes an arbitrary map and produces an iterator over only
// the keys.
func MapKeys[K comparable, V any](in map[K]V) *fun.Iterator[K] { return NewMap(in).Keys() }

// MapValues takes an arbitrary map and produces an iterator over only
// the values.
func MapValues[K comparable, V any](in map[K]V) *fun.Iterator[V] { return NewMap(in).Values() }

// NewMap provides a constructor that will produce a fun.Map without
// specifying types.
func NewMap[K comparable, V any](in map[K]V) Map[K, V] { return in }

// NewMapFromIterator constructs an map from a fun.Iterator of pairs.
func NewMapFromIterator[K comparable, V any](ctx context.Context, it *fun.Iterator[Pair[K, V]]) Map[K, V] {
	out := Map[K, V]{}
	it.Observe(out.AddPair).Ignore().Run(ctx)
	return out
}

// Mapify provides a constructor that will produce a fun.Map without
// specifying types.
//
// Deprecated: use NewMap() for this case.
func Mapify[K comparable, V any](in map[K]V) Map[K, V] { return NewMap(in) }

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

// Pairs exports a Pairs object, containing the contents of the map.
func (m Map[K, V]) Pairs() *Pairs[K, V] {
	p := MakePairs[K, V]()
	for k, v := range m {
		p.Add(k, v)
	}
	return p
}

// Tuples exports a map a Pairs object, containing the contents of the map.
func (m Map[K, V]) Tuples() *Tuples[K, V] {
	tp := MakeTuples[K, V]()
	for k, v := range m {
		tp.Add(k, v)
	}
	return tp
}

// Add adds a key value pair directly to the map.
func (m Map[K, V]) Add(k K, v V)   { m[k] = v }
func (m Map[K, V]) Store(k K, v V) { m.Add(k, v) }

// Delete removes a key from the map.
func (m Map[K, V]) Delete(k K) { delete(m, k) }

// AddPair adds a Pair holding K and V objects to the map.
func (m Map[K, V]) AddPair(p Pair[K, V]) { m.Add(p.Key, p.Value) }

// AddTuple adds a Tuple of K and V objects to the map.
func (m Map[K, V]) AddTuple(p Tuple[K, V]) { m.Add(p.One, p.Two) }

// Append adds a sequence of Pair objects to the map.
func (m Map[K, V]) Append(pairs ...Pair[K, V]) { NewSlice(pairs).Observe(m.AddPair) }

// Len returns the length. It is equivalent to len(Map), but is
// provided for consistency.
func (m Map[K, V]) Len() int { return len(m) }

// Extend adds a sequence of Pairs to the map.
func (m Map[K, V]) Extend(pairs *Pairs[K, V]) {
	fun.Invariant.Must(pairs.Iterator().Observe(m.AddPair).Wait())
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

// ConsumePairs adds items to the map from a Pairs object. Existing
// values for K are always overwritten.
func (m Map[K, V]) ConsumePairs(pairs *Pairs[K, V]) {
	pairs.Iterator().Observe(m.AddPair).Ignore().Wait()
}

// ConsumeTuples adds items to the map from a Tuples object. Existing
// values for K are always overwritten.
func (m Map[K, V]) ConsumeTuples(tuples *Tuples[K, V]) {
	tuples.Iterator().Observe(m.AddTuple).Ignore().Wait()
}

// ConsumeValues adds items to the map, using the function to generate
// the keys for the values.
//
// This operation will panic (with an ErrInvariantValidation) if the
// keyf panics.
func (m Map[K, V]) ConsumeValues(iter *fun.Iterator[V], keyf func(V) K) fun.Worker {
	return iter.Observe(func(in V) { m[keyf(in)] = in })
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

// Producer constructs a fun.Producer function for the pairs in the
// map. The operation starts a goroutine on the first iteration that
// tracks the state of the iterator. Iteration order is randomized.
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
	}).Go().Once()

	return pipe.Receive().Producer().PreHook(init)
}

// ProducerKeys returns a generator that generates the keys of the
// map. The operation requires a goroutine to keep track of the state
// of the iteration, but does not buffer or cache keys.
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
	}).Go().Once()

	return pipe.Receive().Producer().PreHook(init)
}

// ProducerValues returns a generator that generates the values of the
// map. The operation requires a goroutine to keep track of the state
// of the iteration, but does not buffer or cache values.
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
	}).Go().Once()

	return pipe.Receive().Producer().PreHook(init)
}
