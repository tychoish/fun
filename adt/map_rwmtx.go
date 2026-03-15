package adt

import (
	"bytes"
	"iter"
	"sync"

	"github.com/tychoish/fun/irt"
	"github.com/tychoish/fun/stw"
)

// LockedRWMap provides a map implementation that maintains insertion order.
// It has the same interface as stw.Map but iterates in insertion order.
// This implementation uses a mutex to ensure consistency between the
// underlying hash map and the insertion order list.
type LockedRWMap[K comparable, V any] struct {
	mtx  sync.RWMutex
	once sync.Once
	hash stw.Map[K, V]
}

func (m *LockedRWMap[K, V]) init()                 { m.once.Do(m.doInit) }
func (m *LockedRWMap[K, V]) doInit()               { m.hash = stw.Map[K, V]{} }
func (*LockedRWMap[K, V]) zerov() (out V)          { return }
func (*LockedRWMap[K, V]) with(mtx *sync.RWMutex)  { mtx.Unlock() }
func (m *LockedRWMap[K, V]) lock() *sync.RWMutex   { m.mtx.Lock(); m.init(); return &m.mtx }
func (*LockedRWMap[K, V]) withr(mtx *sync.RWMutex) { mtx.RUnlock() }
func (m *LockedRWMap[K, V]) lockr() *sync.RWMutex  { m.mtx.RLock(); m.init(); return &m.mtx }

// Len returns the length of the map.
func (m *LockedRWMap[K, V]) Len() int { defer m.withr(m.lockr()); return m.hash.Len() }

// Check returns true if the value K is in the map.
func (m *LockedRWMap[K, V]) Check(key K) bool { defer m.withr(m.lockr()); return m.hash.Check(key) }

// Get returns the value from the map. If the key is not present in the map,
// this returns the zero value for V.
func (m *LockedRWMap[K, V]) Get(key K) V { out, _ := m.Load(key); return out }

// Load returns the value in the map for the key, and an "ok" value
// which is true if that item is present in the map.
func (m *LockedRWMap[K, V]) Load(key K) (V, bool) { defer m.with(m.lock()); return m.hash.Load(key) }

// Delete removes a key from the map.
func (m *LockedRWMap[K, V]) Delete(k K) { defer m.with(m.lock()); m.hash.Delete(k) }

// Set adds a key-value pair directly to the map. If the key already
// exists, it updates the value but does not change the insertion
// order. When the return value is true, the key existed in the map
// before the operation.
func (m *LockedRWMap[K, V]) Set(key K, value V) bool {
	defer m.with(m.lock())
	return m.hash.Set(key, value)
}

// Ensure sets the provided key in the map to the zero value
// for the value type. If the key already exists, it is not modified.
func (m *LockedRWMap[K, V]) Ensure(key K) bool {
	defer m.with(m.lock())
	if m.hash.Check(key) {
		return true
	}

	return m.hash.Set(key, m.zerov())
}

// Store adds a key-value pair directly to the map. Alias for Add.
func (m *LockedRWMap[K, V]) Store(k K, v V) { m.Set(k, v) }

// Extend adds a sequence of key-value pairs to the map.
func (m *LockedRWMap[K, V]) Extend(seq iter.Seq2[K, V]) { irt.Apply2(seq, m.Store) }

// Iterator returns a standard Go iterator interface to the key-value
// pairs of the map in insertion order.
func (m *LockedRWMap[K, V]) Iterator() iter.Seq2[K, V] {
	m.init()
	return irt.WithRMutex2(m.hash.Iterator(), &m.mtx)
}

// Keys provides an iterator over just the keys in the map in insertion order.
func (m *LockedRWMap[K, V]) Keys() iter.Seq[K] { return irt.WithRMutex(m.hash.Keys(), &m.mtx) }

// Values provides an iterator over just the values in the map in insertion order.
func (m *LockedRWMap[K, V]) Values() iter.Seq[V] { return irt.WithRMutex(m.hash.Values(), &m.mtx) }

// MarshalJSON encodes the map as a JSON object preserving insertion order.
func (m *LockedRWMap[K, V]) MarshalJSON() ([]byte, error) {
	m.init()
	return irt.MarshalJSON2(m.Iterator())
}

// UnmarshalJSON decodes a JSON object and stores key-value pairs in insertion order.
func (m *LockedRWMap[K, V]) UnmarshalJSON(in []byte) error {
	m.init()
	for kv, err := range irt.UnmarshalJSON2[K, V](bytes.NewBuffer(in)) {
		if err != nil {
			return err
		}
		m.Store(kv.Key, kv.Value)
	}
	return nil
}
