//go:build go1.20

package adt

// Swap exchanges the value (v) stored in the map under the key (k) for the current value stored under that key.
func (mp *Map[K, V]) Swap(k K, v V) (V, bool) { return mp.safeCast(mp.mp.Swap(k, v)) }
