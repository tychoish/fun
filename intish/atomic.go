package intish

import (
	"math"
	"sync/atomic"
)

// Atomic is a wrapper around sync/atomic.Int64 that provides
// type-preserving atomic storage for all numeric types.
//
// Atomic shares an interface with the adt.Atomic wrapper types.
type Atomic[T Numbers] struct{ n atomic.Int64 }

func (a *Atomic[T]) Get() T        { return a.Load() }
func (a *Atomic[T]) Set(in T)      { a.Store(in) }
func (a *Atomic[T]) Load() T       { return T(a.n.Load()) }
func (a *Atomic[T]) Store(in T)    { a.n.Store(int64(in)) }
func (a *Atomic[T]) Swap(new T) T  { return T(a.n.Swap(int64(new))) }
func (a *Atomic[T]) Add(delta T) T { return T(a.n.Add(int64(delta))) }
func (a *Atomic[T]) CompareAndSwap(old, new T) bool {
	return a.n.CompareAndSwap(int64(old), int64(new))
}

// AtomicFloat64 provides full-fidelity atomic storage for float64
// values (by converting them) as bits to int64 and storing them using
// sync/atomic.Int64 values. The Add() method is correct, but must
// spin, unlike for integers which rely on an optimized underlying instruction.
//
// AtomicFloat64 shares an interface with the adt.Atomic wrapper types.
type AtomicFloat64 struct{ n atomic.Int64 }

func toInt64(in float64) int64 { return int64(math.Float64bits(in)) }
func toFloat(in int64) float64 { return math.Float64frombits(uint64(in)) }

func (a *AtomicFloat64) Get() float64             { return a.Load() }
func (a *AtomicFloat64) Set(in float64)           { a.Store(in) }
func (a *AtomicFloat64) Load() float64            { return toFloat(a.n.Load()) }
func (a *AtomicFloat64) Store(value float64)      { a.n.Store(toInt64(value)) }
func (a *AtomicFloat64) Swap(new float64) float64 { return toFloat(a.n.Swap(toInt64(new))) }

func (a *AtomicFloat64) CompareAndSwap(old, new float64) bool {
	return a.n.CompareAndSwap(toInt64(old), toInt64(new))
}

func (a *AtomicFloat64) Add(delta float64) float64 {
	for {
		old := a.n.Load()
		new := toFloat(old) + delta
		if a.n.CompareAndSwap(old, toInt64(new)) {
			return new
		}
	}
}
