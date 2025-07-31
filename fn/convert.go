package fn

import (
	"sync"

	"github.com/tychoish/fun/ft"
	priv "github.com/tychoish/fun/internal"
)

type Filter[T any] func(T) T

func MakeFilter[T any](fl func(T) T) Filter[T]         { return fl }
func (fl Filter[T]) Apply(v T) T                       { return fl(v) }
func (fl Filter[T]) Ptr(v *T)                          { *v = fl(*v) }
func (fl Filter[T]) Safe() Filter[T]                   { return func(v T) T { return ft.FilterSafe(fl, v) } }
func (fl Filter[T]) WithNext(next Filter[T]) Filter[T] { return func(v T) T { return next(fl(v)) } }
func (fl Filter[T]) If(cond bool) Filter[T]            { return func(v T) T { return ft.FilterWhen(cond, fl, v) } }
func (fl Filter[T]) Not(cond bool) Filter[T]           { return fl.If(ft.Not(cond)) }
func (fl Filter[T]) Lock() Filter[T]                   { return fl.WithLock(&sync.Mutex{}) }

func (fl Filter[T]) WithLock(mu *sync.Mutex) Filter[T] {
	return func(v T) T { defer priv.With(priv.Lock(mu)); return fl.Apply(v) }
}

func (fl Filter[T]) PreHook(op func()) Filter[T] {
	return func(v T) T { ft.CallSafe(op); return fl.Apply(v) }
}

func (fl Filter[T]) PostHook(op func()) Filter[T] {
	return func(v T) T { defer ft.CallSafe(op); return fl.Apply(v) }
}

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

func (cf Converter[I, O]) PreFilter(fl Filter[I]) Converter[I, O] {
	return func(i I) O { return cf(fl(i)) }
}

func (cf Converter[I, O]) PostFilter(fl Filter[O]) Converter[I, O] {
	return func(i I) O { return fl(cf(i)) }
}

func (cf Converter[I, O]) WithLock(m *sync.Mutex) Converter[I, O] {
	return func(in I) O { defer priv.With(priv.Lock(m)); return cf(in) }
}

func (cf Converter[I, O]) WithLocker(m sync.Locker) Converter[I, O] {
	return func(in I) O { defer priv.WithL(priv.LockL(m)); return cf(in) }
}
