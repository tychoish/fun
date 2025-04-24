package dt

import (
	"context"
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

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
	p.Append(in...)
	return p
}

// ConsumePairs creates a *Pairs[K,V] object from a stream of
// Pair[K,V] objects.
func ConsumePairs[K comparable, V any](iter *fun.Stream[Pair[K, V]]) fun.Generator[*Pairs[K, V]] {
	return func(ctx context.Context) (*Pairs[K, V], error) {
		p := &Pairs[K, V]{}
		if err := p.Consume(iter).Run(ctx); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func (p *Pairs[K, V]) init()     { p.setup.Do(p.initImpl) }
func (p *Pairs[K, V]) initImpl() { ft.WhenCall(p.ll == nil, func() { p.ll = &List[Pair[K, V]]{} }) }

// Consume adds items from a stream of pairs to the current Pairs slice.
func (p *Pairs[K, V]) Consume(iter *fun.Stream[Pair[K, V]]) fun.Worker {
	return iter.ReadAll(func(item Pair[K, V]) { p.Push(item) })
}

// Stream return a stream over each key-value pairs.
func (p *Pairs[K, V]) Stream() *fun.Stream[Pair[K, V]] { p.init(); return p.ll.StreamFront() }

// Keys returns a stream over only the keys in a sequence of
// iterator items.
func (p *Pairs[K, V]) Keys() *fun.Stream[K] {
	return fun.MakeConverter(func(p Pair[K, V]) K { return p.Key }).Stream(p.Stream())
}

// Values returns a stream over only the values in a sequence of
// iterator pairs.
func (p *Pairs[K, V]) Values() *fun.Stream[V] {
	return fun.MakeConverter(func(p Pair[K, V]) V { return p.Value }).Stream(p.Stream())
}

// Slice creates a new slice of all the Pair objects.
func (p *Pairs[K, V]) Slice() []Pair[K, V] { return p.ll.Slice() }

// List returns the sequence of pairs as a list.
func (p *Pairs[K, V]) List() *List[Pair[K, V]] { p.init(); return p.ll.Copy() }

// Copy produces a new Pairs object with the same values.
func (p *Pairs[K, V]) Copy() *Pairs[K, V] { return &Pairs[K, V]{ll: p.List()} }

// SortMerge performs a merge sort on the collected pairs.
func (p *Pairs[K, V]) SortMerge(c cmp.LessThan[Pair[K, V]]) { p.init(); p.ll.SortMerge(c) }

// SortQuick does a quick sort using sort.StableSort. Typically faster than
// SortMerge, but potentially more memory intensive for some types.
func (p *Pairs[K, V]) SortQuick(c cmp.LessThan[Pair[K, V]]) { p.init(); p.ll.SortQuick(c) }

// Len returns the number of items in the pairs object.
func (p *Pairs[K, V]) Len() int { p.init(); return p.ll.Len() }

// ReadAll returns a worker, that when executed calls the processor
// function for every pair in the container.
func (p *Pairs[K, V]) ReadAll(pf fun.Handler[Pair[K, V]]) fun.Worker {
	return func(ctx context.Context) error {
		p.init()
		if p.ll.Len() == 0 {
			return nil
		}

		for val := range p.Iterator() {
			if err := pf(ctx, val); err != nil {
				return err
			}
		}

		return nil
	}
}

// Add adds a new value to the underlying slice. This may add a
// duplicate key. The return value is provided to support chaining
// Add() operations
func (p *Pairs[K, V]) Add(k K, v V) *Pairs[K, V] { p.Push(Pair[K, V]{Key: k, Value: v}); return p }

// Push adds a single pair to the slice of pairs. This may add a
// duplicate key.
func (p *Pairs[K, V]) Push(pair Pair[K, V]) { p.init(); p.ll.PushBack(pair) }

// Append as a collection of pairs to the collection of key/value
// pairs.
func (p *Pairs[K, V]) Append(vals ...Pair[K, V]) { p.init(); p.ll.Append(vals...) }

// Extend adds the items from a Pairs object (slice of Pair) without
// modifying the donating object.
func (p *Pairs[K, V]) Extend(toAdd *Pairs[K, V]) { p.init(); p.ll.Extend(toAdd.ll) }

// Iterator returns a native go iterator function for pair items.
func (p *Pairs[K, V]) Iterator() iter.Seq[Pair[K, V]] {
	return func(yield func(value Pair[K, V]) bool) {
		p.init()
		for item := p.ll.Front(); item.Ok(); item = item.Next() {
			if !yield(item.Value()) {
				return
			}
		}
	}
}

// Iterator2 returns a native go iterator function for the items in a pairs
// sequence.
func (p *Pairs[K, V]) Iterator2() iter.Seq2[K, V] {
	return func(yield func(key K, value V) bool) {
		for item := range p.Iterator() {
			if !yield(item.Key, item.Value) {
				return
			}
		}
	}
}

// This file contains map-specific pair functionality that cannot be
// used to form the tuple type.

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p *Pairs[K, V]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	buf.WriteByte('{')

	idx := 0
	if err := p.ReadAll(fun.MakeHandler(func(item Pair[K, V]) error {
		if idx != 0 {
			buf.WriteByte(',')
		}
		idx++
		if err := enc.Encode(item.Key); err != nil {
			return ers.Wrapf(err, "key at %d", idx)
		}
		buf.WriteByte(':')

		return ers.Wrapf(enc.Encode(item.Value), "key at %d", idx)
	})).Wait(); err != nil {
		return nil, err
	}

	buf.WriteByte('}')
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

// ConsumeValues adds all of the values from the input stream,
// generating the keys using the function provided.
func (p *Pairs[K, V]) ConsumeValues(iter *fun.Stream[V], keyf func(V) K) fun.Worker {
	return p.Consume(fun.MakeConverter(func(in V) Pair[K, V] { return MakePair(keyf(in), in) }).Stream(iter))
}

// ConsumeSlice adds all the values from the input slice to the Pairs
// object, creating the keys using the function provide.
func (p *Pairs[K, V]) ConsumeSlice(in []V, keyf func(V) K) {
	NewSlice(in).ReadAll(func(value V) { p.Add(keyf(value), value) })
}

// ConsumeMap adds all of the items in a map to the Pairs object.
func (p *Pairs[K, V]) ConsumeMap(in map[K]V) { NewMap(in).Stream().ReadAll(p.Push).Ignore().Wait() }

// Map converts a list of pairs to the equivalent map. If there are
// duplicate keys in the Pairs list, only the first occurrence of the
// key is retained.
func (p *Pairs[K, V]) Map() Map[K, V] { m := NewMap(map[K]V{}); m.ConsumePairs(p); return m }
