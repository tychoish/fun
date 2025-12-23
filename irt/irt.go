// Package irt (for IteratoRTools), provides a collection of stateless iterator handling functions, with zero dependencies on other fun packages.
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

	switch idx {
	case n:
		return out
	case 0:
		return out[:0]
	default:
		return out[:idx-1]
	}
}

func One[T any](v T) iter.Seq[T]                          { return func(yield func(T) bool) { yield(v) } }
func Two[A, B any](a A, b B) iter.Seq2[A, B]              { return func(yield func(A, B) bool) { yield(a, b) } }
func Map[K comparable, V any](mp map[K]V) iter.Seq2[K, V] { return maps.All(mp) }
func Slice[T any](sl []T) iter.Seq[T]                     { return slices.Values(sl) }
func Args[T any](items ...T) iter.Seq[T]                  { return Slice(items) }

func Index[T any](seq iter.Seq[T]) iter.Seq2[int, T] { return Flip(With(seq, wrap[T](counter()))) }
func JoinErrors(seq iter.Seq[error]) error           { return errors.Join(slices.Collect(seq)...) }

func Flip[A, B any](seq iter.Seq2[A, B]) iter.Seq2[B, A] { return Convert2(seq, flip) }
func First[A, B any](seq iter.Seq2[A, B]) iter.Seq[A]    { return Merge(seq, first) }
func Second[A, B any](seq iter.Seq2[A, B]) iter.Seq[B]   { return Merge(seq, second) }

func Ptrs[T any](seq iter.Seq[T]) iter.Seq[*T]                { return Convert(seq, ptr) }
func PtrsWithNils[T comparable](seq iter.Seq[T]) iter.Seq[*T] { return Convert(seq, ptrznil) }
func Deref[T any](seq iter.Seq[*T]) iter.Seq[T]               { return Convert(RemoveNils(seq), derefz) }
func DerefWithZeros[T any](seq iter.Seq[*T]) iter.Seq[T]      { return Convert(seq, derefz) }

func Generate[T any](gen func() (T, bool)) iter.Seq[T] {
	// TODO(perf): compare with..
	//    GenerateWhile2(gen, isOk)

	return func(yield func(T) bool) {
		for val, ok := gen(); ok && yield(val); val, ok = gen() {
			continue
		}
	}
}

func Generate2[A, B any](gen func() (A, B, bool)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		for first, second, ok := gen(); ok && yield(first, second); first, second, ok = gen() {
			continue
		}
	}
}

func GenerateWhile[T any](op func() T, while func(T) bool) iter.Seq[T] {
	return While(Perpetual(op), while)
}

func GenerateWhile2[A, B any](op func() (A, B), while func(A, B) bool) iter.Seq2[A, B] {
	return While2(Perpetual2(op), while)
}

func Perpetual[T any](op func() T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for yield(op()) {
			continue
		}
	}
}

func Perpetual2[A, B any](op func() (A, B)) iter.Seq2[A, B] {
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

func Apply2[A, B any](seq iter.Seq2[A, B], op func(A, B)) (count int) {
	// TODO(perf): check performance against:
	//    return Apply(Elems(seq), elemApply(op))

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

func ApplyAll12[A, B any](seq iter.Seq2[A, B], op func(A, B) error) error {
	return JoinErrors(Merge(seq, op))
}

func Channel[T any](ctx context.Context, ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case value, ok := <-ch:
				if !ok || !yield(value) {
					return
				}
			}
		}
	}
}

func Pipe[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	ch := make(chan T)

	go func() {
		defer close(ch)
		for value := range seq {
			select {
			case <-ctx.Done():
				return
			case ch <- value:
				continue
			}
		}
	}()

	return ch
}

func Chunk[T any](seq iter.Seq[T], num int) iter.Seq[iter.Seq[T]] {
	return func(yield func(iter.Seq[T]) bool) {
		next, stop := iter.Pull(seq)
		defer stop()

		for hasMore := true; hasMore; {
			gen := withlimit(next, num)
			if !yield(func(yield func(T) bool) {
				for value, okp := gen(); okp != nil; value, okp = gen() {
					if !deref(okp) {
						hasMore = false
						return
					}
					if !yield(value) {
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
func RemoveErrors[T any](seq iter.Seq2[T, error]) iter.Seq[T]    { return KeepOk(Convert2(seq, checkErr)) }
func KeepOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T]           { return First(Keep2(seq, isOk)) }
func WhileOk[T any](seq iter.Seq2[T, bool]) iter.Seq[T]          { return First(While2(seq, isOk)) }
func WhileSuccess[T any](seq iter.Seq2[T, error]) iter.Seq[T]    { return First(While2(seq, isSuccess2)) }
func UntilNil[T any](seq iter.Seq[*T]) iter.Seq[T]               { return Deref(Until(seq, isNil)) }
func UntilError[T any](seq iter.Seq2[T, error]) iter.Seq[T]      { return First(Until2(seq, isError2)) }
func Until[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] { return While(seq, notf(prd)) }

func Until2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return While2(seq, notf2(prd))
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
	return First(Remove2(With(seq, kfn), seenSecond[K, V]()))
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

			defer lhStop()

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

func flush[T any](seq iter.Seq[T], yield func(T) bool) bool {
	for value := range seq {
		if !yield(value) {
			return false
		}
	}
	return true
}

// simple implementation methods

// aliases of stdlib

func keys[K comparable, V any, M ~map[K]V](in M) iter.Seq[K]   { return maps.Keys(in) }
func values[K comparable, V any, M ~map[K]V](in M) iter.Seq[V] { return maps.Values(in) }

// statefull function utilities

func counter() func() int                              { var count int; return func() int { count++; return count } }
func seen[T comparable]() func(T) bool                 { s := set[T]{}; return s.add }
func equal[T comparable](lh, rh T) bool                { return lh == rh }
func seenFirst[K comparable, V any]() func(K, V) bool  { s := seen[K](); return withFirst[K, V](s) }
func seenSecond[K comparable, V any]() func(V, K) bool { s := seen[K](); return withSecond[V](s) }

// function object wrappers and manipulators

func first[A, B any](a A, _ B) A           { return a }
func second[A, B any](_ A, b B) B          { return b }
func flip[A, B any](a A, b B) (B, A)       { return b, a }
func noop[T any](in T) T                   { return in }
func wrap[A, B any](op func() B) func(A) B { return func(A) B { return op() } }
func args[T any](args ...T) []T            { return args }

func wrap2[A, B any](op func() A, wrap func(A) B) func() (A, B) {
	return func() (A, B) { o := op(); return o, wrap(o) }
}

func funcall[T any](op func(T), arg T)         { op(arg) }
func funcallv[T any](op func(...T), args ...T) { op(args...) }
func funcalls[T any](op func([]T), args []T)   { op(args) }
func funcallr[A, B any](op func(A) B, arg A) B { return op(arg) }

func withFirst[A, B, C any](op func(A) C) func(A, B) C  { return func(a A, _ B) C { return op(a) } }
func withSecond[A, B, C any](op func(B) C) func(A, B) C { return func(_ A, b B) C { return op(b) } }
func withSeen[K comparable, V any]() func(K, V) bool    { return withFirst[K, V](seen[K]()) }

func withFlip[A, B any](op func(A, B) (A, B)) func(B, A) (B, A) {
	return func(b B, a A) (B, A) { return flip(op(a, b)) }
}

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

// predicates

func not(is bool) bool                         { return !is }
func notf[T any](op func(T) bool) func(T) bool { return func(in T) bool { return not(op(in)) } }
func notf2[A, B any](op func(A, B) bool) func(A, B) bool {
	return func(a A, b B) bool { return !op(a, b) }
}

func isNil[T any](in *T) bool                  { return in == nil }
func isZero[T comparable](in T) bool           { var zero T; return in == zero }
func isNonZero[T comparable](in T) bool        { return not(isZero(in)) }
func isOk[T any](_ T, ok bool) bool            { return ok }
func isError(err error) bool                   { return err != nil }
func isError2[T any](_ T, err error) bool      { return err != nil }
func isSuccess(err error) bool                 { return err == nil }
func isSuccess2[T any](_ T, err error) bool    { return err == nil }
func zero[T any]() (zero T)                    { return zero }
func zerofor[T any](T) (zero T)                { return zero }
func checkErr[T any](v T, err error) (T, bool) { ok := isError(err); return ifelsedo(ok, v, zero), ok }
func withCheckErr[T any]() func(T, error) bool { return func(_ T, e error) bool { return e != nil } }

// pointers and references

func ptr[T any](in T) *T                       { return &in }
func ptrznil[T comparable](in T) *T            { return ifelsedo(isZero(in), nil, ptrlazy(in)) }
func ptrznillazy[T comparable](in T) func() *T { return func() *T { return ptrznil((in)) } }
func ptrlazy[T any](in T) func() *T            { return func() *T { return ptr(in) } }
func deref[T any](in *T) T                     { return *in }
func dereflazy[T any](in *T) func() T          { return func() T { return deref(in) } }
func derefzlazy[T any](in *T) func() T         { return func() T { return derefz(in) } }
func derefz[T any](in *T) T                    { return ifdoelsedo(isNil(in), zero[T], dereflazy(in)) }

// default constructors

func orDefault[T comparable](val T, defaultValue T) T { return ifelse(isZero(val), defaultValue, val) }
func orDefaultNew[T comparable](val T, op func() T) T { return ifdoelse(isZero(val), op, val) }

// slices and access-by-index

func idx[E any, S ~[]E](idx int, sl S) E            { return sl[idx] }
func idxlazy[E any, S ~[]E](idx int, sl S) func() E { return func() E { return sl[idx] } }

func idxorz[E any, S ~[]E](idx int, sl S) E {
	// TODO: performance compare
	// ifdoelsedo(len(sl) >= idx, zero[T], idxlazy(idx, sl))

	if len(sl)-1 < idx {
		return zero[E]()
	}
	return sl[idx]
}

////////////////////////////////
//
// Conditionals

func orv(rh, lh bool) bool  { return rh || lh }
func andv(rh, lh bool) bool { return rh && lh }

func orf(conds ...func() bool) (out bool) {
	for _, cond := range conds {
		out = out || cond()
		if out {
			break
		}
	}
	return
}

func andf(conds ...func() bool) (out bool) {
	out = true
	for _, cond := range conds {
		out = out && cond()
		if out {
			break
		}
	}
	return
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

////////////////////////////////
//
// higher order operations

func ntimes[T any](op func() T, times int) func() (T, bool) {
	var count int
	return func() (zero T, _ bool) {
		if count >= times {
			return zero, false
		}
		count++
		return op(), true
	}
}

func withlimit[T any](op func() (T, bool), limit int) func() (T, *bool) {
	if limit <= 0 {
		panic("limit must be greater than zero")
	}

	var count int
	var exhausted bool

	return func() (zero T, _ *bool) {
		switch {
		case exhausted:
			return zero, ptr(false)
		case count >= limit:
			return zero, nil
		default:
			out, ok := op()
			if !ok {
				exhausted = true
			}
			count++
			return out, ptr(ok)
		}
	}
}

////////////////////////////////
//
// Types

type none struct{}

type set[T comparable] map[T]none

func (s set[T]) check(v T) bool         { _, ok := s[v]; return ok }
func (s set[T]) set(v T)                { s[v] = none{} }
func (s set[T]) add(v T) (existed bool) { existed = s.check(v); s.set(v); return existed }
func (s set[T]) iter() iter.Seq[T]      { return maps.Keys(s) }
func (s set[T]) pop(v T) (existed bool) { existed = s.check(v); delete(s, v); return existed }

type groups[K comparable, V any] map[K][]V

func (mp groups[K, V]) check(key K) bool   { _, ok := mp[key]; return ok }
func (mp groups[K, V]) add(key K, value V) { mp[key] = append(mp[key], value) }
func (mp groups[K, V]) pop(key K) []V      { out := mp[key]; delete(mp, key); return out }

type orderedGrouping[K comparable, V any] struct {
	table groups[K, V]
	order []K
}

func grouping[K comparable, V any](mp groups[K, V]) *orderedGrouping[K, V] {
	return &orderedGrouping[K, V]{table: mp, order: []K{}}
}

func (og *orderedGrouping[K, V]) add(key K, value V) {
	whencall(not(og.table.check(key)), og.append, key)
	og.table.add(key, value)
}

func (og *orderedGrouping[K, V]) with(fn func(V) K) func(V) {
	return func(value V) { og.add(fn(value), value) }
}

func (og *orderedGrouping[K, V]) append(key K)            { og.order = append(og.order, key) }
func (og *orderedGrouping[K, V]) iter() iter.Seq2[K, []V] { return With(Slice(og.order), og.table.pop) }

func mapPop[K comparable, V any](mp map[K]V, k K) (V, bool) {
	v, ok := mp[k]
	delete(mp, k)
	return v, ok
}

// Tuple wrapper type

type Elem[A, B any] struct {
	First  A
	Second B
}

func WithElem[A, B any](a A, with func(A) B) Elem[A, B]         { return NewElem(a, with(a)) }
func NewElem[A, B any](a A, b B) Elem[A, B]                     { return Elem[A, B]{First: a, Second: b} }
func Elems[A, B any](seq iter.Seq2[A, B]) iter.Seq[Elem[A, B]]  { return Merge(seq, NewElem) }
func Splits[A, B any](seq iter.Seq[Elem[A, B]]) iter.Seq2[A, B] { return With2(seq, elemSplit[A, B]) }
func (e Elem[A, B]) Split() (A, B)                              { return e.First, e.Second }
func (e Elem[A, B]) Apply(op func(A, B))                        { op(e.First, e.Second) }
func elemSplit[A, B any](in Elem[A, B]) (A, B)                  { return in.Split() }
func elemApply[A, B any](op func(A, B)) func(Elem[A, B])        { return func(e Elem[A, B]) { e.Apply(op) } }
