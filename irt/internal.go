package irt

import (
	"cmp"
	"context"
	"iter"
	"maps"
	"sync"
)

// aliases of stdlib

func keys[K comparable, V any, M ~map[K]V](in M) iter.Seq[K]   { return maps.Keys(in) }
func values[K comparable, V any, M ~map[K]V](in M) iter.Seq[V] { return maps.Values(in) }
func cmpf[T cmp.Ordered](lh, rh T) int                         { return cmp.Compare(lh, rh) }
func oncewhenop(op func()) func()                              { op = whendowith(op != nil, once, op); return op }
func once(op func()) func()                                    { return sync.OnceFunc(op) }
func oncev[T any](op func() T) func() T                        { return sync.OnceValue(op) }
func oncevs[A, B any](op func() (A, B)) func() (A, B)          { return sync.OnceValues(op) }
func oncego(op func()) func()                                  { return once(goop(op)) }
func goop(op func()) func()                                    { return func() { go op() } }
func with(mtx *sync.Mutex)                                     { mtx.Unlock() }
func lock(mtx *sync.Mutex) *sync.Mutex                         { mtx.Lock(); return mtx }
func mtxcall(mtx *sync.Mutex, op func()) func()                { return func() { defer with(lock(mtx)); op() } }
func toany[T any](in T) any                                    { return in }
func toany2[A, B any](first A, second B) (A, any)              { return first, any(second) }

func mtxdo[T any](mtx *sync.Mutex, op func() T) func() T {
	return func() T { defer with(lock(mtx)); return op() }
}

func mtxdo2[A, B any](m *sync.Mutex, o func() (A, B)) func() (A, B) {
	return func() (A, B) { defer with(lock(m)); return o() }
}

func mtxdo3[A, B, C any](m *sync.Mutex, o func() (A, B, C)) func() (A, B, C) {
	return func() (A, B, C) { defer with(lock(m)); return o() }
}

func mtxcallwith[T any](mtx *sync.Mutex, op func(T)) func(T) {
	return func(arg T) { defer with(lock(mtx)); op(arg) }
}

func mtxdowith[T any](mtx *sync.Mutex, op func(T) T) func(T) T {
	return func(arg T) T { defer with(lock(mtx)); return op(arg) }
}

func loopWhile(op func() bool) {
	for op() {
		continue
	}
}

func mapPop[K comparable, V any](mp map[K]V, k K) (V, bool) {
	v, ok := mp[k]
	delete(mp, k)
	return v, ok
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
func funcallok[T any](op func(T), arg T) bool  { op(arg); return true }
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
func isWithin(index, length int) bool       { return index >= 0 && index < length }

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

func idx[E any, S ~[]E](sl S, idx int) E          { return sl[idx] }
func idxfn[E any, S ~[]E](sl S, idx int) func() E { return func() E { return sl[idx] } }
func idxcheck[E any, S ~[]E](s S, idx int) bool   { return isWithin(idx, len(s)) }
func idxorz[E any, S ~[]E](sl S, idx int) E       { return first(idxok(sl, idx)) }
func idxorzfn[E any, S ~[]E](sl S) func(int) E    { return methodize(sl, idxorz) }

func idxok[E any, S ~[]E](sl S, i int) (out E, ok bool) {
	if ok = isWithin(i, len(sl)); ok {
		out = sl[i]
	}
	return
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

func whencallok(cond bool, do func()) bool                 { whencall(cond, do); return cond }
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

func toCmp2[A, B any, K cmp.Ordered](to func(A, B) K) func(KV[A, B], KV[A, B]) int {
	return func(l KV[A, B], r KV[A, B]) int { return cmp.Compare(to(l.Key, l.Value), to(r.Key, l.Value)) }
}

////////////////////////////////
//
// higher order operations

func repeat[T any](times int, op func() T) func() (T, bool) {
	return func() (out T, ok bool) {
		if times > 0 {
			out, ok = op(), true
			times--
		}
		return
	}
}

func repeat2[A, B any](limit int, op func() (A, B)) func() (A, B) {
	return func() (a A, b B) {
		if limit > 0 {
			a, b = op()
			limit--
		}
		return
	}
}

func repeatok[T any](limit int, op func() (T, bool)) func() (T, *bool) {
	return func() (zero T, _ *bool) {
		if limit > 0 {
			limit--
			v, ok := op()
			return v, ptr(ok)
		}
		return
	}
}

// channel handling

func opwithstart[T any](ch chan T, op func()) chan T       { go op(); return ch }
func opwithclose[T any](ch chan T, op func(chan T)) func() { return func() { defer close(ch); op(ch) } }
func opwithch[T any](o func(chan T)) (chan T, func())      { c := make(chan T); return c, opwithclose(c, o) }

func seqToChan[T any](ctx context.Context, seq iter.Seq[T], ch chan T) { flush(seq, yieldTo(ctx, ch)) }

func yieldTo[T any](ctx context.Context, ch chan T) func(T) bool {
	return func(v T) bool { return ctx.Err() == nil && sendTo(ctx, v, ch) }
}

func yieldFrom[T any](ctx context.Context, ch <-chan T, yield func(T) bool) bool {
	if ctx.Err() == nil {
		item, ok := recieveFrom(ctx, ch)
		return ok && yield(item)
	}
	return false
}

func recieveFrom[T any](ctx context.Context, ch <-chan T) (out T, ok bool) {
	select {
	case <-ctx.Done():
	case out, ok = <-ch:
	}
	return
}

func sendTo[T any](ctx context.Context, value T, ch chan<- T) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- value:
		return true
	}
}

// higher order iterator helpers

func flushTo[T any](ctx context.Context, seq iter.Seq[T], ch chan<- T) bool {
	for item := range seq {
		if !sendTo(ctx, item, ch) {
			return false
		}
	}

	return true
}

func flush[T any](seq iter.Seq[T], yield func(T) bool) bool {
	for value := range seq {
		if !yield(value) {
			return false
		}
	}
	return true
}

func flush2[A, B any](seq iter.Seq2[A, B], yield func(A, B) bool) bool {
	for key, value := range seq {
		if !yield(key, value) {
			return false
		}
	}
	return true
}

// unpull converts next and stop functions back into an iter.Seq.
// This is the inverse of iter.Pull.
func unpull[T any](next func() (T, bool), stop func()) iter.Seq[T] {
	return func(yield func(T) bool) {
		defer stop()
		for value, ok := next(); ok && yield(value); value, ok = next() {
			continue
		}
	}
}

// unpull2 converts next and stop functions back into an iter.Seq2.
// This is the inverse of iter.Pull2.
func unpull2[A, B any](next func() (A, B, bool), stop func()) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		defer stop()
		for key, value, ok := next(); ok && yield(key, value); key, value, ok = next() {
			continue
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

func (og *orderedGrouping[K, V]) add(k K, v V) {
	whencallwith(not(og.table.check(k)), og.append, k)
	og.table.add(k, v)
}

func (og *orderedGrouping[K, V]) with(f func(V) K) func(V) { return func(v V) { og.add(f(v), v) } }
func (og *orderedGrouping[K, V]) append(key K)             { og.order = append(og.order, key) }
func (og *orderedGrouping[K, V]) iter() iter.Seq2[K, []V]  { return With(Slice(og.order), og.table.pop) }
