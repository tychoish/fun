package fun

import (
	"context"
	"io"
	"sort"
)

// Slice is just a local wrapper around a slice, providing a similarly
// expressive interface to the Map and Pair types in the fun package.
type Slice[T any] []T

// Sliceify produces a slice object as a convenience constructor.
func Sliceify[T any](in []T) Slice[T]  { return in }
func Variadic[T any](in ...T) Slice[T] { return in }

// Iterator returns an iterator to the items of the slice the range
// keyword also works for these slices.
func (s Slice[T]) Iterator() *Iterator[T] {
	var idx int = -1
	return Producer[T](func(ctx context.Context) (out T, _ error) {
		if len(s) <= idx+1 {
			return out, io.EOF
		}
		idx++
		return s[idx], ctx.Err()
	}).Iterator()
}

// Sort reorders the slice using the provided com parator function,
// which should return true if a is less than b and, false
// otherwise. The slice is sorted in "lowest" to "highest" order.
func (s Slice[T]) Sort(cp func(a, b T) bool) {
	sort.Slice(s, func(i, j int) bool { return cp(s[i], s[j]) })
}

// Add adds a single item to the slice.
func (s *Slice[T]) Add(in T)                { *s = append(*s, in) }
func (s *Slice[T]) AddWhen(cond bool, in T) { WhenCall(cond, func() { s.Add(in) }) }

// Append adds all of the items to the slice.
func (s *Slice[T]) Append(in ...T)                { s.Extend(in) }
func (s *Slice[T]) AppendWhen(cond bool, in ...T) { WhenCall(cond, func() { s.Extend(in) }) }

// Extend adds the items from the input slice to the root slice.
func (s *Slice[T]) Extend(in []T)                { *s = append(*s, in...) }
func (s *Slice[T]) ExtendWhen(cond bool, in []T) { WhenCall(cond, func() { s.Extend(in) }) }

// Copy performs a shallow copy of the Slice.
func (s Slice[T]) Copy() Slice[T] { out := make([]T, len(s)); copy(out, s); return out }

// Len returns the length of the slice.
func (s Slice[T]) Len() int { return len(s) }

// Cap returns the capacity of the slice.
func (s Slice[T]) Cap() int { return cap(s) }

// Empty re-slices the slice, to omit all items, but retain the allocation.
func (s *Slice[T]) Empty() { *s = (*s)[:0] }

// Reset constructs a new empty slice releasing the original
// allocation.
func (s *Slice[T]) Reset() { o := make([]T, 0); *s = o }

func (s *Slice[T]) Observe(ob Observer[T]) {
	sl := *s
	for idx := range sl {
		ob(sl[idx])
	}
}

func (s *Slice[T]) Filter(p func(T) bool) (o Slice[T]) {
	s.Observe(func(in T) { o.AddWhen(p(in), in) })
	return
}

// Reslice modifies the slice to set the new start and end indexes.
//
// Slicing operations, can lead to panics if the indexes are out of
// bounds.
func (s *Slice[T]) Reslice(start, end int) { *s = (*s)[start:end] }

// ResliceBeginning moves the "beginning" of the slice to the
// specified index.
//
// Slicing operations, can lead to panics if the indexes are out of
// bounds.
func (s *Slice[T]) ResliceBeginning(start int) { *s = (*s)[start:] }

// ResliceEnd moves the "end" of the slice to the specified index.
//
// Slicing operations, can lead to panics if the indexes are out of
// bounds.
func (s *Slice[T]) ResliceEnd(end int) { *s = (*s)[:end] }

// Truncate removes the last n items from the end of the list.
//
// Slicing operations, can lead to panics if the indexes are out of
// bounds.
func (s *Slice[T]) Truncate(n int) { *s = (*s)[:len(*s)-n] }

// Last returns the index of the last element in the slice. Empty
// slices have `-1` last items.
func (s Slice[T]) Last() int { return len(s) - 1 }

// IsEmpty returns true when there are no items in the slice (or it is
// nil.
func (s Slice[T]) IsEmpty() bool { return len(s) == 0 }

// Item returns the item at the specified index.
//
// If the provided index is not within the bounds of the slice the
// operation panics.
func (s Slice[T]) Item(index int) T { return s[index] }

func (s Slice[T]) Observe(of Observer[T]) {
	for idx := range s {
		of(s[idx])
	}
}

func (s Slice[T]) Process(pf Processor[T]) Worker {
	return func(ctx context.Context) error {
		for idx := range s {
			if err := pf(ctx, s[idx]); err != nil {
				return err
			}
		}
		return nil
	}
}
