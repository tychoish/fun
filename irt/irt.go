// Package irt, for iterator tools, provides a collection of stateless iterator handling functions, with zero dependencies on other fun packages.
package irt

import (
	"context"
	"errors"
	"iter"
	"maps"
	"slices"
)

func Collect[T any](seq iter.Seq[T], args ...int) []T {
	return slices.AppendSeq(make([]T, idxorz(0, args), idxorz(1, args)), seq)
}

func Collect2[K comparable, V any](seq iter.Seq2[K, V], args ...int) map[K]V {
	mp := make(map[K]V, idxorz(0, args))
	maps.Insert(mp, seq)
	return mp
}

func CollectFirstN[T any](seq iter.Seq[T], n int) []T {
	out := make([]T, n)
	idx := 0
	for value := range seq {
		out[idx] = value
		if idx+1 == n {
			break
		}
		idx++
	}
	return out
}

func Slice[T any](sl []T) iter.Seq[T]                     { return slices.Values(sl) }
func Args[T any](items ...T) iter.Seq[T]                  { return Slice(items) }
func Map[K comparable, V any](mp map[K]V) iter.Seq2[K, V] { return maps.All(mp) }

func WithIndex[T any](seq iter.Seq[T]) iter.Seq2[int, T] { return Flip(Zip(seq, wrap[T](counter()))) }
func JoinErrors(seq iter.Seq[error]) error               { return errors.Join(slices.Collect(seq)...) }

func Flip[A, B any](seq iter.Seq2[A, B]) iter.Seq2[B, A] { return nil }
func First[A, B any](seq iter.Seq2[A, B]) iter.Seq[A]    { return nil }
func Second[A, B any](seq iter.Seq2[A, B]) iter.Seq[B]   { return nil }

func Zip[A, B any](seq iter.Seq[A], op func(A) B) iter.Seq2[A, B]                        { return nil }
func Convert[A, B any](seq iter.Seq[A], op func(A) B) iter.Seq[B]                        { return nil }
func Convert2[A, B, C, D any](seq iter.Seq2[A, B], op func(A, B) (C, D)) iter.Seq2[C, D] { return nil }
func Merge[A, B, C any](seq iter.Seq2[A, B], op func(A, B) C) iter.Seq[C]                { return nil }

func Apply[T any](seq iter.Seq[T], op func(T)) int              { return -1 }
func ApplyWhile[T any](seq iter.Seq[T], op func(T) bool) int    { return -1 }
func ApplyUnless[T any](seq iter.Seq[T], op func(T) bool) int   { return -1 }
func ApplyUntil[T any](seq iter.Seq[T], op func(T) error) error { return nil }
func ApplyAll[T any](seq iter.Seq[T], op func(T) error) error   { return nil }

func Apply2[A, B any](seq iter.Seq2[A, B], op func(A, B)) int              { return -1 }
func ApplyWhile2[A, B any](seq iter.Seq2[A, B], op func(A, B) bool) int    { return -1 }
func ApplyUnless2[A, B any](seq iter.Seq2[A, B], op func(A, B) bool) int   { return -1 }
func ApplyUntil2[A, B any](seq iter.Seq2[A, B], op func(A, B) error) error { return nil }
func ApplyAll12[A, B any](seq iter.Seq2[A, B], op func(A, B) error) error  { return nil }

func Channel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] { return nil }
func Pipe[T any](ctx context.Context, seq iter.Seq[T]) <-chan T   { return nil }

func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] { return nil }
func Chain[T any](seq iter.Seq[iter.Seq[T]]) iter.Seq[T]          { return nil }
func ChainSlices[T any](seq iter.Seq[[]T]) iter.Seq[T]            { return Chain(Convert(seq, slices.Values)) }

func Keep[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T]                  { return nil }
func Keep2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B]   { return nil }
func Remove[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T]                { return nil }
func Remove2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] { return nil }

func GroupBy[K comparable, V any](input iter.Seq[V], groupBy func(V) K) iter.Seq2[K, []V] { return nil }
func Group[K comparable, V any](seq iter.Seq2[K, V]) iter.Seq2[K, []V]                    { return nil }
func Unique[T comparable](seq iter.Seq[T]) iter.Seq[T]                                    { return nil }
func UniqueBy[K comparable, V any](seq iter.Seq[V], kfn func(V) K) iter.Seq[V]            { return nil }

func Reduce[T any](seq iter.Seq[T], rfn func(T, T) T) (out T) {
	for v := range seq {
		out = rfn(out, v)
	}
	return out
}

func Contains[T comparable](seq iter.Seq[T], cmp T) (ok bool) {
	ApplyWhile(seq, func(in T) bool { ok = (cmp == in); return !ok })
	return
}

func Equal[T comparable](rhs iter.Seq[T], lhs iter.Seq[T]) bool {
	rhNext, rhStop := iter.Pull(rhs)
	defer rhStop()
	lhNext, lhStop := iter.Pull(lhs)
	defer lhStop()
	for {
		rhv, okr := rhNext()
		lhv, okl := lhNext()
		if okr != okl {
			return false
		}
		if !okr && !okl {
			return true
		}
		if rhv != lhv {
			return false
		}
	}
}

// Tuple wrapper type

type Elem[A, B any] struct {
	First  A
	Second B
}

func NewElem[A, B any](a A, b B) Elem[A, B]                    { return Elem[A, B]{First: a, Second: b} }
func Elems[A, B any](seq iter.Seq2[A, B]) iter.Seq[Elem[A, B]] { return Merge(seq, NewElem) }

// simple implementation methods

func counter() func() int               { var count int; return func() int { count++; return count } }
func seen[T comparable]() func(T) bool  { s := set[T]{}; return s.add }
func equal[T comparable](lh, rh T) bool { return lh == rh }

func first[A any, B any](a A, _ B) A           { return a }
func second[A any, B any](_ A, b B) B          { return b }
func wrap[A any, B any](op func() B) func(A) B { return func(A) B { return op() } }
func noop[T any](in T) T                       { return in }

func methodize[A, B, C, D any](self A, op func(A, B) (C, D)) func(B) (C, D) {
	return func(in B) (C, D) { return op(self, in) }
}

func methodizethread[A, B, C, D, E any](self A, op func(A, B) (C, D), outer func(C, D) E) func(B) E {
	return zipthread(methodize(self, op), outer)
}

func thread[A, B, C any](a func(A) B, b func(B) C) func(A) C { return func(v A) C { return b(a(v)) } }
func zipthread[A, B, C, D any](op func(A) (B, C), merge func(B, C) D) func(A) D {
	return func(v A) D { return merge(op(v)) }
}

func not(is bool) bool                           { return !is }
func negate[T any](op func(T) bool) func(T) bool { return func(in T) bool { return not(op(in)) } }

func isNil[T any](in *T) bool        { return in == nil }
func isZero[T comparable](in T) bool { var zero T; return in == zero }
func zero[T any]() (zero T)          { return zero }
func zerofor[T any](T) (zero T)      { return zero }

func ptr[T any](in T) *T              { return &in }
func ptrlazy[T any](in T) func() *T   { return func() *T { return ptr(in) } }
func deref[T any](in *T) T            { return *in }
func dereflazy[T any](in *T) func() T { return func() T { return deref(in) } }
func derefzero[T any](in *T) T        { return ifdoelsedo(isNil(in), zero[T], dereflazy(in)) }

func idx[E any, S ~[]E](idx int, sl S) E            { return sl[idx] }
func idxlazy[E any, S ~[]E](idx int, sl S) func() E { return func() E { return sl[idx] } }

func funcall[T any](op func(T), arg T)         { op(arg) }
func funcallv[T any](op func([]T), args ...T)  { op(args) }
func funcallr[A, B any](op func(A) B, arg A) B { return op(arg) }

func orDefault[T comparable](val T, defaultValue T) T { return ifelse(isZero(val), defaultValue, val) }
func orDefaultNew[T comparable](val T, op func() T) T { return ifdoelse(isZero(val), op, val) }

func idxorz[E any, S ~[]E](idx int, sl S) E {
	// TODO: performance compare
	// ifdoelsedo(len(sl) >= idx, zero[T], idxlazy(idx, sl))

	if len(sl)-1 < idx {
		return zero[E]()
	}
	return sl[idx]
}

func ifelse[T any](cond bool, then T, elsewise T) T {
	if cond {
		return then
	}
	return elsewise
}

func ifelsedo[T any](cond bool, then T, elsewise func() T) T {
	if cond {
		return then
	}
	return elsewise()
}

func ifdoelse[T any](cond bool, then func() T, elsewise T) T {
	if cond {
		return then()
	}
	return elsewise
}

func ifdoelsedo[T any](cond bool, then func() T, elsewise func() T) T {
	if cond {
		return then()
	}
	return elsewise()
}

func ifcallelsecall[T any](cond bool, then func(), elsewise func()) {
	if cond {
		then()
	} else {
		elsewise()
	}
}

func whendo[T any](cond bool, then func() T) (out T) {
	if cond {
		out = then()
	}
	return
}

func whencall[T any](cond bool, then func(T), arg T) {
	if cond {
		then(arg)
	}
}

type none struct{}

type set[T comparable] map[T]none

func (s set[T]) check(v T) bool         { _, ok := s[v]; return ok }
func (s set[T]) set(v T)                { s[v] = none{} }
func (s set[T]) add(v T) (existed bool) { existed = s.check(v); s.set(v); return }
func (s set[T]) iter() iter.Seq[T]      { return maps.Keys(s) }
