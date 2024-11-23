// GENERATED FILE FROM PAIR IMPLEMENTATION
package dt

import (
	"context"
	"sync"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/dt/cmp"
	"github.com/tychoish/fun/ft"
)

// Tuple represents a key-value tuple. Used by the adt synchronized map
// implementation and the set package to handle ordered key-value tuples.
type Tuple[K any, V any] struct {
	One K
	Two V
}

// MakeTuple constructs a tuple object. This is identical to using the
// literal constructor but may be more ergonomic as the compiler seems
// to be better at inferring types in function calls over literal
// constructors.
func MakeTuple[K any, V any](k K, v V) Tuple[K, V] { return Tuple[K, V]{One: k, Two: v} }

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
	p.init()
	p.Append(in...)
	return p
}

// ConsumeTuples creates a *Tuples[K,V] object from an iterator of
// Tuple[K,V] objects.
func ConsumeTuples[K any, V any](iter *fun.Iterator[Tuple[K, V]]) fun.Producer[*Tuples[K, V]] {
	return func(ctx context.Context) (*Tuples[K, V], error) {
		p := &Tuples[K, V]{}
		if err := p.Consume(iter).Run(ctx); err != nil {
			return nil, err
		}
		return p, nil
	}
}

func (p *Tuples[K, V]) init() { p.setup.Do(p.initalizeList) }
func (p *Tuples[K, V]) initalizeList() {
	ft.WhenCall(p.ll == nil, func() { p.ll = &List[Tuple[K, V]]{} })
}

// Consume adds items from an iterator of tuples to the current Tuples slice.
func (p *Tuples[K, V]) Consume(iter *fun.Iterator[Tuple[K, V]]) fun.Worker {
	return iter.Observe(func(item Tuple[K, V]) { p.Push(item) })
}

// Iterator return an iterator over each key-value tuples.
func (p *Tuples[K, V]) Iterator() *fun.Iterator[Tuple[K, V]] { p.init(); return p.ll.Iterator() }

// Ones returns an iterator over only the keys in a sequence of
// iterator items.
func (p *Tuples[K, V]) Ones() *fun.Iterator[K] {
	return fun.Converter(func(p Tuple[K, V]) K { return p.One }).Process(p.Iterator())
}

// Twos returns an iterator over only the values in a sequence of
// iterator tuples.
func (p *Tuples[K, V]) Twos() *fun.Iterator[V] {
	return fun.Converter(func(p Tuple[K, V]) V { return p.Two }).Process(p.Iterator())
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

// Observe calls the handler function for every tuple in the container.
func (p *Tuples[K, V]) Observe(hf fun.Handler[Tuple[K, V]]) {
	p.Process(hf.Processor()).Ignore().Wait()
}

// Process returns a worker, that when executed calls the processor
// function for every tuple in the container.
func (p *Tuples[K, V]) Process(pf fun.Processor[Tuple[K, V]]) fun.Worker {
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
