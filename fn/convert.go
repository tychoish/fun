package fn

import (
	"context"
	"iter"
	"sync"

	"github.com/tychoish/fun/ft"
	"github.com/tychoish/fun/irt"
)

// Converter describes a function that takes a value of one type and returns a value of another
// type. provides a simplified version of the fun.Converter type (without contexts or errors). The
// Converter type provides a number of higher level operation over the function types with the
// provided methods.
type Converter[I, O any] func(I) O

// MakeConverter creates a new Converter from a function that transforms input type I to output type O.
func MakeConverter[I, O any](f func(I) O) Converter[I, O] { return f }
func (Converter[I, O]) noop(I) (zero O)                   { return }

// Convert applies the converter function to the input value and returns the converted output. This
// is no different than running the converter directly.
func (cf Converter[I, O]) Convert(in I) O { return cf(in) }

// Lock returns a converter that protects the execution of the conversion with a new mutex, ensuring
// that only one conversion runs at a time.
func (cf Converter[I, O]) Lock() Converter[I, O] { return cf.WithLock(&sync.Mutex{}) }

// If returns a converter that executes only when the condition is true, otherwise is a noop and
// returns zero value of the output type..
func (cf Converter[I, O]) If(cond bool) Converter[I, O] { return ft.IfElse(cond, cf, cf.noop) }

// When returns a converter that executes only when the provided condition function returns true.
func (cf Converter[I, O]) When(c func() bool) Converter[I, O] {
	return func(in I) O { return cf.If(c()).Convert(in) }
}

// PreHook returns a converter that executes the provided hook function before the conversion.
func (cf Converter[I, O]) PreHook(h func()) Converter[I, O] {
	return func(i I) O { ft.CallSafe(h); return cf(i) }
}

// PostHook returns a converter that executes the provided hook function after the conversion using
// defer, ensuring that the hook always runs. Nil hook functions are ignored.
func (cf Converter[I, O]) PostHook(h func()) Converter[I, O] {
	return func(i I) O { defer ft.CallSafe(h); return cf(i) }
}

// PreFilter returns a converter that applies the provided filter to the input before executing the
// conversion.
func (cf Converter[I, O]) PreFilter(f Filter[I]) Converter[I, O] {
	return func(i I) O { return cf(f(i)) }
}

// PostFilter returns a converter that applies the provided filter to the output after executing the
// conversion.
func (cf Converter[I, O]) PostFilter(f Filter[O]) Converter[I, O] {
	return func(i I) O { return f(cf(i)) }
}

// WithLock returns a converter that protects the execution of the converter with the provided
// mutex.
func (cf Converter[I, O]) WithLock(m *sync.Mutex) Converter[I, O] {
	return func(in I) O { m.Lock(); defer m.Unlock(); return cf(in) }
}

// WithLocker returns a converter that protects the execution of the conversion with the provided
// sync.Locker instance.
func (cf Converter[I, O]) WithLocker(m sync.Locker) Converter[I, O] {
	return func(in I) O { m.Lock(); defer m.Unlock(); return cf(in) }
}

// WithContext returns a wrapped version of the converter function that has he same signature as
// fnx.Converter functions.
func (cf Converter[I, O]) WithContext() func(context.Context, I) (O, error) {
	return func(_ context.Context, in I) (O, error) { return cf(in), nil }
}

// Iterator returns an iterator that yields converted values from the input iterator into the output
// type. This is an equivalent operation to `irt.Convert`.
func (cf Converter[I, O]) Iterator(seq iter.Seq[I]) iter.Seq[O] { return irt.Convert(seq, cf) }
