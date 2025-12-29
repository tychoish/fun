// Package irt (for IteratoRTools), provides a collection of stateless iterator handling functions, with zero dependencies on other fun packages.
package irt

import (
	"cmp"
	"context"
	"errors"
	"iter"
	"maps"
	"slices"
)

func Collect[T any](seq iter.Seq[T], args ...int) (s []T) {
	switch len(args) {
	case 0:
	// pass, use the nil slice
	case 1:
		s = make([]T, idxorz(args, 0))
	case 2:
		fallthrough
	default:
		size := max(0, idxorz(args, 0))
		capacity := min(size, idxorz(args, 1))
		s = make([]T, size, capacity)
	}
	return slices.AppendSeq(s, seq)
}

func Collect2[K comparable, V any](seq iter.Seq2[K, V], args ...int) map[K]V {
	mp := make(map[K]V, max(0, idxorz(args, 0)))
	maps.Insert(mp, seq)
	return mp
}

func CollectFirstN[T any](seq iter.Seq[T], n int) []T {
	if n <= 0 {
		return make([]T, 0)
	}
	out := make([]T, 0, n)
	idx := 0
	for value := range seq {
		out = append(out, value)
		if idx+1 == n {
			break
		}
		idx++
	}

	return out
}

func JoinErrors(seq iter.Seq[error]) error { return errors.Join(Collect(seq)...) }

func One[T any](v T) iter.Seq[T]                          { return func(yield func(T) bool) { yield(v) } }
func Two[A, B any](a A, b B) iter.Seq2[A, B]              { return func(yield func(A, B) bool) { yield(a, b) } }
func Map[K comparable, V any](mp map[K]V) iter.Seq2[K, V] { return maps.All(mp) }
func Slice[T any](sl []T) iter.Seq[T]                     { return slices.Values(sl) }
func Args[T any](items ...T) iter.Seq[T]                  { return Slice(items) }

func Monotonic() iter.Seq[int]                       { return Generate(counter()) }
func MonotonicFrom(start int) iter.Seq[int]          { return Generate(counterFrom(start - 1)) }
func Range(start int, end int) iter.Seq[int]         { return While(MonotonicFrom(start), predLTE(end)) }
func Index[T any](seq iter.Seq[T]) iter.Seq2[int, T] { return Flip(WithEach(seq, counterFrom(-1))) }

func Flip[A, B any](seq iter.Seq2[A, B]) iter.Seq2[B, A] { return Convert2(seq, flip) }
func First[A, B any](seq iter.Seq2[A, B]) iter.Seq[A]    { return Merge(seq, first) }
func Second[A, B any](seq iter.Seq2[A, B]) iter.Seq[B]   { return Merge(seq, second) }

func Ptrs[T any](seq iter.Seq[T]) iter.Seq[*T]                { return Convert(seq, ptr) }
func PtrsWithNils[T comparable](seq iter.Seq[T]) iter.Seq[*T] { return Convert(seq, ptrznil) }
func Deref[T any](seq iter.Seq[*T]) iter.Seq[T]               { return Convert(RemoveNils(seq), derefz) }
func DerefWithZeros[T any](seq iter.Seq[*T]) iter.Seq[T]      { return Convert(seq, derefz) }

func FirstValue[T any](seq iter.Seq[T]) (zero T, ok bool) {
	for value := range seq {
		return value, true
	}
	return
}

func FirstValue2[A, B any](seq iter.Seq2[A, B]) (azero A, bzero B, ok bool) {
	for k, v := range seq {
		return k, v, true
	}
	return
}

func Limit[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		inc := counter()
		for elem := range seq {
			if !yield(elem) || inc() >= n {
				return
			}
		}
	}
}

func Limit2[A, B any](seq iter.Seq2[A, B], n int) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if n <= 0 {
			return
		}
		inc := counter()
		for k, v := range seq {
			if !yield(k, v) || inc() >= n {
				return
			}
		}
	}
}

func Convert[A, B any](seq iter.Seq[A], op func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		for value := range seq {
			if !yield(op(value)) {
				return
			}
		}
	}
}

func Convert2[A, B, C, D any](seq iter.Seq2[A, B], op func(A, B) (C, D)) iter.Seq2[C, D] {
	return func(yield func(C, D) bool) {
		for key, value := range seq {
			if !yield(op(key, value)) {
				return
			}
		}
	}
}

func Merge[A, B, C any](seq iter.Seq2[A, B], op func(A, B) C) iter.Seq[C] {
	return func(yield func(C) bool) {
		for key, value := range seq {
			if !yield(op(key, value)) {
				return
			}
		}
	}
}

func Generate[T any](op func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for yield(op()) {
			continue
		}
	}
}

func Generate2[A, B any](op func() (A, B)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for yield(op()) {
			continue
		}
	}
}

func With[A, B any](seq iter.Seq[A], op func(A) B) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for value := range seq {
			if !yield(value, op(value)) {
				return
			}
		}
	}
}

func With2[A, B, C any](seq iter.Seq[A], op func(A) (B, C)) iter.Seq2[B, C] {
	return func(yield func(B, C) bool) {
		for value := range seq {
			if !yield(op(value)) {
				return
			}
		}
	}
}

func WithEach[A, B any](seq iter.Seq[A], op func() B) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for value := range seq {
			if !yield(value, op()) {
				return
			}
		}
	}
}

func GenerateOk[T any](gen func() (T, bool)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for val, ok := gen(); ok && yield(val); val, ok = gen() {
			continue
		}
	}
}

func GenerateWhile[T any](op func() T, while func(T) bool) iter.Seq[T] {
	return While(Generate(op), while)
}

func GenerateOk2[A, B any](gen func() (A, B, bool)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for first, second, ok := gen(); ok && yield(first, second); first, second, ok = gen() {
			continue
		}
	}
}

func GenerateWhile2[A, B any](op func() (A, B), while func(A, B) bool) iter.Seq2[A, B] {
	return While2(Generate2(op), while)
}

func GenerateN[T any](num int, op func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < num && yield(op()); i++ {
			continue
		}
	}
}

func ForEach[T any](seq iter.Seq[T], op func(T)) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			op(value)
			if !yield(value) {
				return
			}
		}
	}
}

func ForEachWhile[T any](seq iter.Seq[T], op func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if !op(value) || !yield(value) {
				return
			}
		}
	}
}

func ForEach2[A, B any](seq iter.Seq2[A, B], op func(A, B)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			op(key, value)
			if !yield(key, value) {
				return
			}
		}
	}
}

func ForEachWhile2[A, B any](seq iter.Seq2[A, B], op func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			if !op(key, value) || !yield(key, value) {
				return
			}
		}
	}
}

func Apply[T any](seq iter.Seq[T], op func(T)) (count int) {
	for value := range seq {
		count++
		op(value)
	}
	return count
}

func ApplyWhile[T any](seq iter.Seq[T], op func(T) bool) (count int) {
	for value := range seq {
		count++
		if !op(value) {
			return count
		}
	}
	return count
}

func ApplyUntil[T any](seq iter.Seq[T], op func(T) error) error {
	for value := range seq {
		if err := op(value); err != nil {
			return err
		}
	}
	return nil
}

func ApplyUnless[T any](seq iter.Seq[T], op func(T) bool) int { return ApplyWhile(seq, notf(op)) }
func ApplyAll[T any](seq iter.Seq[T], op func(T) error) error { return JoinErrors(Convert(seq, op)) }

func ApplyAll2[A, B any](seq iter.Seq2[A, B], op func(A, B) error) error {
	return JoinErrors(Merge(seq, op))
}

func Apply2[A, B any](seq iter.Seq2[A, B], op func(A, B)) (count int) {
	for key, value := range seq {
		count++
		op(key, value)
	}
	return count
}

func ApplyWhile2[A, B any](seq iter.Seq2[A, B], op func(A, B) bool) (count int) {
	for key, value := range seq {
		count++
		if !op(key, value) {
			break
		}
	}
	return count
}

func ApplyUnless2[A, B any](seq iter.Seq2[A, B], op func(A, B) bool) int {
	return ApplyWhile2(seq, notf2(op))
}

func ApplyUntil2[A, B any](seq iter.Seq2[A, B], op func(A, B) error) error {
	for key, value := range seq {
		if err := op(key, value); err != nil {
			return err
		}
	}
	return nil
}

func Channel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) { loopWhile(func() bool { return yieldFrom(ctx, ch, yield) }) }
}

func Pipe[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	return opwithstart(opwithch(func(ch chan T) { seqToChan(ctx, seq, ch) }))
}

func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] {
	return func(yield func(iter.Seq[T]) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		for hasMore, gen := true, repeatok(num, next); num > 0 && hasMore && gen != nil; gen = repeatok(num, next) {
			if !yield(func(yield func(T) bool) {
				for value, ok := gen(); ok != nil; value, ok = gen() {
					if hasMore = deref(ok); !hasMore || !yield(value) {
						return
					}
				}
			}) {
				return
			}
		}
	}
}

func Chain[T any](seq iter.Seq[iter.Seq[T]]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for inner := range seq {
			for value := range inner {
				if !yield(value) {
					return
				}
			}
		}
	}
}

func ChainSlices[T any](seq iter.Seq[[]T]) iter.Seq[T]           { return Chain(Convert(seq, Slice)) }
func RemoveNils[T any](seq iter.Seq[*T]) iter.Seq[*T]            { return Remove(seq, isNil) }
func RemoveZeros[T comparable](seq iter.Seq[T]) iter.Seq[T]      { return Remove(seq, isZero) }
func RemoveErrors[T any](seq iter.Seq2[T, error]) iter.Seq[T]    { return First(Remove2(seq, isError2)) }
func KeepOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T]           { return First(Keep2(seq, isOk)) }
func WhileOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T]          { return First(While2(seq, isOk)) }
func WhileSuccess[T any](seq iter.Seq2[T, error]) iter.Seq[T]    { return First(While2(seq, isSuccess2)) }
func UntilNil[T any](seq iter.Seq[*T]) iter.Seq[T]               { return Deref(Until(seq, isNil)) }
func UntilError[T any](seq iter.Seq2[T, error]) iter.Seq[T]      { return First(Until2(seq, isError2)) }
func Until[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] { return While(seq, notf(prd)) }

func Until2[A, B any](seq iter.Seq2[A, B], is func(A, B) bool) iter.Seq2[A, B] {
	return While2(seq, notf2(is))
}

func While[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			switch {
			case prd(value) && yield(value):
				continue
			default:
				return
			}
		}
	}
}

func While2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			switch {
			case prd(key, value) && yield(key, value):
				continue
			default:
				return
			}
		}
	}
}

func Keep[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if prd(value) && !yield(value) {
				return
			}
		}
	}
}

func Keep2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for key, value := range seq {
			if prd(key, value) && !yield(key, value) {
				return
			}
		}
	}
}

func Shard[T any](ctx context.Context, num int, seq iter.Seq[T]) iter.Seq[iter.Seq[T]] {
	return GenerateOk(repeat(num, curry2(Channel, ctx, Pipe(ctx, seq))))
}

func WithHooks[T any](seq iter.Seq[T], before func(), after func()) iter.Seq[T] {
	return func(yield func(T) bool) { whenop(before); defer whenop(after); flush(seq, yield) }
}

func WithSetup[T any](seq iter.Seq[T], setup func()) iter.Seq[T] {
	setup = oncewhenop(setup)
	return func(yield func(T) bool) { whenop(setup); flush(seq, yield) }
}

func Remove[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] { return Keep(seq, notf(prd)) }
func Remove2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return Keep2(seq, notf2(prd))
}

func GroupBy[K comparable, V any](seq iter.Seq[V], groupBy func(V) K) iter.Seq2[K, []V] {
	grp := grouping(groups[K, V]{})
	Apply(seq, grp.with(groupBy))
	return grp.iter()
}

func Group[K comparable, V any](seq iter.Seq2[K, V]) iter.Seq2[K, []V] {
	grp := grouping(groups[K, V]{})
	Apply2(seq, grp.add)
	return grp.iter()
}

func Unique[T comparable](seq iter.Seq[T]) iter.Seq[T] { return Remove(seq, seen[T]()) }
func UniqueBy[K comparable, V any](seq iter.Seq[V], kfn func(V) K) iter.Seq[V] {
	return First(Remove2(With(seq, kfn), seenvalue[K, V]()))
}

func Count[T any](seq iter.Seq[T]) (size int) {
	inc := counter()
	Apply(seq, func(T) { size = inc() })
	return
}

func Count2[A, B any](seq iter.Seq2[A, B]) (size int) {
	inc := counter()
	Apply2(seq, func(A, B) { size = inc() })
	return
}

func Reduce[T any](seq iter.Seq[T], rfn func(T, T) T) (out T) {
	for v := range seq {
		out = rfn(out, v)
	}
	return out
}

func Contains[T comparable](seq iter.Seq[T], cmp T) (ok bool) {
	ApplyUnless(seq, func(in T) bool { ok = (cmp == in); return ok })
	return
}

func Equal[T comparable](rh iter.Seq[T], lh iter.Seq[T]) bool {
	rhNext, rhStop := iter.Pull(rh)
	defer rhStop()

	lhNext, lhStop := iter.Pull(lh)
	defer lhStop()

	for {
		rhv, okr := rhNext()
		lhv, okl := lhNext()

		switch {
		case okr != okl:
			return false
		case !okr && !okl:
			return true
		case rhv != lhv:
			return false
		}
	}
}

func Zip[A, B any](rh iter.Seq[A], lh iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		rhNext, rhStop := iter.Pull(rh)
		defer rhStop()

		lhNext, lhStop := iter.Pull(lh)
		defer lhStop()

		for {
			vr, okr := rhNext()
			if !okr {
				return
			}

			vl, okl := lhNext()
			if !okl {
				return
			}

			if !yield(vr, vl) {
				return
			}
		}
	}
}

func SortBy[K cmp.Ordered, T any](seq iter.Seq[T], cf func(T) K) iter.Seq[T] {
	return slices.Values(slices.SortedFunc(seq, toCmp(cf)))
}

func SortBy2[K cmp.Ordered, A, B any](seq iter.Seq2[A, B], cf func(A, B) K) iter.Seq2[A, B] {
	return ElemsSplit(Slice(slices.SortedFunc(Elems(seq), toCmp2(cf))))
}
