package dt

import (
	"context"
	"errors"
	"io"
	"sort"

	"github.com/tychoish/fun"
	"github.com/tychoish/fun/ers"
	"github.com/tychoish/fun/ft"
)

// Slice is just a local wrapper around a slice, providing a similarly
// expressive interface to the Map and Pair types in the fun package.
type Slice[T any] []T

// Sliceify produces a slice object as a convenience constructor.
func Sliceify[T any](in []T) Slice[T] { return in }

// SlicePtrs converts a slice of values to a slice of
// values. This is a helper for Sliceify(in).Ptrs().
func SlicePtrs[T any](in []T) Slice[*T] { return Sliceify(in).Ptrs() }

// SliceRefs converts a slice of pointers to a slice of objects,
// dropping nil values. from the output slice.
func SliceSparseRefs[T any](in []*T) Slice[T] {
	out := make([]T, 0, len(in))
	for idx := range in {
		if in[idx] == nil {
			continue
		}
		out = append(out, *in[idx])
	}
	return out
}

// SliceRefs converts a slice of pointers to a slice of values,
// replacing all nil pointers with the zero type for that value.
func SliceRefs[T any](in []*T) Slice[T] {
	var zero T
	out := make([]T, 0, len(in))
	for idx := range in {
		if in[idx] == nil {
			out = append(out, zero)
			continue
		}
		out = append(out, *in[idx])
	}
	return out
}

// Variadic constructs a slice of type T from a sequence of variadic
// options.
func Variadic[T any](in ...T) Slice[T] { return in }

// DefaultSlice takes a slice value and returns it if it's non-nil. If
// the slice is nil, it returns a slice of the specified length (and
// capacity,) as specified.
func DefaultSlice[T any](input []T, args ...int) Slice[T] {
	if input != nil {
		return input
	}
	switch len(args) {
	case 0:
		return Slice[T]{}
	case 1:
		return make([]T, args[0])
	case 2:
		return make([]T, args[0], args[1])
	default:
		panic(ers.Wrap(ers.ErrInvariantViolation, "cannot specify >2 arguments to make() for a slice"))
	}
}

// Transform processes a slice of one type into a slice of another
// type using the transformation function. Errors abort the
// transformation, with the exception of fun.ErrIteratorSkip. All
// errors are returned to the caller, except io.EOF which indicates
// the (early) end of iteration.
func Transform[T any, O any](in Slice[T], op func(T) (O, error)) (Slice[O], error) {
	out := make([]O, 0, len(in))

	for idx := range in {
		n, err := op(in[idx])
		switch {
		case err == nil:
			out = append(out, n)
		case errors.Is(err, fun.ErrIteratorSkip):
			continue
		case errors.Is(err, io.EOF):
			return out, nil
		default:
			return nil, err
		}
	}

	return out, nil
}

// Iterator returns an iterator to the items of the slice the range
// keyword also works for these slices.
func (s Slice[T]) Iterator() *fun.Iterator[T] { return fun.SliceIterator(s) }

// Sort reorders the slice using the provided com parator function,
// which should return true if a is less than b and, false
// otherwise. The slice is sorted in "lowest" to "highest" order.
func (s Slice[T]) Sort(cp func(a, b T) bool) {
	sort.Slice(s, func(i, j int) bool { return cp(s[i], s[j]) })
}

// Add adds a single item to the end of the slice.
func (s *Slice[T]) Add(in T) { *s = append(*s, in) }

// AddWhen embeds a conditional check in the Add, and only adds the item to the
// slice when the condition is true.
func (s *Slice[T]) AddWhen(cond bool, in T) { ft.WhenCall(cond, func() { s.Add(in) }) }

// Append adds all of the items to the end of the slice.
func (s *Slice[T]) Append(in ...T) { s.Extend(in) }

// Prepend adds the items to beginning of the slice.
func (s *Slice[T]) Prepend(in ...T) { *s = append(in, *s...) }

// AppendWhen embeds a conditional check in the Append operation, and
// only adds the items to the slice when the condition is true.
func (s *Slice[T]) AppendWhen(cond bool, in ...T) { ft.WhenCall(cond, func() { s.Extend(in) }) }

// Extend adds the items from the input slice to the root slice.
func (s *Slice[T]) Extend(in []T) { *s = append(*s, in...) }

// ExtendWhen embeds a conditional check in the Extend operatio and
// only adds the items to the slice when the condition is true.
func (s *Slice[T]) ExtendWhen(cond bool, in []T) { ft.WhenCall(cond, func() { s.Extend(in) }) }

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

// Observe calls the observer function on every item in the slice.
func (s Slice[T]) Observe(of fun.Handler[T]) {
	for idx := range s {
		of(s[idx])
	}
}

// Filter returns a new slice, having passed all the items in the
// input slice. Items that the filter function returns true for are
// included and others are skipped.
func (s *Slice[T]) Filter(p func(T) bool) (o Slice[T]) {
	s.Observe(func(in T) { o.AddWhen(p(in), in) })
	return
}

// FilterFuture returns a future that generates a new slice using the
// filter to select items from the root slice.
func (s *Slice[T]) FilterFuture(p func(T) bool) fun.Future[Slice[T]] {
	return func() Slice[T] { return s.Filter(p) }
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

// Index returns the item at the specified index.
//
// If the provided index is not within the bounds of the slice the
// operation panics.
func (s Slice[T]) Index(index int) T { return s[index] }

// Ptr provides a pointer to the item at the provided index.
func (s Slice[T]) Ptr(index int) *T { return &s[index] }

// Grow adds zero items to the slice until it reaches the desired
// size.
func (s *Slice[T]) Grow(size int) { *s = s.FillTo(size) }

// Zero replaces all items in the slice with the zero value for the
// type T.
func (s Slice[T]) Zero() {
	for idx := range s {
		var zero T
		s[idx] = zero
	}
}

// ZeroRange replaces the type
func (s Slice[T]) ZeroRange(start, end int) {
	fun.Invariant.OK(start >= 0 && end > start && end < len(s)-1,
		"start = ", start, "end = ", end, "are not valid bounds")
	for i := start; i <= end; i++ {
		var zero T
		s[i] = zero
	}

}

// FillTo appends zero values to the slice until it reaches the
// specified length.
func (s Slice[T]) FillTo(length int) Slice[T] {
	for len(s) < length {
		var val T
		s = append(s, val)
	}
	return s
}

// Ptrs converts a slice in to a slice of pointers to the values in
// the original slice
func (s Slice[T]) Ptrs() []*T {
	out := make([]*T, len(s))
	for idx := range s {
		out[idx] = &s[idx]
	}
	return out
}

// Process creates a future in the form of a work that, when called
// iterates through all items in the slice, returning when the
// processor errors. io.EOF errors are not returned, but do abort
// iteration early, while fun.ErrIteratorSkip is respected.
func (s Slice[T]) Process(pf fun.Processor[T]) fun.Worker {
	return func(ctx context.Context) error {
		for idx := range s {
			err := pf(ctx, s[idx])

			switch {
			case err == nil || errors.Is(err, fun.ErrIteratorSkip):
				continue
			case errors.Is(err, io.EOF):
				return nil
			default:
				return err
			}
		}
		return nil
	}
}

// Populate constructs an operation that adds all items from the
// iterator to the slice.
func (s *Slice[T]) Populate(iter *fun.Iterator[T]) fun.Operation {
	return func(ctx context.Context) {
		prod := iter.Producer()
		for {
			it, ok := prod.Check(ctx)
			if !ok {
				return
			}
			s.Append(it)
		}
	}
}
