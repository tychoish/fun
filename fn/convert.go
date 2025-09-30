package fn

import (
	"sync"

	"github.com/tychoish/fun/ft"
	prv "github.com/tychoish/fun/internal"
)

// Converter describes a function that takes a value of one type and returns a value of another type. provides a simplified
// version of the fun.Converter type (without contexts or errors). The Converter type provides a number of higher level
// operation over the function types with the provided methods.
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
