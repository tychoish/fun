package strut

import (
	"iter"
	"slices"
)

func ifop(cond bool, op func()) {
	if cond {
		op()
	}
}

func ifargs[T any](cond bool, op func(...T), args []T) {
	if cond {
		op(args...)
	}
}

func iftuple[A, B any](cond bool, op func(A, B), a A, b B) {
	if cond {
		op(a, b)
	}
}

func iffmt[T any](cond bool, op func(string, ...T), tmpl string, args []T) {
	if cond {
		op(tmpl, args...)
	}
}

func ifwith[T any](cond bool, op func(T), n T) {
	if cond {
		op(n)
	}
}

func ntimes(n int, op func()) {
	for range n {
		op()
	}
}

func nwith[T any](n int, op func(T), arg T) {
	for range n {
		op(arg)
	}
}

func apply[T any](op func(T), vals []T) { flush(slices.Values(vals), op) }

func flush[T any](seq iter.Seq[T], op func(T)) {
	for str := range seq {
		op(str)
	}
}

// FromMutable provides a clear, legible operation to convert iterators of Mutable instances to iterators of strings.
func FromMutable(seq iter.Seq[Mutable]) iter.Seq[string] {
	return func(yield func(string) bool) {
		for mut := range seq {
			if !yield(string(mut)) {
				return
			}
		}
	}
}
