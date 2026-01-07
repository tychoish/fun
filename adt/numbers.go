package adt

import (
	"math"
	"sync/atomic"

	"github.com/tychoish/fun/stw"
)

// AtomicInteger is a wrapper around sync/atomic.Int64 that provides
// type-preserving atomic storage for all numeric types.
//
// AtomicInteger shares an interface with the adt.AtomicInteger wrapper types.
type AtomicInteger[T stw.Integers] struct{ n atomic.Int64 }

// Get retrieves the current value stored in the atomic variable.
// This is an alias for Load() for consistency with other atomic interfaces.
func (a *AtomicInteger[T]) Get() T { return a.Load() }

// Set writes a new value to the atomic variable.
// This is an alias for Store() for consistency with other atomic interfaces.
func (a *AtomicInteger[T]) Set(in T) { a.Store(in) }

// Load atomically retrieves and returns the current value.
func (a *AtomicInteger[T]) Load() T { return T(a.n.Load()) }

// Store atomically writes the given value.
func (a *AtomicInteger[T]) Store(in T) { a.n.Store(int64(in)) }

// Swap atomically replaces the current value with newVal and returns the previous value.
func (a *AtomicInteger[T]) Swap(newVal T) T { return T(a.n.Swap(int64(newVal))) }

// Add atomically increments the current value by delta and returns the new value.
func (a *AtomicInteger[T]) Add(delta T) T { return T(a.n.Add(int64(delta))) }

// CompareAndSwap atomically examines the current value against oldVal and,
// if they are equal, replaces it with newValue. It returns true if the replacement was performed.
func (a *AtomicInteger[T]) CompareAndSwap(oldVal, newValue T) bool {
	return a.n.CompareAndSwap(int64(oldVal), int64(newValue))
}

// AtomicFloat64 provides full-fidelity atomic storage for float64
// values (by converting them) as bits to int64 and storing them using
// sync/atomic.Int64 values. The Add() method is correct, but must
// spin, unlike for integers which rely on an optimized underlying
// instruction.
//
// AtomicFloat64 shares an interface with the adt.Atomic wrapper types.
type AtomicFloat64 struct{ n atomic.Int64 }

func toInt64(in float64) int64   { return int64(math.Float64bits(in)) }
func toFloat64(in int64) float64 { return math.Float64frombits(uint64(in)) }

// Get retrieves the current float64 value from the atomic variable.
// This is an alias for Load() for consistency with other atomic interfaces.
func (a *AtomicFloat64) Get() float64 { return a.Load() }

// Set writes a new float64 value to the atomic variable.
// This is an alias for Store() for consistency with other atomic interfaces.
func (a *AtomicFloat64) Set(in float64) { a.Store(in) }

// Load atomically retrieves and returns the current float64 value.
func (a *AtomicFloat64) Load() float64 { return toFloat64(a.n.Load()) }

// Store atomically writes the given float64 value.
func (a *AtomicFloat64) Store(val float64) { a.n.Store(toInt64(val)) }

// Swap atomically replaces the current value with newVal and returns the previous float64 value.
func (a *AtomicFloat64) Swap(newVal float64) float64 { return toFloat64(a.n.Swap(toInt64(newVal))) }

// CompareAndSwap atomically examines the current value against oldVal and,
// if they are equal, replaces it with newVal. It returns true if the replacement was performed.
func (a *AtomicFloat64) CompareAndSwap(oldVal, newVal float64) bool {
	return a.n.CompareAndSwap(toInt64(oldVal), toInt64(newVal))
}

// Add atomically increments the current float64 value by delta and returns the new value.
// This method uses a compare-and-swap loop since there is no atomic float64 increment instruction.
func (a *AtomicFloat64) Add(delta float64) float64 {
	for {
		oldf := a.n.Load()
		newf := toFloat64(oldf) + delta
		if a.n.CompareAndSwap(oldf, toInt64(newf)) {
			return newf
		}
	}
}
