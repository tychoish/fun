package dt

import (
	"context"
	"encoding/json"
	"iter"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/internal"
)

// Tuples implements a collection of key-value tuples.
type Tuples[K any, V any] struct {
	ll    *List[Tuple[K, V]]
	setup sync.Once
}

// MakeTuples constructs a Tuples object from a sequence of Tuples. This
// is identical to using the literal constructor but may be more
// ergonomic as the compiler seems to be better at inferring types in
// function calls over literal constructors.
//
// To build Tuples objects from other types, use the Consume methods.
func MakeTuples[K any, V any](in ...Tuple[K, V]) *Tuples[K, V] {
	p := &Tuples[K, V]{}
	p.Append(in...)
	return p
}

// ConsumeTuples creates a *Tuples[K,V] object from a stream of
// Tuple[K,V] objects.
func ConsumeTuples[K any, V any](iter *fun.Stream[Tuple[K, V]]) fun.Generator[*Tuples[K, V]] {
	return func(ctx context.Context) (*Tuples[K, V], error) {
		p := &Tuples[K, V]{}
		if err := p.Consume(iter).Run(ctx); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func (p *Tuples[K, V]) init()     { p.setup.Do(p.initImpl) }
func (p *Tuples[K, V]) initImpl() { ft.WhenCall(p.ll == nil, func() { p.ll = &List[Tuple[K, V]]{} }) }

// Consume adds items from a stream of tuples to the current Tuples slice.
func (p *Tuples[K, V]) Consume(iter *fun.Stream[Tuple[K, V]]) fun.Worker {
	return iter.ReadAll(func(item Tuple[K, V]) { p.Push(item) })
}

// Iterator returns a native go iterator function for tuples.
func (p *Tuples[K, V]) Iterator() iter.Seq[Tuple[K, V]] {
	return func(yield func(value Tuple[K, V]) bool) {
		p.init()
		for item := p.ll.Front(); item.Ok(); item = item.Next() {
			if !yield(item.Value()) {
				return
			}
		}
	}
}

// Iterator2 returns a native go iterator over the items in a collections of tuples.
func (p *Tuples[K, V]) Iterator2() iter.Seq2[K, V] {
	return func(yield func(key K, value V) bool) {
		for item := range p.Iterator() {
			if !yield(item.One, item.Two) {
				return
			}
		}
	}
}

// Stream return a stream over each key-value tuples.
func (p *Tuples[K, V]) Stream() *fun.Stream[Tuple[K, V]] { p.init(); return p.ll.StreamFront() }

// Ones returns a stream over only the first item from a sequence of
// tuples.
func (p *Tuples[K, V]) Ones() *fun.Stream[K] {
	return fun.MakeConverter(func(p Tuple[K, V]) K { return p.One }).Stream(p.Stream())
}

// Twos returns a stream over only the second item from a sequence of
// tuples.
func (p *Tuples[K, V]) Twos() *fun.Stream[V] {
	return fun.MakeConverter(func(p Tuple[K, V]) V { return p.Two }).Stream(p.Stream())
}

// Slice creates a new slice of all the Tuple objects.
func (p *Tuples[K, V]) Slice() []Tuple[K, V] { return p.ll.Slice() }

// List returns the sequence of tuples as a list.
func (p *Tuples[K, V]) List() *List[Tuple[K, V]] { p.init(); return p.ll.Copy() }

// Copy produces a new Tuples object with the same values.
func (p *Tuples[K, V]) Copy() *Tuples[K, V] { return &Tuples[K, V]{ll: p.List()} }

// SortMerge performs a merge sort on the collected tuples.
func (p *Tuples[K, V]) SortMerge(c cmp.LessThan[Tuple[K, V]]) { p.init(); p.ll.SortMerge(c) }

// SortQuick does a quick sort using sort.StableSort. Typically faster than
// SortMerge, but potentially more memory intensive for some types.
func (p *Tuples[K, V]) SortQuick(c cmp.LessThan[Tuple[K, V]]) { p.init(); p.ll.SortQuick(c) }

// Len returns the number of items in the tuples object.
func (p *Tuples[K, V]) Len() int { p.init(); return p.ll.Len() }

// ReadAll returns a worker, that when executed calls the processor
// function for every tuple in the container.
func (p *Tuples[K, V]) ReadAll(pf fun.Handler[Tuple[K, V]]) fun.Worker {
	return func(ctx context.Context) error {
		p.init()
		if p.ll.Len() == 0 {
			return nil
		}

		for item := range p.Iterator() {
			if err := pf(ctx, item); err != nil {
				return err
			}
		}
		return nil
	}
}

// Add adds a new value to the underlying slice. This may add a
// duplicate key. The return value is provided to support chaining
// Add() operations
func (p *Tuples[K, V]) Add(k K, v V) *Tuples[K, V] { p.Push(Tuple[K, V]{One: k, Two: v}); return p }

// Push adds a single tuple to the slice of tuples. This may add a
// duplicate key.
func (p *Tuples[K, V]) Push(tuple Tuple[K, V]) { p.init(); p.ll.PushBack(tuple) }

// Append as a collection of tuples to the collection of key/value
// tuples.
func (p *Tuples[K, V]) Append(vals ...Tuple[K, V]) { p.init(); p.ll.Append(vals...) }

// Extend adds the items from a Tuples object (slice of Tuple) without
// modifying the donating object.
func (p *Tuples[K, V]) Extend(toAdd *Tuples[K, V]) { p.init(); p.ll.Extend(toAdd.ll) }

// MarshalJSON produces a JSON encoding for the Pairs object by first
// converting it to a map and then encoding that map as JSON. The JSON
// serialization does not necessarily preserve the order of the pairs
// object.
func (p *Tuples[K, V]) MarshalJSON() ([]byte, error) {
	buf := &internal.IgnoreNewLinesBuffer{}
	enc := json.NewEncoder(buf)
	buf.WriteByte('[')

	idx := 0
	if err := p.ReadAll(fun.MakeHandler(func(item Tuple[K, V]) error {
		if idx != 0 {
			buf.WriteByte(',')
		}
		idx++
		if err := enc.Encode(item); err != nil {
			return erc.Wrapf(err, "tuple at index %d", idx)
		}

		return nil
	})).Wait(); err != nil {
		return nil, err
	}

	buf.WriteByte(']')
	return buf.Bytes(), nil
}

// UnmarshalJSON provides consistent JSON decoding for Pairs
// objects. It reads a JSON document into a map and converts it to
// pairs, and appends it to the existing Pairs objects without
// removing or resetting the current object.
func (p *Tuples[K, V]) UnmarshalJSON(in []byte) error {
	t := []json.RawMessage{}
	if err := json.Unmarshal(in, &t); err != nil {
		return err
	}
	p.init()
	for idx, item := range t {
		tuple := Tuple[K, V]{}

		if err := json.Unmarshal(item, &tuple); err != nil {
			return erc.Wrapf(err, "tuple at index %d", idx)
		}
		p.ll.PushBack(tuple)
	}

	return nil
}
