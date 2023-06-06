package set

import (
	"context"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/internal"
	"github.com/tychoish/fun/itertool"
	"github.com/tychoish/fun/seq"
)

type orderedLLSet[T comparable] struct {
	set   fun.Map[T, *seq.Element[T]]
	elems seq.List[T]
}

// MakeOrdered constructs an ordered set implementation.
func MakeOrdered[T comparable]() Set[T] {
	return &orderedLLSet[T]{set: fun.Map[T, *seq.Element[T]]{}}
}

// BuildOrdered creates an ordered set (new implementation) from
// the contents of the input iterator.
func BuildOrdered[T comparable](ctx context.Context, iter *fun.Iterator[T]) Set[T] {
	set := MakeOrdered[T]()
	Populate(ctx, set, iter)
	return set
}

func (lls *orderedLLSet[T]) Add(it T) {
	if lls.set.Check(it) {
		return
	}

	lls.elems.PushBack(it)

	lls.set[it] = lls.elems.Back()
}

func (lls *orderedLLSet[T]) Producer() fun.Producer[T]  { return lls.elems.Producer() }
func (lls *orderedLLSet[T]) Iterator() *fun.Iterator[T] { return lls.elems.Iterator() }
func (lls *orderedLLSet[T]) Len() int                   { return lls.elems.Len() }
func (lls *orderedLLSet[T]) Check(it T) bool            { return lls.set.Check(it) }
func (lls *orderedLLSet[T]) Delete(it T) {
	e, ok := lls.set.Load(it)
	if !ok {
		return
	}

	e.Remove()
	delete(lls.set, it)
}

func (lls *orderedLLSet[T]) MarshalJSON() ([]byte, error) {
	return itertool.MarshalJSON(internal.BackgroundContext, lls.Iterator())
}

func (lls *orderedLLSet[T]) UnmarshalJSON(in []byte) error {
	iter := itertool.UnmarshalJSON[T](in)
	Populate[T](internal.BackgroundContext, lls, iter)
	return iter.Close()
}
