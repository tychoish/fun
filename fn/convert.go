package fn

import (
	"sync"

	"github.com/tychoish/fun/ft"
	prv "github.com/tychoish/fun/internal"
)

type Filter[T any] Converter[T, T]

func MakeFilter[T any](fl func(T) T) Filter[T]         { return fl }
func ctf[T any](in Converter[T, T]) Filter[T]          { return MakeFilter(in) }
func (fl Filter[T]) c() Converter[T, T]                { return MakeConverter(fl) }
func (fl Filter[T]) Apply(v T) T                       { return fl(v) }
func (fl Filter[T]) Ptr(v *T)                          { *v = fl(*v) }
func (fl Filter[T]) Safe() Filter[T]                   { return func(v T) T { return ft.FilterSafe(fl, v) } }
func (fl Filter[T]) WithNext(next Filter[T]) Filter[T] { return func(v T) T { return next(fl(v)) } }
func (fl Filter[T]) If(cond bool) Filter[T]            { return func(v T) T { return ft.FilterWhen(cond, fl, v) } }
func (fl Filter[T]) Not(cond bool) Filter[T]           { return fl.If(ft.Not(cond)) }
func (fl Filter[T]) Lock() Filter[T]                   { return fl.WithLock(&sync.Mutex{}) }
func (fl Filter[T]) WithLock(mu *sync.Mutex) Filter[T] {
	return func(v T) T { defer prv.With(prv.Lock(mu)); return fl.Apply(v) }
}
func (fl Filter[T]) PreHook(op func()) Filter[T] { return ctf(fl.c().PreHook(op)) }

func (fl Filter[T]) PostHook(op func()) Filter[T] { return ctf(fl.c().PostHook(op)) }

func (fl Filter[T]) Join(fls ...Filter[T]) Filter[T] {
	return func(v T) T {
		for _, iterFl := range append([]Filter[T]{fl}, fls...) {
			if iterFl != nil {
				v = iterFl(v)
			}
		}
		return v
	}
}

type Converter[I, O any] func(I) O

func MakeConverter[I, O any](f func(I) O) Converter[I, O] { return f }
func (Converter[I, O]) noop() Converter[I, O]             { return func(I) (out O) { return } }
func (cf Converter[I, O]) Convert(in I) O                 { return cf(in) }
func (cf Converter[I, O]) Safe() Converter[I, O]          { return cf.If(cf != nil) }
func (cf Converter[I, O]) Lock() Converter[I, O]          { return cf.WithLock(&sync.Mutex{}) }
func (cf Converter[I, O]) Not(cond bool) Converter[I, O]  { return cf.If(ft.Not(cond)) }
func (cf Converter[I, O]) If(cond bool) Converter[I, O]   { return ft.IfElse(cond, cf, cf.noop()) }
func (cf Converter[I, O]) When(c func() bool) Converter[I, O] {
	return func(in I) O { return cf.If(c()).Convert(in) }
}

func (cf Converter[I, O]) PreHook(h func()) Converter[I, O] { return func(i I) O { h(); return cf(i) } }
func (cf Converter[I, O]) PostHook(h func()) Converter[I, O] {
	return func(i I) O { defer ft.CallSafe(h); return cf(i) }
}

func (cf Converter[I, O]) PreFilter(f Filter[I]) Converter[I, O] {
	return func(i I) O { return cf(f(i)) }
}

func (cf Converter[I, O]) PostFilter(f Filter[O]) Converter[I, O] {
	return func(i I) O { return f(cf(i)) }
}

func (cf Converter[I, O]) WithLock(m *sync.Mutex) Converter[I, O] {
	return func(in I) O { defer prv.With(prv.Lock(m)); return cf(in) }
}

func (cf Converter[I, O]) WithLocker(m sync.Locker) Converter[I, O] {
	return func(in I) O { defer prv.WithL(prv.LockL(m)); return cf(in) }
}
