//go:build go1.20

package adt

func (mp *Map[K, V]) Swap(k K, v V) (V, bool) { return mp.safeCast(mp.mp.Swap(k, v)) }
