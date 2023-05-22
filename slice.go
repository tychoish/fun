package fun

import (
	"sort"

	"github.com/tychoish/fun/internal"
)

// Slice is just a local wrapper around a slice, providing a similarly
// expressive interface to the Map and Pair types in the fun package.
type Slice[T any] []T

func (s Slice[T]) Add(in T) Slice[T]                    { return append(s, in) }
func (s Slice[T]) Sort(cp func(i, j int) bool) Slice[T] { sort.Slice(s, cp); return s }
func (s Slice[T]) Append(in ...T) Slice[T]              { return s.Extend(in) }
func (s Slice[T]) Extend(in []T) Slice[T]               { return append(s, in...) }
func (s Slice[T]) Iterator() Iterator[T]                { return internal.NewSliceIter(s) }
func (s Slice[T]) Len() int                             { return len(s) }
func (s Slice[T]) Cap() int                             { return cap(s) }

func IndexOf[T comparable](s Slice[T], item T) int {
	for idx, it := range s {
		if it == item {
			return idx
		}
	}
	return -1
}
