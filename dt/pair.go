package dt

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ft"
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
type Pairs[K comparable, V any] struct {
	ll    *List[Pair[K, V]]
	setup sync.Once
}

// MakePairs constructs a Pairs object from a sequence of Pairs. This
// is identical to using the literal constructor but may be more
// ergonomic as the compiler seems to be better at inferring types in
// function calls over literal constructors.
//
// To build Pairs objects from other types, use the Consume methods.
func MakePairs[K comparable, V any](in ...Pair[K, V]) *Pairs[K, V] {
	p := &Pairs[K, V]{}
	p.init()
	p.Append(in...)
	return p
}

// ConsumePairs creates a *Pairs[K,V] object from an iterator of
// Pair[K,V] objects.
func ConsumePairs[K comparable, V any](
	ctx context.Context,
	iter *fun.Iterator[Pair[K, V]],
) (*Pairs[K, V], error) {
	p := &Pairs[K, V]{}
	p.init()
	if err := iter.Observe(ctx, func(in Pair[K, V]) {
		p.AddPair(in)
	}); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Pairs[K, V]) init() { p.setup.Do(p.initalizeList) }
func (p *Pairs[K, V]) initalizeList() {
	if p.ll == nil {
		p.ll = &List[Pair[K, V]]{}
	}
}

// Iterator return an iterator over each key-value pairs.
func (p *Pairs[K, V]) Iterator() *fun.Iterator[Pair[K, V]] { p.init(); return p.ll.Iterator() }

// Slice creates a new slice of all the Pair objects.
func (p *Pairs[K, V]) Slice() []Pair[K, V] {
	p.init()
	return ft.Must(p.ll.Iterator().Slice(context.Background()))
}

// List returns the sequence of pairs as a list.
func (p *Pairs[K, V]) List() *List[Pair[K, V]] { p.init(); return p.ll.Copy() }

// SortMerge performs a merge sort on the collected pairs.
func (p *Pairs[K, V]) SortMerge(c cmp.LessThan[Pair[K, V]]) { p.init(); p.ll.SortMerge(c) }

// SortQuick does a quick sort using sort.Sort. Typically faster than
// SortMerge, but potentially more memory intensive for some types.
func (p *Pairs[K, V]) SortQuick(c cmp.LessThan[Pair[K, V]]) { p.init(); p.ll.SortQuick(c) }

// Len returns the number of items in the pairs object.
func (p *Pairs[K, V]) Len() int { p.init(); return p.ll.Len() }

// Keys returns an iterator over only the keys in a sequence of
// iterator items.
func (p *Pairs[K, V]) Keys() *fun.Iterator[K] {
	p.init()
	return fun.Converter(func(p Pair[K, V]) K { return p.Key }).Convert(p.Iterator())
}

// Values returns an iterator over only the values in a sequence of
// iterator pairs.
func (p *Pairs[K, V]) Values() *fun.Iterator[V] {
	p.init()
	return fun.Converter(func(p Pair[K, V]) V { return p.Value }).Convert(p.Iterator())
}

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p *Pairs[K, V]) MarshalJSON() ([]byte, error) {
	p.init()

	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	_, _ = buf.Write([]byte("{"))
	first := true

	for li := p.ll.Front(); li.Ok(); li = li.Next() {
		item := li.Value()

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
func (p *Pairs[K, V]) Add(k K, v V) *Pairs[K, V] { return p.AddPair(Pair[K, V]{Key: k, Value: v}) }

// AddPair adds a single pair to the slice of pairs. This may add a
// duplicate key.
func (p *Pairs[K, V]) AddPair(pair Pair[K, V]) *Pairs[K, V] { p.init(); p.ll.PushBack(pair); return p }

// Append as a collection of pairs to the collection of key/value
// pairs.
func (p *Pairs[K, V]) Append(new ...Pair[K, V]) *Pairs[K, V] { p.init(); p.ll.Append(new...); return p }

// Extend adds the items from a Pairs object (slice of Pair) without
// modifying the donating object.
func (p *Pairs[K, V]) Extend(toAdd *Pairs[K, V]) { p.init(); p.ll.Extend(toAdd.ll) }

// Consume adds items from an iterator of pairs to the current Pairs slice.
func (p *Pairs[K, V]) Consume(ctx context.Context, iter *fun.Iterator[Pair[K, V]]) error {
	return iter.Observe(ctx, func(item Pair[K, V]) { p.AddPair(item) })
}

// ConsumeValues adds all of the values in the input iterator,
// generating the keys using the function provided.
func (p *Pairs[K, V]) ConsumeValues(ctx context.Context, iter *fun.Iterator[V], keyf func(V) K) error {
	return p.Consume(ctx, fun.Converter(func(in V) Pair[K, V] { return MakePair(keyf(in), in) }).Convert(iter))
}

// ConsumeMap adds all of the items in a map to the Pairs object.
func (p *Pairs[K, V]) ConsumeMap(in map[K]V) *Pairs[K, V] {
	fun.Invariant.Must(p.Consume(context.Background(), Mapify(in).Iterator()))
	return p
}

// ConsumeSlice adds all the values in the input slice to the Pairs
// object, creating the keys using the function provide.
func (p *Pairs[K, V]) ConsumeSlice(in []V, keyf func(V) K) *Pairs[K, V] {
	Sliceify(in).Observe(func(value V) { p.Add(keyf(value), value) })
	return p
}

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p *Pairs[K, V]) Map() map[K]V {
	p.init()
	out := make(map[K]V, p.ll.Len())
	for i := p.ll.Front(); i.Ok(); i = i.Next() {
		pair := i.Value()
		if _, ok := out[pair.Key]; ok {
			continue
		}

		out[pair.Key] = pair.Value

	}

	return out
}
