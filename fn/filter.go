package fn

import (
	"sync"

	"github.com/tychoish/fun/ft"
)

// Filter describes a common function object and provides higher level operations on the functions themselves. These functions
// take an object and returns a value of the same type. This is a special case of the Converter type.
type Filter[T any] Converter[T, T]

// MakeFilter creates a new Filter from a function that transforms a value of type T to another value of the same type.
func MakeFilter[T any](fl func(T) T) Filter[T] { return fl }
func ctf[T any](in Converter[T, T]) Filter[T]  { return MakeFilter(in) }
func (fl Filter[T]) c() Converter[T, T]        { return MakeConverter(fl) }

// Apply executes the filter function on the provided value and returns the filtered result.
func (fl Filter[T]) Apply(v T) T { return fl(v) }

// Ptr applies the filter to the value pointed to by the pointer, modifying it in place.
func (fl Filter[T]) Ptr(v *T) { *v = fl(*v) }

// WithNext returns a filter that applies this filter first, then applies the next filter to the result.
func (fl Filter[T]) WithNext(next Filter[T]) Filter[T] { return func(v T) T { return next(fl(v)) } }

// If returns a filter that only executes when the condition is true, otherwise the filter will return the input as is.
func (fl Filter[T]) If(cond bool) Filter[T] { return func(v T) T { return ft.FilterWhen(cond, fl, v) } }

// Lock returns a filter that executes the filter operation using a new mutex, ensuring only one instance of the filter runs at
// a time.
func (fl Filter[T]) Lock() Filter[T] { return fl.WithLock(&sync.Mutex{}) }

// WithLock returns a filter that protects the execution of the filter operation with the provided mutex.
func (fl Filter[T]) WithLock(mu *sync.Mutex) Filter[T] {
	return func(v T) T { mu.Lock(); defer mu.Unlock(); return fl.Apply(v) }
}

// PreHook returns a filter that executes the provided hook function before applying the filter.
func (fl Filter[T]) PreHook(op func()) Filter[T] { return ctf(fl.c().PreHook(op)) }

// PostHook returns a filter that executes the provided hook function after applying the filter.
func (fl Filter[T]) PostHook(op func()) Filter[T] { return ctf(fl.c().PostHook(op)) }

// Join returns a filter that applies this filter followed by all the provided filters in order. Nil filters cause panics.
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
