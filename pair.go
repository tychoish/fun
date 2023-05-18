package fun

import (
	"context"
	"encoding/json"

	"github.com/tychoish/fun/internal"
)

// Pair represents a key-value pair. Used by the adt synchronized map
// implementation and the set package to handle ordered key-value pairs.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// MakePair constructs a pair object. This is identical to using the
// literal constructor but may be more ergonomic as the compiler seems
// to be better at inferring types in function calls over literal
// constructors.
func MakePair[K comparable, V any](k K, v V) Pair[K, V] { return Pair[K, V]{Key: k, Value: v} }

// Pairs implements a collection of key-value pairs.
type Pairs[K comparable, V any] []Pair[K, V]

// MakePairs constructs a Pairs object from a sequence of Pairs. This
// is identical to using the literal constructor but may be more
// ergonomic as the compiler seems to be better at inferring types in
// function calls over literal constructors.
//
// To build Pairs objects from other types, use the Consume methods.
func MakePairs[K comparable, V any](in ...Pair[K, V]) Pairs[K, V] { return in }

// Pair constructs an iterator of pairs, which it builds from a
// sequence of values, generating the keys using the provided key
// function.
func BuildPairs[K comparable, V any](ctx context.Context, iter Iterator[V], keyf func(V) K) Iterator[Pair[K, V]] {
	return Transform(iter, func(in V) (Pair[K, V], error) { return MakePair(keyf(in), in), nil })
}

// Iterator return an iterator over each key-value pairs.
func (p Pairs[K, V]) Iterator() Iterator[Pair[K, V]] { return internal.NewSliceIter(p) }

// Keys returns an iterator over only the keys in a sequence of
// iterator items.
func (p Pairs[K, V]) Keys() Iterator[K] { return PairKeys(p.Iterator()) }

// Values returns an iterator over only the values in a sequence of
// iterator pairs.
func (p Pairs[K, V]) Values() Iterator[V] { return PairValues(p.Iterator()) }

// PairKeys converts an iterator of Pairs to an iterator of its keys.
func PairKeys[K comparable, V any](iter Iterator[Pair[K, V]]) Iterator[K] {
	return Transform(iter, func(p Pair[K, V]) (K, error) { return p.Key, nil })
}

// PairValues converts an iterator of pairs to an iterator of its values.
func PairValues[K comparable, V any](iter Iterator[Pair[K, V]]) Iterator[V] {
	return Transform(iter, func(p Pair[K, V]) (V, error) { return p.Value, nil })
}

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p Pairs[K, V]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	_, _ = buf.Write([]byte("{"))
	first := true
	for _, item := range p {
		if first {
			first = false
		} else {
			_, _ = buf.Write([]byte(","))
		}
		if err := enc.Encode(item.Key); err != nil {
			return nil, err
		}
		_, _ = buf.Write([]byte(":"))
		if err := enc.Encode(item.Value); err != nil {
			return nil, err
		}
	}
	_, _ = buf.Write([]byte("}"))

	return buf.Bytes(), nil
}

// UnmarshalJSON provides consistent JSON decoding for Pairs
// objects. It reads a JSON document into a map and converts it to
// pairs, and appends it to the existing Pairs objects without
// removing or resetting the current object.
func (p *Pairs[K, V]) UnmarshalJSON(in []byte) error {
	t := make(map[K]V)
	if err := json.Unmarshal(in, &t); err != nil {
		return err
	}

	p.ConsumeMap(t)
	return nil
}

// Add adds a new value to the underlying slice. This may add a
// duplicate key.
func (p *Pairs[K, V]) Add(k K, v V) { *p = p.Append(Pair[K, V]{Key: k, Value: v}) }

// AddPair adds a single pair to the slice of pairs.
func (p *Pairs[K, V]) AddPair(pair Pair[K, V]) { *p = p.Append(pair) }

// Append, mirroring the semantics of the built in append() function
// adds one or more Pair items to a Pairs slice, and returns the new
// slice without changing the value of the original slice:
//
//	p = p.Append(pair, pare, pear)
func (p Pairs[K, V]) Append(new ...Pair[K, V]) Pairs[K, V] { return append(p, new...) }

// Extend adds the items from a Pairs object (slice of Pair) without
// modifying the donating object.
func (p *Pairs[K, V]) Extend(toAdd Pairs[K, V]) { *p = append(*p, toAdd...) }

// Consume adds items from an iterator of pairs to the current Pairs slice.
func (p *Pairs[K, V]) Consume(ctx context.Context, iter Iterator[Pair[K, V]]) error {
	return Observe(ctx, iter, func(item Pair[K, V]) { p.AddPair(item) })
}

// ConsumeValues adds all of the values in the input iterator,
// generating the keys using the function provided.
func (p *Pairs[K, V]) ConsumeValues(ctx context.Context, iter Iterator[V], keyf func(V) K) error {
	return p.Consume(ctx, BuildPairs(ctx, iter, keyf))
}

// ConsumeMap adds all of the items in a map to the Pairs object.
func (p *Pairs[K, V]) ConsumeMap(in map[K]V) {
	InvariantMust(p.Consume(internal.BackgroundContext, Mapify(in).Iterator()))
}

// ConsumeSlice adds all the values in the input slice to the Pairs
// object, creating the keys using the function provide.
func (p *Pairs[K, V]) ConsumeSlice(in []V, keyf func(V) K) {
	for idx := range in {
		value := in[idx]
		p.Add(keyf(value), value)
	}
}

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
