package dt

import (
	"iter"
	"maps"

	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/fnx"
)

// ErrUninitializedContainer is the content of the panic produced when you
// attempt to perform an operation on an uninitialized sequence.
const ErrUninitializedContainer ers.Error = ers.Error("uninitialized container")

// Map is just a generic type wrapper around a map, mostly for the
// purpose of being able to interact with Pair[K,V] objects and
// Streams.
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

// NewMap provides a constructor that will produce a fun.Map without
// specifying types.
func NewMap[K comparable, V any](in map[K]V) Map[K, V] { return in }

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

// Add adds a key value pair directly to the map.
func (m Map[K, V]) Add(k K, v V) { m[k] = v }

// Store adds a key value pair directly to the map.
func (m Map[K, V]) Store(k K, v V) { m.Add(k, v) }

// Delete removes a key from the map.
func (m Map[K, V]) Delete(k K) { delete(m, k) }

// AddPair adds a Pair holding K and V objects to the map.
func (m Map[K, V]) AddPair(p Pair[K, V]) { m.Store(p.Key, p.Value) }

// AddTuple adds a Tuple of K and V objects to the map.
func (m Map[K, V]) AddTuple(p Tuple[K, V]) { m.Add(p.One, p.Two) }

// Append adds a sequence of Pair objects to the map.
func (m Map[K, V]) Append(pairs ...Pair[K, V]) { NewSlice(pairs).ReadAll(m.AddPair) }

// Len returns the length. It is equivalent to len(Map), but is
// provided for consistency.
func (m Map[K, V]) Len() int { return len(m) }

// Extend adds a sequence of Pairs to the map.
func (m Map[K, V]) Extend(pairs *Pairs[K, V]) {
	pairs.Stream().ReadAll(fnx.FromHandler(m.AddPair)).Ignore().Wait()
}

// ExtendWithPairs adds items to the map from a Pairs object. Existing
// values for K are always overwritten.
func (m Map[K, V]) ExtendWithPairs(pairs *Pairs[K, V]) {
	pairs.Stream().ReadAll(fnx.FromHandler(m.AddPair)).Ignore().Wait()
}

// ExtendWithTuples adds items to the map from a Tuples object. Existing
// values for K are always overwritten.
func (m Map[K, V]) ExtendWithTuples(tuples *Tuples[K, V]) {
	tuples.Stream().ReadAll(fnx.FromHandler(m.AddTuple)).Ignore().Wait()
}

// Stream converts a map into a stream of dt.Pair objects. The
// stream is panic-safe, and uses one go routine to track the
// progress through the map. As a result you should always, either
// exhaust the stream, cancel the context that you pass to the
// stream OR call stream.Close().
//
// To use this stream the items in the map are not copied, and the
// iteration order is randomized following the convention in go.
//
// Use in combination with other stream processing tools
// (futures, observers, transformers, etc.) to limit the number of
// times a collection of data must be coppied.
func (m Map[K, V]) Iterator() iter.Seq2[K, V] { return maps.All(m) }

// Keys provides a stream over just the keys in the map.
func (m Map[K, V]) Keys() iter.Seq[K] { return maps.Keys(m) }

// Values provides a stream over just the values in the map.
func (m Map[K, V]) Values() iter.Seq[V] { return maps.Values(m) }
