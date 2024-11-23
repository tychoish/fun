package dt

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ft"
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
func ConsumePairs[K comparable, V any](iter *fun.Iterator[Pair[K, V]]) fun.Producer[*Pairs[K, V]] {
	return func(ctx context.Context) (*Pairs[K, V], error) {
		p := &Pairs[K, V]{}
		if err := p.Consume(iter).Run(ctx); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func (p *Pairs[K, V]) init() { p.setup.Do(p.initalizeList) }
func (p *Pairs[K, V]) initalizeList() {
	ft.WhenCall(p.ll == nil, func() { p.ll = &List[Pair[K, V]]{} })
}

// Consume adds items from an iterator of pairs to the current Pairs slice.
func (p *Pairs[K, V]) Consume(iter *fun.Iterator[Pair[K, V]]) fun.Worker {
	return iter.Observe(func(item Pair[K, V]) { p.Push(item) })
}

// Iterator return an iterator over each key-value pairs.
func (p *Pairs[K, V]) Iterator() *fun.Iterator[Pair[K, V]] { p.init(); return p.ll.Iterator() }

// Keys returns an iterator over only the keys in a sequence of
// iterator items.
func (p *Pairs[K, V]) Keys() *fun.Iterator[K] {
	return fun.Converter(func(p Pair[K, V]) K { return p.Key }).Process(p.Iterator())
}

// Values returns an iterator over only the values in a sequence of
// iterator pairs.
func (p *Pairs[K, V]) Values() *fun.Iterator[V] {
	return fun.Converter(func(p Pair[K, V]) V { return p.Value }).Process(p.Iterator())
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

// Observe calls the handler function for every pair in the container.
func (p *Pairs[K, V]) Observe(hf fun.Handler[Pair[K, V]]) { p.Process(hf.Processor()).Ignore().Wait() }

// Process returns a worker, that when executed calls the processor
// function for every pair in the container.
func (p *Pairs[K, V]) Process(pf fun.Processor[Pair[K, V]]) fun.Worker {
	return func(ctx context.Context) error {
		p.init()
		if p.ll.Len() == 0 {
			return nil
		}
		for l := p.ll.Front(); l.Ok(); l = l.Next() {
			if err := pf(ctx, l.Value()); err != nil {
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
