package fun

import "encoding/json"

// Pair represents a key-value pair. Used by the adt synchronized map
// implementation and the set package to handle ordered key-value pairs.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// MakePair constructs a pair object. This is identical to using the
// literal constructor but may be more ergonomic.
func MakePair[K comparable, V any](k K, v V) Pair[K, V] { return Pair[K, V]{Key: k, Value: v} }

// Pairs implements a collection of key-value pairs.
type Pairs[K comparable, V any] []Pair[K, V]

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p Pairs[K, V]) MarshalJSON() ([]byte, error) { return json.Marshal(p.Map()) }

// UnmarshalJSON provides consistent JSON decoding for Pairs
// objects. It reads a JSON document into a map and converts it to
// pairs, and appends it to the existing Pairs objects without
// removing or resetting the current object.
func (p *Pairs[K, V]) UnmarshalJSON(in []byte) error {
	t := make(map[K]V)
	if err := json.Unmarshal(in, &t); err != nil {
		return err
	}
	tt := MakePairs(t)
	*p = p.Append(tt...)
	return nil
}

// MakePairs converts a map type into a slice of Pair types
// that can be usable in a set.
func MakePairs[K comparable, V any](in map[K]V) Pairs[K, V] {
	out := make([]Pair[K, V], 0, len(in))
	for k, v := range in {
		out = append(out, MakePair(k, v))
	}
	return out
}

// Add adds a new value to the underlying slice. This may add a
// duplicate key.
func (p *Pairs[K, V]) Add(k K, v V) { *p = p.Append(Pair[K, V]{Key: k, Value: v}) }

// Append, mirroring the semantics of the built in append() function
// adds one or more Pair items to a Pairs slice, and returns the new
// slice without changing the value of the original slice:
//
//	p = p.Append(pair, pare, pear)
func (p Pairs[K, V]) Append(new ...Pair[K, V]) Pairs[K, V] { return append(p, new...) }

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p Pairs[K, V]) Map() map[K]V {
	out := make(map[K]V, len(p))

	for idx := range p {
		if _, ok := out[p[idx].Key]; ok {
			continue
		}

		out[p[idx].Key] = p[idx].Value
	}
	return out
}
