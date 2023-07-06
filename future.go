package fun

import (
	"sync"

	"github.com/tychoish/fun/ft"
)

type Future[T any] func() T

func Futurize[T any](f func() T) Future[T] { return f }

func (f Future[T]) Run() T                       { return f() }
func (f Future[T]) Once() Future[T]              { return ft.OnceDo(f) }
func (f Future[T]) Producer() Producer[T]        { return ConsistentProducer(f) }
func (f Future[T]) Safe() Future[T]              { return func() T { return ft.SafeDo(f) } }
func (f Future[T]) Ignore() func()               { return func() { _ = f() } }
func (f Future[T]) If(cond bool) Future[T]       { return func() T { return ft.WhenDo(cond, f) } }
func (f Future[T]) Not(cond bool) Future[T]      { return f.If(!cond) }
func (f Future[T]) When(c func() bool) Future[T] { return func() T { return ft.WhenDo(c(), f) } }
func (f Future[T]) Lock() Future[T]              { return f.WithLock(&sync.Mutex{}) }
func (f Future[T]) PreHook(fn func()) Future[T]  { return func() T { fn(); return f() } }
func (f Future[T]) PostHook(fn func()) Future[T] { return func() T { defer fn(); return f() } }
func (f Future[T]) Slice() func() []T            { return func() []T { return []T{f()} } }

func (f Future[T]) WithLock(m *sync.Mutex) Future[T] {
	return func() T { defer with(lock(m)); return f() }
}

func (f Future[T]) Reduce(merge func(T, T) T, next Future[T]) Future[T] {
	return func() T { return merge(f(), next()) }
}

func (f Future[T]) Join(merge func(T, T) T, ops ...Future[T]) Future[T] {
	return func() (out T) {
		out = f()
		for idx := range ops {
			out = merge(out, ops[idx]())
		}
		return out
	}
}
