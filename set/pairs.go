package set

import (
	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
)

// Pair represents a key-value pair.
type Pair[K comparable, V comparable] struct {
	Key   K
	Value V
}

// Pairs implements a collection of key-value pairs.
type Pairs[K comparable, V comparable] []Pair[K, V]

// Add adds a new value to the underlying slice.
func (p *Pairs[K, V]) Add(k K, v V) { *p = p.Append(Pair[K, V]{Key: k, Value: v}) }

// Append, mirroring the semantics of the built in append() function
// adds one or more Pair items to a Pairs slice, and returns the new
// slice without changing the value of the original slice:
//
//	p = p.Append(pair, pare, pear)
func (p Pairs[K, V]) Append(new ...Pair[K, V]) Pairs[K, V] { return append(p, new...) }

// MakePairs converts a map type into a slice of Pair types
// that can be usable in a set.
func MakePairs[K comparable, V comparable](in map[K]V) Pairs[K, V] {
	out := make([]Pair[K, V], 0, len(in))
	for k, v := range in {
		out = append(out, Pair[K, V]{Key: k, Value: v})
	}
	return out
}

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p Pairs[K, V]) Map() map[K]V {
	out := make(map[K]V, len(p))
	for idx := range p {
		if checkInMap(p[idx].Key, out) {
			continue
		}
		out[p[idx].Key] = p[idx].Value
	}
	return out
}

// Set converts a Pairs object into a set.
func (p Pairs[K, V]) Set() Set[Pair[K, V]] {
	return BuildUnordered(internal.BackgroundContext, fun.Iterator[Pair[K, V]](internal.NewSliceIter(p)))
}

// OrderedSet produces an order-preserving set based on the Pairs.
func (p Pairs[K, V]) OrderedSet() Set[Pair[K, V]] {
	return BuildOrdered(internal.BackgroundContext, fun.Iterator[Pair[K, V]](internal.NewSliceIter(p)))
}
