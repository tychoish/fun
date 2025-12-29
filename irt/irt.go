// Package irt (for IteratoRTools), provides a collection of stateless iterator handling functions, with zero dependencies on other fun packages.
package irt

import (
	"cmp"
	"context"
	"errors"
	"iter"
	"maps"
	"slices"
	"sync"
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
	// TODO(perf): compare with..
	//    GenerateWhile2(gen, isOk)

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
		if seq == nil {
			return
		}
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
		if seq == nil {
			return
		}
		for value := range seq {
			if !op(value) || !yield(value) {
				return
			}
		}
	}
}

func ForEach2[A, B any](seq iter.Seq2[A, B], op func(A, B)) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if seq == nil {
			return
		}
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
		if seq == nil {
			return
		}
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
	// TODO(perf): check performance against:2
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
		if prd != nil && seq != nil {
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
}

func While2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if prd != nil && seq != nil {
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
}

func Keep[T any](seq iter.Seq[T], prd func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		if prd != nil && seq != nil {
			for value := range seq {
				if prd(value) && !yield(value) {
					return
				}
			}
		}
	}
}

func Keep2[A, B any](seq iter.Seq2[A, B], prd func(A, B) bool) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		if prd != nil && seq != nil {
			for key, value := range seq {
				if prd(key, value) && !yield(key, value) {
					return
				}
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

// simple implementation methods

// aliases of stdlib

func keys[K comparable, V any, M ~map[K]V](in M) iter.Seq[K]   { return maps.Keys(in) }
func values[K comparable, V any, M ~map[K]V](in M) iter.Seq[V] { return maps.Values(in) }
func oncewhenop(op func()) func()                              { op = whendowith(op != nil, once, op); return op }
func once(op func()) func()                                    { return sync.OnceFunc(op) }
func oncev[T any](op func() T) func() T                        { return sync.OnceValue(op) }
func oncevs[A, B any](op func() (A, B)) func() (A, B)          { return sync.OnceValues(op) }
func cmpf[T cmp.Ordered](lh, rh T) int                         { return cmp.Compare(lh, rh) }
func with(mtx *sync.Mutex)                                     { mtx.Unlock() }
func lock(mtx *sync.Mutex) *sync.Mutex                         { mtx.Lock(); return mtx }

func loopWhile(op func() bool) {
	for op() {
		continue
	}
}

// statefull function utilities

func counter() func() int                             { return counterFrom(0) }
func counterFrom(next int) func() int                 { return func() int { next++; return next } }
func seen[T comparable]() func(T) bool                { s := set[T]{}; return s.add }
func seenkey[K comparable, V any]() func(K, V) bool   { return ignoreSecond[K, V](seen[K]()) }
func seenvalue[K comparable, V any]() func(V, K) bool { s := seen[K](); return ignoreFirst[V](s) }

func counterLT(n int) func() bool  { inc := counter(); return func() bool { return inc() < n } }
func counterLTE(n int) func() bool { inc := counter(); return func() bool { return inc() <= n } }

func yieldPre[T any](h func(), yield func(T) bool) func(T) bool {
	return func(in T) bool { h(); return yield(in) }
}

func yieldPost[T any](h func(), yield func(T) bool) func(T) bool {
	return func(in T) bool { defer h(); return yield(in) }
}

func yieldPreHook[T any](h func(T), yield func(T) bool) func(T) bool {
	return func(in T) bool { h(in); return yield(in) }
}

func yieldPostHook[T any](h func(T), yield func(T) bool) func(T) bool {
	return func(in T) bool { defer h(in); return yield(in) }
}

// function object wrappers and manipulators

func first[A, B any](a A, _ B) A     { return a }
func second[A, B any](_ A, b B) B    { return b }
func flip[A, B any](a A, b B) (B, A) { return b, a }
func noop[T any](in T) T             { return in }
func args[T any](args ...T) []T      { return args }

func flipfn[A, B any](op func(A, B) (A, B)) func(B, A) (B, A) {
	return func(b B, a A) (B, A) { return flip(op(a, b)) }
}

func whenop(op func())                             { whencall(op != nil, op) }
func whenopwith[T any](op func(T), arg T)          { whencallwith(op != nil, op, arg) }
func whenopdo[T any](op func() T) T                { return whendo(op != nil, op) }
func whenopdowith[A, B any](op func(A) B, arg A) B { return whendowith(op != nil, op, arg) }

func funcall[T any](op func(T), arg T)         { op(arg) }
func funcallOk[T any](op func(T), arg T) bool  { op(arg); return true }
func funcallv[T any](op func(...T), args ...T) { op(args...) }
func funcalls[T any](op func([]T), args []T)   { op(args) }
func funcallr[A, B any](op func(A) B, arg A) B { return op(arg) }

func ignoreSecond[A, B, C any](op func(A) C) func(A, B) C { return func(a A, _ B) C { return op(a) } }
func ignoreFirst[A, B, C any](op func(B) C) func(A, B) C  { return func(_ A, b B) C { return op(b) } }

func curry[A, B any](op func(A) B, a A) func() B             { return func() B { return op(a) } }
func curry2[A, B, C any](op func(A, B) C, a A, b B) func() C { return func() C { return op(a, b) } }
func wrap[A, B any](op func() A, fn func(A) B) func() (A, B) {
	return func() (A, B) { o := op(); return o, fn(o) }
}

func methodize[A, B, C any](r A, m func(A, B) C) func(B) C { return func(a B) C { return m(r, a) } }

func methodize1[A, B, C any](r A, m func(A, B) C) func(B) C {
	return func(a B) C { return m(r, a) }
}

func methodize2[A, B, C, D any](r A, m func(A, B) (C, D)) func(B) (C, D) {
	return func(a B) (C, D) { return m(r, a) }
}

func methodize3[A, B, C, D any](r A, m func(A, B, C) D) func(B, C) D {
	return func(b B, c C) D { return m(r, b, c) }
}

func methodize4[A, B, C, D, E any](r A, m func(A, B, C) (D, E)) func(B, C) (D, E) {
	return func(b B, c C) (D, E) { return m(r, b, c) }
}

func withctx[A, B any](ctx context.Context, op func(context.Context, A) B) func(A) B {
	return methodize(ctx, op)
}

func withctx1[A, B, C any](ctx context.Context, op func(context.Context, A, B) C) func(A, B) C {
	return methodize3(ctx, op)
}

func withctx2[A, B, C, D any](ctx context.Context, op func(context.Context, A, B) (C, D)) func(A, B) (C, D) {
	return methodize4(ctx, op)
}

func methodizethread[A, B, C, D, E any](r A, m func(A, B) (C, D), wrap func(C, D) E) func(B) E {
	return threadzip(methodize2(r, m), wrap)
}

func thread[A, B, C any](a func(A) B, b func(B) C) func(A) C { return func(v A) C { return b(a(v)) } }

func threadzip2[A, B, C, D any](op func(A, B) C, merge func(C) D) func(A, B) D {
	return func(a A, b B) D { return merge(op(a, b)) }
}

func threadzip[A, B, C, D any](op func(A) (B, C), merge func(B, C) D) func(A) D {
	return func(v A) D { return merge(op(v)) }
}

// predicates

func predLT[N cmp.Ordered](n N) func(N) bool  { return func(value N) bool { return value < n } }
func predGT[N cmp.Ordered](n N) func(N) bool  { return func(value N) bool { return value > n } }
func predEQ[N cmp.Ordered](n N) func(N) bool  { return func(value N) bool { return value == n } }
func predLTE[N cmp.Ordered](n N) func(N) bool { return func(value N) bool { return value <= n } }
func predGTE[N cmp.Ordered](n N) func(N) bool { return func(value N) bool { return value >= n } }

func equal[T comparable](lh, rh T) bool                  { return lh == rh }
func notf2[A, B any](op func(A, B) bool) func(A, B) bool { return threadzip2(op, not) }
func not(is bool) bool                                   { return !is }
func notf[T any](op func(T) bool) func(T) bool           { return thread(op, not) }

func isNil[T any](in *T) bool               { return in == nil }
func isNilChan[T any](in chan T) bool       { return in == nil }
func isZero[T comparable](in T) bool        { var zero T; return in == zero }
func isNotZero[T comparable](in T) bool     { return not(isZero(in)) }
func isOk[T any](_ T, ok bool) bool         { return ok }
func isError(err error) bool                { return err != nil }
func isError2[T any](_ T, err error) bool   { return err != nil }
func isSuccess(err error) bool              { return err == nil }
func isSuccess2[T any](_ T, err error) bool { return err == nil }

func withcheck[T any](v T, err error) (T, bool) { ok := isError(err); return ifelsedo(ok, v, zero), ok }

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

func zero[T any]() (zero T)                           { return zero }
func orDefault[T comparable](val T, defaultValue T) T { return ifelse(isZero(val), defaultValue, val) }
func orDefaultNew[T comparable](val T, op func() T) T { return ifdoelse(isZero(val), op, val) }

// slices -- access-by-index

func idxorzfn[E any, S ~[]E](sl S) func(int) E      { return methodize(sl, idxorz) }
func idxcheck[E any, S ~[]E](s S, idx int) bool     { return idx < 0 || idx >= len(s) }
func idx[E any, S ~[]E](sl S, idx int) E            { return sl[idx] }
func idxlazy[E any, S ~[]E](sl S, idx int) func() E { return func() E { return sl[idx] } }
func idxok[E any, S ~[]E](sl S, i int) (E, bool)    { return whendook(idxcheck(sl, i), idxlazy(sl, i)) }

func idxorz[E any, S ~[]E](sl S, idx int) E {
	// TODO: performance compare
	// ifdoelsedo(idx >= len(sl) || idx < 0, zero[E], idxlazy(sl, idx))

	if idxcheck(sl, idx) {
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
		if out = out || cond(); out {
			break
		}
	}
	return
}

func andf(conds ...func() bool) (out bool) {
	for _, cond := range conds {
		if out = out || !cond(); out {
			break
		}
	}
	return !out
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

func whendowith[A, B any](cond bool, then func(A) B, with A) (out B) {
	if cond {
		out = then(with)
	}
	return
}

func whencall(cond bool, then func()) {
	if cond {
		then()
	}
}

func whencallwith[T any](cond bool, then func(T), arg T) {
	if cond {
		then(arg)
	}
}

func whencallok[T any](cond bool, do func()) bool          { whencall(cond, do); return cond }
func whencallwithok[T any](c bool, do func(T), a T) bool   { whencallwith(c, do, a); return c }
func whencallfn(cond bool, do func()) func()               { return func() { whencall(cond, do) } }
func whencallwithfn[T any](c bool, do func(T), a T) func() { return func() { whencallwith(c, do, a) } }

func whendook[T any](cond bool, do func() T) (T, bool)           { return whendo(cond, do), cond }
func whendofn[T any](cond bool, do func() T) func() T            { return func() T { return whendo(cond, do) } }
func whendowithok[A, B any](c bool, do func(A) B, a A) (B, bool) { return whendowith(c, do, a), c }
func whendowithfn[A, B any](c bool, o func(A) B, a A) func() B   { return whendofn(c, curry(o, a)) }

// sorting helpers

func toCmp[T any, K cmp.Ordered](to func(T) K) func(T, T) int {
	return func(l, r T) int { return cmpf(to(l), to(r)) }
}

func toCmp2[A, B any, K cmp.Ordered](to func(A, B) K) func(Elem[A, B], Elem[A, B]) int {
	return func(l Elem[A, B], r Elem[A, B]) int { return cmp.Compare(to(l.First, l.Second), to(r.First, l.Second)) }
}

////////////////////////////////
//
// higher order operations

func flush[T any](seq iter.Seq[T], yield func(T) bool) bool {
	if seq == nil {
		return true
	}
	for value := range seq {
		if !yield(value) {
			return false
		}
	}
	return true
}

func repeat[T any](times int, op func() T) func() (T, bool) {
	return func() (zero T, _ bool) {
		if times <= 0 {
			return zero, false
		}
		times--
		return op(), true
	}
}

func repeat2[A, B any](limit int, op func() (A, B)) func() (A, B) {
	return func() (za A, zb B) {
		if limit <= 0 {
			return za, zb
		}
		limit--
		return op()
	}
}

func repeatok[T any](limit int, op func() (T, bool)) func() (T, *bool) {
	return func() (zero T, _ *bool) {
		switch {
		case limit <= 0:
			return zero, nil
		default:
			limit--
			out, ok := op()
			return out, ptr(ok)
		}
	}
}

// channel handling

func opwithstart[T any](ch chan T, op func()) chan T       { go op(); return ch }
func opwithclose[T any](ch chan T, op func(chan T)) func() { return func() { defer close(ch); op(ch) } }
func opwithch[T any](o func(chan T)) (chan T, func())      { c := make(chan T); return c, opwithclose(c, o) }

func seqToChan[T any](ctx context.Context, seq iter.Seq[T], ch chan T) { flush(seq, yieldTo(ctx, ch)) }

func yieldFrom[T any](ctx context.Context, ch <-chan T, yield func(T) bool) bool {
	select {
	case <-ctx.Done():
		return false
	case item, ok := <-ch:
		return ok && yield(item)
	}
}

func sendTo[T any](ctx context.Context, value T, ch chan T) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- value:
		return true
	}
}

func yieldTo[T any](ctx context.Context, ch chan T) func(T) bool {
	return func(v T) bool { return sendTo(ctx, v, ch) }
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

func (og *orderedGrouping[K, V]) add(k K, v V) {
	whencallwith(not(og.table.check(k)), og.append, k)
	og.table.add(k, v)
}

func (og *orderedGrouping[K, V]) with(f func(V) K) func(V) { return func(v V) { og.add(f(v), v) } }
func (og *orderedGrouping[K, V]) append(key K)             { og.order = append(og.order, key) }
func (og *orderedGrouping[K, V]) iter() iter.Seq2[K, []V]  { return With(Slice(og.order), og.table.pop) }

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

func WithElem[A, B any](a A, with func(A) B) Elem[A, B]             { return NewElem(a, with(a)) }
func NewElem[A, B any](a A, b B) Elem[A, B]                         { return Elem[A, B]{First: a, Second: b} }
func Elems[A, B any](seq iter.Seq2[A, B]) iter.Seq[Elem[A, B]]      { return Merge(seq, NewElem) }
func ElemsSplit[A, B any](seq iter.Seq[Elem[A, B]]) iter.Seq2[A, B] { return With2(seq, elemSplit) }
func ElemsApply[A, B any](seq iter.Seq[Elem[A, B]], op func(A, B))  { Apply(seq, elemApply(op)) }
func ElemCmp[A, B cmp.Ordered](lh, rh Elem[A, B]) int               { return lh.Compare(cmpf, cmpf).With(rh) }
func (e Elem[A, B]) Split() (A, B)                                  { return e.First, e.Second }
func (e Elem[A, B]) Apply(op func(A, B))                            { op(e.First, e.Second) }
func elemSplit[A, B any](in Elem[A, B]) (A, B)                      { return in.Split() }
func elemApply[A, B any](op func(A, B)) func(Elem[A, B])            { return func(e Elem[A, B]) { e.Apply(op) } }

func (e Elem[A, B]) Compare(aop func(A, A) int, bop func(B, B) int) interface{ With(Elem[A, B]) int } {
	return &elemcmp[A, B]{lh: e, ac: aop, bc: bop}
}

type elemcmp[A, B any] struct {
	lh Elem[A, B]
	ac func(A, A) int
	bc func(B, B) int
}

func (ec *elemcmp[A, B]) With(rh Elem[A, B]) int {
	return cmp.Compare(ec.ac(ec.lh.First, rh.First), ec.bc(ec.lh.Second, rh.Second))
}
